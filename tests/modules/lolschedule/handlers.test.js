import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  handleDailyPushCron,
  handleSubscribe,
  handleUnsubscribe,
} from "../../../src/modules/lolschedule/handlers.js";
import { listSubscribers } from "../../../src/modules/lolschedule/subscribers.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

function scheduleResponse(events) {
  return {
    ok: true,
    status: 200,
    text: async () => JSON.stringify({ data: { schedule: { events, pages: {} } } }),
  };
}

function majorEvt(startTime, leagueSlug = "lck", leagueName = "LCK") {
  return {
    startTime,
    state: "unstarted",
    type: "match",
    blockName: "W1",
    league: { name: leagueName, slug: leagueSlug },
    match: {
      id: `m-${startTime}`,
      teams: [{ code: "T1" }, { code: "GEN" }],
      strategy: { type: "bestOf", count: 3 },
    },
  };
}

function fakeCtx(chatId) {
  const replied = [];
  return {
    chat: chatId != null ? { id: chatId } : undefined,
    reply: async (text, opts) => replied.push({ text, opts }),
    replied,
  };
}

describe("handleSubscribe / handleUnsubscribe", () => {
  let db;
  beforeEach(() => {
    db = createStore("lolschedule", { KV: makeFakeKv() });
  });

  it("subscribes a new chat and persists the id", async () => {
    const ctx = fakeCtx(10);
    await handleSubscribe(ctx, db);
    expect(ctx.replied[0].text).toMatch(/Subscribed/);
    expect(await listSubscribers(db)).toEqual([10]);
  });

  it("tells an already-subscribed chat they're in", async () => {
    const ctx = fakeCtx(10);
    await handleSubscribe(ctx, db);
    await handleSubscribe(ctx, db);
    expect(ctx.replied[1].text).toMatch(/Already subscribed/);
  });

  it("unsubscribes an existing chat", async () => {
    const ctx = fakeCtx(10);
    await handleSubscribe(ctx, db);
    await handleUnsubscribe(ctx, db);
    expect(ctx.replied[1].text).toMatch(/Unsubscribed/);
    expect(await listSubscribers(db)).toEqual([]);
  });

  it("tells a non-subscriber they weren't subscribed", async () => {
    const ctx = fakeCtx(10);
    await handleUnsubscribe(ctx, db);
    expect(ctx.replied[0].text).toMatch(/weren't subscribed/);
  });

  it("warns when chat id is missing", async () => {
    const ctx = fakeCtx(null);
    await handleSubscribe(ctx, db);
    expect(ctx.replied[0].text).toMatch(/Could not read chat id/);
  });
});

describe("handleDailyPushCron", () => {
  let db;
  let telegramSpy;

  beforeEach(() => {
    db = createStore("lolschedule", { KV: makeFakeKv() });
    telegramSpy = vi.fn().mockResolvedValue({ ok: true, status: 200, text: async () => "{}" });
  });

  afterEach(() => vi.restoreAllMocks());

  function mockFetch(scheduleEvents) {
    global.fetch = vi.fn(async (url, opts) => {
      if (String(url).includes("esports-api.lolesports.com")) {
        return scheduleResponse(scheduleEvents);
      }
      return telegramSpy(url, opts);
    });
  }

  it("skips cleanly when the bot token is missing", async () => {
    mockFetch([majorEvt("2026-04-21T09:00:00Z")]);
    await db.putJSON("subscribers", [123]);
    await handleDailyPushCron({}, { db, env: {} });
    expect(telegramSpy).not.toHaveBeenCalled();
  });

  it("skips cleanly when there are no subscribers", async () => {
    mockFetch([majorEvt("2026-04-21T09:00:00Z")]);
    await handleDailyPushCron({}, { db, env: { TELEGRAM_BOT_TOKEN: "t" } });
    expect(telegramSpy).not.toHaveBeenCalled();
  });

  it("does not fan out when no major-league matches today", async () => {
    mockFetch([majorEvt("2026-04-21T09:00:00Z", "foo-cup", "Foo Cup")]);
    await db.putJSON("subscribers", [1, 2]);
    await handleDailyPushCron({}, { db, env: { TELEGRAM_BOT_TOKEN: "t" } });
    expect(telegramSpy).not.toHaveBeenCalled();
  });

  it("sends once per subscriber when matches exist today", async () => {
    const ictOffsetMs = 7 * 60 * 60 * 1000;
    const now = Date.now();
    const ictDayStart = new Date(new Date(now + ictOffsetMs).setUTCHours(0, 0, 0, 0) - ictOffsetMs);
    const pickTime = new Date(ictDayStart.getTime() + 12 * 60 * 60 * 1000).toISOString();
    mockFetch([majorEvt(pickTime)]);
    await db.putJSON("subscribers", [100, 200, 300]);

    await handleDailyPushCron({}, { db, env: { TELEGRAM_BOT_TOKEN: "tok" } });
    expect(telegramSpy).toHaveBeenCalledTimes(3);

    const bodies = telegramSpy.mock.calls.map((c) => JSON.parse(c[1].body));
    expect(bodies.map((b) => b.chat_id).sort()).toEqual([100, 200, 300]);
  });

  it("continues fanning out when one subscriber fails", async () => {
    const ictOffsetMs = 7 * 60 * 60 * 1000;
    const ictDayStart = new Date(
      new Date(Date.now() + ictOffsetMs).setUTCHours(0, 0, 0, 0) - ictOffsetMs,
    );
    const pickTime = new Date(ictDayStart.getTime() + 12 * 60 * 60 * 1000).toISOString();
    mockFetch([majorEvt(pickTime)]);
    await db.putJSON("subscribers", [1, 2]);

    // First Telegram call rejects, second resolves
    telegramSpy
      .mockImplementationOnce(async () => ({ ok: false, status: 403, text: async () => "blocked" }))
      .mockImplementationOnce(async () => ({ ok: true, status: 200, text: async () => "{}" }));

    await handleDailyPushCron({}, { db, env: { TELEGRAM_BOT_TOKEN: "tok" } });
    expect(telegramSpy).toHaveBeenCalledTimes(2);
  });
});
