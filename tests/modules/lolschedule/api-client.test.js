import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  fetchMatchesInRange,
  getCachedMatches,
} from "../../../src/modules/lolschedule/api-client.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

function cargoResponse(rows) {
  return {
    ok: true,
    status: 200,
    json: async () => ({ cargoquery: rows.map((title) => ({ title })) }),
  };
}

const sampleRow = {
  DateTime: "2026-04-21 09:00:00",
  T1: "T1",
  T2: "Gen.G",
  S1: "",
  S2: "",
  Winner: "",
  Tournament: "LCK 2026 Spring",
  BO: "3",
  OP: "LCK/2026_Season/Spring_Season",
};

describe("fetchMatchesInRange", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("posts cargoquery with correct where clause and parses rows", async () => {
    const fetchSpy = vi.fn().mockResolvedValue(cargoResponse([sampleRow]));
    global.fetch = fetchSpy;

    const from = new Date("2026-04-21T00:00:00Z");
    const to = new Date("2026-04-22T00:00:00Z");
    const rows = await fetchMatchesInRange(from, to);

    expect(rows).toEqual([sampleRow]);
    expect(fetchSpy).toHaveBeenCalledOnce();
    const [url, opts] = fetchSpy.mock.calls[0];
    expect(url).toBe("https://lol.fandom.com/api.php");
    expect(opts.method).toBe("POST");
    expect(opts.headers["User-Agent"]).toMatch(/miti99bot/);
    const body = opts.body.toString();
    expect(body).toContain("action=cargoquery");
    expect(body).toContain("MatchSchedule%3DMS"); // "MatchSchedule=MS" url-encoded
    expect(body).toContain("2026-04-21+00%3A00%3A00");
    expect(body).toContain("2026-04-22+00%3A00%3A00");
  });

  it("surfaces Leaguepedia API error field", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ error: { info: "Bad query", code: "x" } }),
    });
    await expect(
      fetchMatchesInRange(new Date("2026-04-21T00:00:00Z"), new Date("2026-04-22T00:00:00Z")),
    ).rejects.toThrow(/Bad query/);
  });
});

describe("getCachedMatches", () => {
  let db;
  let kv;

  beforeEach(() => {
    kv = makeFakeKv();
    db = createStore("lolschedule", { KV: kv });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("caches a successful fetch and returns cached on the second call", async () => {
    const fetchSpy = vi.fn().mockResolvedValue(cargoResponse([sampleRow]));
    global.fetch = fetchSpy;

    const from = new Date("2026-04-21T00:00:00Z");
    const to = new Date("2026-04-22T00:00:00Z");
    const r1 = await getCachedMatches(db, from, to, 60);
    const r2 = await getCachedMatches(db, from, to, 60);

    expect(r1).toEqual([sampleRow]);
    expect(r2).toEqual([sampleRow]);
    expect(fetchSpy).toHaveBeenCalledOnce();
  });

  it("falls back to stale cache when the fetch fails", async () => {
    const from = new Date("2026-04-21T00:00:00Z");
    const to = new Date("2026-04-22T00:00:00Z");

    // Seed the cache with stale data directly.
    await db.putJSON(`matches:${from.toISOString()}:${to.toISOString()}`, {
      ts: Date.now() - 10 * 60 * 1000, // 10 min ago — older than the 60s TTL
      rows: [sampleRow],
    });

    global.fetch = vi.fn().mockRejectedValue(new Error("boom"));
    const out = await getCachedMatches(db, from, to, 60);
    expect(out).toEqual([sampleRow]);
  });

  it("propagates the error when no cache is available", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("boom"));
    await expect(
      getCachedMatches(db, new Date("2026-04-21T00:00:00Z"), new Date("2026-04-22T00:00:00Z"), 60),
    ).rejects.toThrow(/boom/);
  });
});
