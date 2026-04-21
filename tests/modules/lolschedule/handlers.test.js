import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import { handleDailyPushCron } from "../../../src/modules/lolschedule/handlers.js";
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

describe("handleDailyPushCron", () => {
  let db;
  let telegramSpy;

  beforeEach(() => {
    db = createStore("lolschedule", { KV: makeFakeKv() });
    telegramSpy = vi.fn().mockResolvedValue({ ok: true, status: 200, text: async () => "{}" });
  });

  afterEach(() => vi.restoreAllMocks());

  function mockFetch(scheduleEvents) {
    global.fetch = vi.fn(async (url) => {
      if (String(url).includes("esports-api.lolesports.com")) {
        return scheduleResponse(scheduleEvents);
      }
      return telegramSpy(url);
    });
  }

  it("skips cleanly when LOLSCHEDULE_CHAT_ID is missing", async () => {
    mockFetch([majorEvt("2026-04-21T09:00:00Z")]);
    await handleDailyPushCron({}, { db, env: { TELEGRAM_BOT_TOKEN: "t" } });
    expect(telegramSpy).not.toHaveBeenCalled();
  });

  it("skips cleanly when TELEGRAM_BOT_TOKEN is missing", async () => {
    mockFetch([majorEvt("2026-04-21T09:00:00Z")]);
    await handleDailyPushCron({}, { db, env: { LOLSCHEDULE_CHAT_ID: "123" } });
    expect(telegramSpy).not.toHaveBeenCalled();
  });

  it("does not send when no major-league matches today", async () => {
    mockFetch([majorEvt("2026-04-21T09:00:00Z", "lck-challengers", "LCK CL")]);
    await handleDailyPushCron(
      {},
      { db, env: { LOLSCHEDULE_CHAT_ID: "123", TELEGRAM_BOT_TOKEN: "t" } },
    );
    expect(telegramSpy).not.toHaveBeenCalled();
  });

  it("sends the rendered today message when major-league matches exist", async () => {
    // Use a time within the current ICT day window so fetchEventsInRange keeps it.
    const ictOffsetMs = 7 * 60 * 60 * 1000;
    const now = Date.now();
    const ictDayStart = new Date(new Date(now + ictOffsetMs).setUTCHours(0, 0, 0, 0) - ictOffsetMs);
    const pickTime = new Date(ictDayStart.getTime() + 12 * 60 * 60 * 1000).toISOString();

    mockFetch([majorEvt(pickTime)]);
    await handleDailyPushCron(
      {},
      { db, env: { LOLSCHEDULE_CHAT_ID: "42", TELEGRAM_BOT_TOKEN: "tok" } },
    );

    expect(telegramSpy).toHaveBeenCalledOnce();
    const [url] = telegramSpy.mock.calls[0];
    expect(String(url)).toContain("/bottok/sendMessage");
  });
});
