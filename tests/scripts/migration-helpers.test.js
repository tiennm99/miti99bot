/**
 * @file migration-helpers.test.js — pure-logic unit tests for migration-helpers.
 *
 * Tests: sha256, checkpoint round-trip, countDiffRatio, sampleStrategy,
 * cfKvList / cfKvGet response parsing (via fetch mock).
 *
 * No real Atlas connection, no real CF REST calls.
 */

import { existsSync, unlinkSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cfKvGet,
  cfKvList,
  clearCheckpoint,
  closeMongoClient,
  countDiffRatio,
  loadCheckpoint,
  sampleStrategy,
  saveCheckpoint,
  sha256,
  sleep,
} from "../../scripts/lib/migration-helpers.js";

// ─── sleep ────────────────────────────────────────────────────────────────────

describe("sleep", () => {
  it("resolves after at least ms milliseconds", async () => {
    const start = Date.now();
    await sleep(20);
    expect(Date.now() - start).toBeGreaterThanOrEqual(15); // allow 5ms jitter
  });
});

// ─── sha256 ───────────────────────────────────────────────────────────────────

describe("sha256", () => {
  it("produces a 64-char hex string", () => {
    expect(sha256("hello")).toMatch(/^[0-9a-f]{64}$/);
  });

  it("is deterministic for the same input", () => {
    expect(sha256("test")).toBe(sha256("test"));
  });

  it("differs for different inputs", () => {
    expect(sha256("a")).not.toBe(sha256("b"));
  });

  it("known vector: empty string", () => {
    // SHA256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
    expect(sha256("")).toBe("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855");
  });
});

// ─── Checkpoint ───────────────────────────────────────────────────────────────

describe("checkpoint round-trip", () => {
  const TEST_MODULE = "__test_module_unit__";
  const cpFile = resolve(process.cwd(), `.backfill-cursor-${TEST_MODULE}.json`);

  afterEach(() => {
    if (existsSync(cpFile)) unlinkSync(cpFile);
  });

  it("loadCheckpoint returns null when no file exists", () => {
    expect(loadCheckpoint(TEST_MODULE)).toBeNull();
  });

  it("saveCheckpoint + loadCheckpoint round-trip", () => {
    const state = { cursor: "abc123", lastKey: "wordle:games:42" };
    saveCheckpoint(TEST_MODULE, state);
    expect(loadCheckpoint(TEST_MODULE)).toEqual(state);
  });

  it("clearCheckpoint removes the file", () => {
    saveCheckpoint(TEST_MODULE, { cursor: "x", lastKey: "k" });
    expect(existsSync(cpFile)).toBe(true);
    clearCheckpoint(TEST_MODULE);
    expect(existsSync(cpFile)).toBe(false);
  });

  it("clearCheckpoint is a no-op when file absent", () => {
    expect(() => clearCheckpoint(TEST_MODULE)).not.toThrow();
  });

  it("loadCheckpoint returns null on corrupt JSON", () => {
    writeFileSync(cpFile, "{bad json", "utf8");
    expect(loadCheckpoint(TEST_MODULE)).toBeNull();
  });
});

// ─── countDiffRatio ───────────────────────────────────────────────────────────

describe("countDiffRatio", () => {
  it("returns 0 for equal counts", () => {
    expect(countDiffRatio(100, 100)).toBe(0);
  });

  it("returns 1 for (0, N)", () => {
    expect(countDiffRatio(0, 100)).toBe(1);
  });

  it("returns 1 for (N, 0)", () => {
    expect(countDiffRatio(50, 0)).toBe(1);
  });

  it("handles (0, 0) without division-by-zero — max(a,b,1)=1", () => {
    expect(countDiffRatio(0, 0)).toBe(0);
  });

  it("1% diff on 1000", () => {
    expect(countDiffRatio(1000, 990)).toBeCloseTo(0.01);
  });

  it("below 1% threshold for 5 diff on 1000", () => {
    expect(countDiffRatio(1000, 995)).toBeLessThan(0.01);
  });
});

// ─── sampleStrategy ───────────────────────────────────────────────────────────

describe("sampleStrategy", () => {
  it("full-scan for 0 total", () => {
    const { fullScan, sampleSize } = sampleStrategy(0);
    expect(fullScan).toBe(true);
    expect(sampleSize).toBe(0);
  });

  it("full-scan for 9999 total", () => {
    const { fullScan, sampleSize } = sampleStrategy(9999);
    expect(fullScan).toBe(true);
    expect(sampleSize).toBe(9999);
  });

  it("full-scan boundary: 10000 triggers sampling", () => {
    const { fullScan } = sampleStrategy(10000);
    expect(fullScan).toBe(false);
  });

  it("sample size = ceil(sqrt(total)) for mid range", () => {
    // sqrt(40000) = 200 → sampleSize = 200
    const { sampleSize } = sampleStrategy(40000);
    expect(sampleSize).toBe(200);
  });

  it("sample size capped at 500", () => {
    // sqrt(1_000_000) = 1000, capped to 500
    const { sampleSize } = sampleStrategy(1_000_000);
    expect(sampleSize).toBe(500);
  });

  it("ceil applied: sqrt(10001) ≈ 100.005 → 101", () => {
    const { sampleSize } = sampleStrategy(10201); // sqrt = 101 exactly
    expect(sampleSize).toBe(101);
  });
});

// ─── cfKvList ─────────────────────────────────────────────────────────────────

describe("cfKvList (fetch mock)", () => {
  afterEach(() => vi.restoreAllMocks());

  function mockFetch(body, status = 200) {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: status >= 200 && status < 300,
        status,
        statusText: status === 200 ? "OK" : "Error",
        text: () => Promise.resolve(JSON.stringify(body)),
        json: () => Promise.resolve(body),
      }),
    );
  }

  it("parses a successful list response", async () => {
    mockFetch({
      success: true,
      result: [
        { name: "wordle:game:1", metadata: { expiration: 1700000000 } },
        { name: "wordle:game:2" },
      ],
      result_info: { count: 2 },
    });
    const result = await cfKvList("acct", "ns", "tok", "wordle:", null);
    expect(result.keys).toHaveLength(2);
    expect(result.keys[0].name).toBe("wordle:game:1");
    expect(result.keys[0].metadata?.expiration).toBe(1700000000);
    expect(result.list_complete).toBe(true);
    expect(result.cursor).toBeNull();
  });

  it("includes cursor when result_info.cursor present", async () => {
    mockFetch({
      success: true,
      result: Array.from({ length: 1000 }, (_, i) => ({ name: `k:${i}` })),
      result_info: { count: 1000, cursor: "next-page-cursor" },
    });
    const result = await cfKvList("acct", "ns", "tok", "k:", null);
    expect(result.cursor).toBe("next-page-cursor");
    expect(result.list_complete).toBe(false);
  });

  it("throws on HTTP error", async () => {
    mockFetch({ errors: ["unauthorized"] }, 401);
    await expect(cfKvList("acct", "ns", "tok", "x:", null)).rejects.toThrow("401");
  });

  it("throws when success=false", async () => {
    mockFetch({ success: false, errors: [{ message: "namespace not found" }] });
    await expect(cfKvList("acct", "ns", "tok", "x:", null)).rejects.toThrow(/error/i);
  });
});

// ─── cfKvGet ──────────────────────────────────────────────────────────────────

describe("cfKvGet (fetch mock)", () => {
  afterEach(() => vi.restoreAllMocks());

  it("returns value text on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        text: () => Promise.resolve('{"score":42}'),
      }),
    );
    const val = await cfKvGet("acct", "ns", "tok", "wordle:game:1");
    expect(val).toBe('{"score":42}');
  });

  it("URL-encodes the key in the request URL", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve("v"),
    });
    vi.stubGlobal("fetch", fetchMock);
    await cfKvGet("acct", "ns", "tok", "some key/with spaces");
    const calledUrl = fetchMock.mock.calls[0][0];
    expect(calledUrl).toContain("some%20key%2Fwith%20spaces");
  });

  it("throws on HTTP 404", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
        statusText: "Not Found",
        text: () => Promise.resolve("not found"),
      }),
    );
    await expect(cfKvGet("acct", "ns", "tok", "missing:key")).rejects.toThrow("404");
  });
});

// ─── getMongoClient teardown ──────────────────────────────────────────────────

// Ensure any singleton client opened during tests is closed (prevents open handles).
afterEach(async () => {
  await closeMongoClient();
});
