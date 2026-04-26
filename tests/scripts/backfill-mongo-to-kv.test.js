/**
 * @file backfill-mongo-to-kv.test.js — unit tests for reverse-backfill TTL math.
 *
 * Tests the `computeTtl` helper exported from backfill-mongo-to-kv.js.
 * No real Atlas connection, no real CF REST calls.
 */

import { afterEach, describe, expect, it, vi } from "vitest";
import { computeTtl } from "../../scripts/backfill-mongo-to-kv.js";

// ─── computeTtl ───────────────────────────────────────────────────────────────

describe("computeTtl", () => {
  const NOW = 1_700_000_000_000; // fixed epoch ms for deterministic tests

  it("returns null when expiresAt is null (persistent key)", () => {
    expect(computeTtl(null, NOW)).toBeNull();
  });

  it("returns null when expiresAt is undefined (persistent key)", () => {
    expect(computeTtl(undefined, NOW)).toBeNull();
  });

  it("returns 'expired' when expiresAt is in the past", () => {
    const past = new Date(NOW - 1000); // 1 second ago
    expect(computeTtl(past, NOW)).toBe("expired");
  });

  it("returns 'expired' when expiresAt equals now exactly", () => {
    const now = new Date(NOW);
    expect(computeTtl(now, NOW)).toBe("expired");
  });

  it("returns correct ttl (seconds) for a future expiresAt", () => {
    const future = new Date(NOW + 120_000); // 120 seconds from now
    const result = computeTtl(future, NOW);
    expect(result).not.toBeNull();
    expect(result).not.toBe("expired");
    expect(/** @type {{ttl: number}} */ (result).ttl).toBe(120);
  });

  it("floors partial seconds (no rounding up)", () => {
    // 90.9 seconds remaining → floor → 90
    const future = new Date(NOW + 90_900);
    const result = computeTtl(future, NOW);
    expect(/** @type {{ttl: number}} */ (result).ttl).toBe(90);
  });

  it("clamps ttl to MIN_TTL_SECS (60) when remaining < 60s", () => {
    // 30 seconds remaining — below CF KV minimum of 60
    const future = new Date(NOW + 30_000);
    const result = computeTtl(future, NOW);
    expect(/** @type {{ttl: number}} */ (result).ttl).toBe(60);
  });

  it("clamps ttl to MIN_TTL_SECS (60) when remaining is 1s", () => {
    const future = new Date(NOW + 1_000);
    const result = computeTtl(future, NOW);
    expect(/** @type {{ttl: number}} */ (result).ttl).toBe(60);
  });

  it("does NOT clamp when remaining is exactly 60s", () => {
    const future = new Date(NOW + 60_000);
    const result = computeTtl(future, NOW);
    expect(/** @type {{ttl: number}} */ (result).ttl).toBe(60);
  });

  it("does NOT clamp for large TTL values", () => {
    // 7 days = 604800s
    const future = new Date(NOW + 7 * 24 * 3600 * 1000);
    const result = computeTtl(future, NOW);
    expect(/** @type {{ttl: number}} */ (result).ttl).toBe(604800);
  });
});

// ─── cfKvPut integration (fetch mock) ────────────────────────────────────────
// Verify that computeTtl=null produces a PUT with no expiration_ttl query param,
// and computeTtl={ttl:N} appends the param.

describe("cfKvPut TTL query param (fetch mock)", () => {
  afterEach(() => vi.restoreAllMocks());

  async function mockPut(expirationTtl) {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: () => Promise.resolve(""),
    });
    vi.stubGlobal("fetch", fetchMock);

    const { cfKvPut } = await import("../../scripts/lib/migration-helpers.js");
    const opts = expirationTtl != null ? { expirationTtl } : {};
    await cfKvPut("acct", "ns", "tok", "mod:key", "value", opts);
    return fetchMock.mock.calls[0][0]; // the URL string
  }

  it("omits expiration_ttl param when no TTL provided", async () => {
    const url = await mockPut(undefined);
    expect(url).not.toContain("expiration_ttl");
  });

  it("appends expiration_ttl param when TTL provided", async () => {
    const url = await mockPut(300);
    expect(url).toContain("expiration_ttl=300");
  });

  it("URL-encodes the key in the PUT request URL", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(""),
    });
    vi.stubGlobal("fetch", fetchMock);
    const { cfKvPut } = await import("../../scripts/lib/migration-helpers.js");
    await cfKvPut("acct", "ns", "tok", "mod:key with spaces", "v", {});
    const url = fetchMock.mock.calls[0][0];
    expect(url).toContain("mod%3Akey%20with%20spaces");
  });
});
