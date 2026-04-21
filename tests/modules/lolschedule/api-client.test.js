import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  fetchEventsInRange,
  fetchSchedulePage,
  getEventsCached,
} from "../../../src/modules/lolschedule/api-client.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

function scheduleResponse(events, { newer, older } = {}) {
  const payload = {
    data: { schedule: { events, pages: { newer, older } } },
  };
  return {
    ok: true,
    status: 200,
    text: async () => JSON.stringify(payload),
  };
}

function evt(startTime, state = "unstarted") {
  return {
    startTime,
    state,
    type: "match",
    blockName: "W1",
    league: { name: "LCK", slug: "lck" },
    match: {
      id: `m-${startTime}`,
      teams: [{ code: "T1" }, { code: "GEN" }],
      strategy: { type: "bestOf", count: 3 },
    },
  };
}

describe("fetchSchedulePage", () => {
  afterEach(() => vi.restoreAllMocks());

  it("calls the getSchedule endpoint with the public API key", async () => {
    const spy = vi.fn().mockResolvedValue(scheduleResponse([evt("2026-04-21T09:00:00Z")]));
    global.fetch = spy;

    const { events } = await fetchSchedulePage();
    expect(events).toHaveLength(1);
    const [url, opts] = spy.mock.calls[0];
    expect(url).toContain("esports-api.lolesports.com/persisted/gw/getSchedule");
    expect(url).toContain("hl=en-US");
    expect(opts.headers["x-api-key"]).toBeDefined();
    expect(opts.headers["User-Agent"]).toMatch(/miti99bot/);
  });

  it("drops type=show entries (pre/post shows)", async () => {
    const show = { ...evt("2026-04-21T07:00:00Z"), type: "show" };
    global.fetch = vi.fn().mockResolvedValue(scheduleResponse([show, evt("2026-04-21T09:00:00Z")]));
    const { events } = await fetchSchedulePage();
    expect(events.map((e) => e.type)).not.toContain("show");
  });

  it("surfaces non-2xx responses as errors", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      text: async () => "forbidden",
    });
    await expect(fetchSchedulePage()).rejects.toThrow(/HTTP 403/);
  });
});

describe("fetchEventsInRange", () => {
  afterEach(() => vi.restoreAllMocks());

  it("filters events by startTime in [from, to)", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      scheduleResponse(
        [
          evt("2026-04-20T09:00:00Z"), // yesterday
          evt("2026-04-21T09:00:00Z"), // today
          evt("2026-04-22T09:00:00Z"), // tomorrow — outside 1-day window
        ],
        { newer: null },
      ),
    );
    const out = await fetchEventsInRange(
      new Date("2026-04-21T00:00:00Z"),
      new Date("2026-04-22T00:00:00Z"),
    );
    expect(out.map((e) => e.startTime)).toEqual(["2026-04-21T09:00:00Z"]);
  });

  it("pages forward when the last event is before window end and a newer token exists", async () => {
    const spy = vi
      .fn()
      .mockResolvedValueOnce(scheduleResponse([evt("2026-04-21T09:00:00Z")], { newer: "tok1" }))
      .mockResolvedValueOnce(scheduleResponse([evt("2026-04-25T09:00:00Z")], { newer: null }));
    global.fetch = spy;
    const out = await fetchEventsInRange(
      new Date("2026-04-21T00:00:00Z"),
      new Date("2026-04-28T00:00:00Z"),
    );
    expect(spy).toHaveBeenCalledTimes(2);
    expect(out).toHaveLength(2);
  });
});

describe("getEventsCached", () => {
  let db;
  beforeEach(() => {
    db = createStore("lolschedule", { KV: makeFakeKv() });
  });
  afterEach(() => vi.restoreAllMocks());

  const from = new Date("2026-04-21T00:00:00Z");
  const to = new Date("2026-04-22T00:00:00Z");

  it("caches a successful fetch then serves from cache", async () => {
    const spy = vi
      .fn()
      .mockResolvedValue(scheduleResponse([evt("2026-04-21T09:00:00Z")], { newer: null }));
    global.fetch = spy;
    const r1 = await getEventsCached(db, from, to);
    const r2 = await getEventsCached(db, from, to);
    expect(r1).toHaveLength(1);
    expect(r2).toHaveLength(1);
    expect(spy).toHaveBeenCalledOnce();
  });

  it("falls back to stale cache if the fetch fails and cache is within STALE_MAX_AGE_SEC", async () => {
    await db.putJSON(`matches:${from.toISOString()}:${to.toISOString()}`, {
      ts: Date.now() - 10 * 60 * 1000, // 10 min ago, older than 120s TTL, newer than 1h stale max
      events: [evt("2026-04-21T09:00:00Z")],
    });
    global.fetch = vi.fn().mockRejectedValue(new Error("boom"));
    const out = await getEventsCached(db, from, to);
    expect(out).toHaveLength(1);
  });

  it("throws when fetch fails and no cache is available", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("boom"));
    await expect(getEventsCached(db, from, to)).rejects.toThrow(/boom/);
  });
});
