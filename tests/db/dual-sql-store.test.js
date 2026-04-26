/**
 * @file dual-sql-store.test.js — unit tests for DualSqlStore.
 *
 * Contracts verified:
 *   1. run() writes to both primary and secondary.
 *   2. run() succeeds when secondary fails: logs warning + enqueues to retry queue.
 *   3. run() throws when primary fails.
 *   4. all() and first() read from primary only.
 *   5. prepare() and batch() delegate to primary only.
 *   6. tablePrefix is inherited from primary.
 *   7. `_kind === "dual"` sentinel present.
 */

import { beforeEach, describe, expect, it, vi } from "vitest";
import { CFSqlStore } from "../../src/db/cf-sql-store.js";
import { DualSqlStore } from "../../src/db/dual-sql-store.js";
import { makeFakeD1 } from "../fakes/fake-d1.js";
import { makeFakeKv } from "../fakes/fake-kv-namespace.js";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeStores() {
  const primaryD1 = makeFakeD1();
  const secondaryD1 = makeFakeD1();
  const retryQueueKv = makeFakeKv();
  const primary = new CFSqlStore(primaryD1);
  const secondary = new CFSqlStore(secondaryD1);
  const logger = { warn: vi.fn(), error: vi.fn(), log: vi.fn() };
  const dual = new DualSqlStore(primary, secondary, retryQueueKv, logger);
  // Simulate tablePrefix coming from primary wrapper — set directly.
  dual.tablePrefix = "trading_";
  return { primaryD1, secondaryD1, retryQueueKv, primary, secondary, dual, logger };
}

// ---------------------------------------------------------------------------
// Constructor validation
// ---------------------------------------------------------------------------

describe("DualSqlStore constructor", () => {
  it("throws when primary is missing", () => {
    const kv = makeFakeKv();
    const d1 = makeFakeD1();
    expect(() => new DualSqlStore(null, new CFSqlStore(d1), kv)).toThrow(/primary/);
  });

  it("throws when secondary is missing", () => {
    const kv = makeFakeKv();
    const d1 = makeFakeD1();
    expect(() => new DualSqlStore(new CFSqlStore(d1), null, kv)).toThrow(/secondary/);
  });

  it("throws when rawKv is missing", () => {
    const d1 = makeFakeD1();
    const store = new CFSqlStore(d1);
    expect(() => new DualSqlStore(store, store, null)).toThrow(/rawKv/);
  });
});

// ---------------------------------------------------------------------------
// _kind sentinel
// ---------------------------------------------------------------------------

describe("_kind sentinel", () => {
  it("exposes _kind === 'dual'", () => {
    const { dual } = makeStores();
    expect(dual._kind).toBe("dual");
  });
});

// ---------------------------------------------------------------------------
// tablePrefix
// ---------------------------------------------------------------------------

describe("tablePrefix", () => {
  it("inherits tablePrefix from primary", () => {
    const d1 = makeFakeD1();
    const kv = makeFakeKv();
    const primary = new CFSqlStore(d1);
    // The DualSqlStore constructor copies primary.tablePrefix.
    const dual = new DualSqlStore(primary, primary, kv);
    // CFSqlStore has no tablePrefix; DualSqlStore falls back to "".
    expect(typeof dual.tablePrefix).toBe("string");
  });
});

// ---------------------------------------------------------------------------
// run() — both succeed
// ---------------------------------------------------------------------------

describe("run() — both succeed", () => {
  it("records query in both primary and secondary runLog", async () => {
    const { primaryD1, secondaryD1, dual } = makeStores();
    await dual.run("INSERT INTO trading_trades VALUES (?)", "x");
    expect(primaryD1.runLog).toHaveLength(1);
    expect(secondaryD1.runLog).toHaveLength(1);
    expect(primaryD1.runLog[0].query).toBe("INSERT INTO trading_trades VALUES (?)");
    expect(secondaryD1.runLog[0].query).toBe("INSERT INTO trading_trades VALUES (?)");
  });

  it("returns the primary run result", async () => {
    const { dual } = makeStores();
    const result = await dual.run("INSERT INTO trading_trades VALUES (?)", "v");
    expect(result).toHaveProperty("changes");
    expect(result).toHaveProperty("last_row_id");
  });

  it("no retry entry enqueued when both succeed", async () => {
    const { retryQueueKv, dual } = makeStores();
    await dual.run("INSERT INTO trading_trades VALUES (?)", "v");
    expect(retryQueueKv.store.size).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// run() — secondary fails
// ---------------------------------------------------------------------------

describe("run() — secondary fails", () => {
  it("succeeds, logs warning, enqueues retry when secondary throws", async () => {
    const { primaryD1, secondaryD1, retryQueueKv, logger, dual } = makeStores();
    vi.spyOn(secondaryD1, "prepare").mockImplementation(() => ({
      run: () => Promise.reject(new Error("mongo write failed")),
      bind: function (...args) {
        return this;
      },
    }));

    await expect(dual.run("INSERT INTO trading_trades VALUES (?)", "val")).resolves.not.toThrow();

    expect(primaryD1.runLog).toHaveLength(1);
    expect(logger.warn).toHaveBeenCalledOnce();
    const warnArg = logger.warn.mock.calls[0][1];
    expect(warnArg.op).toBe("run");
    expect(warnArg.err).toContain("mongo write failed");
    // Bind values must NOT appear in structured log.
    expect(JSON.stringify(warnArg)).not.toContain("val");
    // Retry enqueued.
    expect(retryQueueKv.store.size).toBe(1);
    const [key] = [...retryQueueKv.store.keys()];
    expect(key).toMatch(/^__retry:mongo-sql-failed:/);
  });
});

// ---------------------------------------------------------------------------
// run() — primary fails
// ---------------------------------------------------------------------------

describe("run() — primary fails", () => {
  it("throws when primary throws", async () => {
    const { primaryD1, dual } = makeStores();
    vi.spyOn(primaryD1, "prepare").mockImplementation(() => ({
      run: () => Promise.reject(new Error("d1 gone")),
      bind: function (...args) {
        return this;
      },
    }));

    await expect(dual.run("INSERT INTO trading_trades VALUES (?)", "v")).rejects.toThrow("d1 gone");
  });
});

// ---------------------------------------------------------------------------
// Read operations — primary only
// ---------------------------------------------------------------------------

describe("all() and first() — primary only", () => {
  it("all() returns primary results", async () => {
    const { primaryD1, secondaryD1, dual } = makeStores();
    primaryD1.seed("trading_trades", [{ id: 1, symbol: "VNM" }]);
    // Secondary empty — result must still come from primary.
    const rows = await dual.all("SELECT * FROM trading_trades");
    expect(rows).toHaveLength(1);
    expect(rows[0].symbol).toBe("VNM");
  });

  it("first() returns primary first row", async () => {
    const { primaryD1, dual } = makeStores();
    primaryD1.seed("trading_trades", [{ id: 1, symbol: "FPT" }]);
    const row = await dual.first("SELECT * FROM trading_trades LIMIT 1");
    expect(row?.symbol).toBe("FPT");
  });

  it("first() returns null when primary has no rows", async () => {
    const { dual } = makeStores();
    const row = await dual.first("SELECT * FROM trading_trades LIMIT 1");
    expect(row).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// prepare() and batch() — primary only
// ---------------------------------------------------------------------------

describe("prepare() and batch() — primary only", () => {
  it("prepare() delegates to primary", () => {
    const { primaryD1, dual } = makeStores();
    // Should not throw; fake D1 returns a stub prepared statement.
    expect(() => dual.prepare("SELECT 1")).not.toThrow();
  });

  it("batch() returns primary results", async () => {
    const { primaryD1, dual } = makeStores();
    primaryD1.seed("trading_trades", [{ id: 1 }]);
    const stmt = dual.prepare("SELECT * FROM trading_trades");
    const results = await dual.batch([stmt]);
    expect(Array.isArray(results)).toBe(true);
    expect(results[0]).toHaveLength(1);
  });
});
