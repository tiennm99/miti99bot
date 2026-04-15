/**
 * @file Tests for trading/retention — trimTradesHandler per-user and global caps.
 *
 * Uses enhanced fake-d1 which supports:
 *   - SELECT DISTINCT user_id
 *   - SELECT id ... WHERE user_id = ? ORDER BY ts DESC LIMIT -1 OFFSET N
 *   - SELECT id ... ORDER BY ts DESC LIMIT -1 OFFSET N
 *   - DELETE FROM ... WHERE id IN (?, ...)
 */

import { beforeEach, describe, expect, it, vi } from "vitest";
import { createSqlStore } from "../../../src/db/create-sql-store.js";
import { trimTradesHandler } from "../../../src/modules/trading/retention.js";
import { makeFakeD1 } from "../../fakes/fake-d1.js";

// ─── helpers ─────────────────────────────────────────────────────────────────

/** Build a fresh sql store backed by a new fake D1. */
function makeTestSql() {
  const fakeDb = makeFakeD1();
  const sql = createSqlStore("trading", { DB: fakeDb });
  return { fakeDb, sql };
}

/**
 * Build N trade rows for a given user_id.
 * ts increases from 1..N so that the LAST row (index N-1) has the highest ts
 * and is therefore the "newest".
 *
 * @param {number} userId
 * @param {number} count
 * @param {number} [idOffset=0] — added to each id to keep ids globally unique.
 * @returns {object[]}
 */
function makeTrades(userId, count, idOffset = 0) {
  return Array.from({ length: count }, (_, i) => ({
    id: idOffset + i + 1,
    user_id: userId,
    symbol: "TCB",
    side: "buy",
    qty: 1,
    price_vnd: 1000,
    ts: i + 1, // ts=1 is oldest, ts=count is newest
  }));
}

/** Convenience: run trimTradesHandler with muted console output. */
async function runTrim(sql, caps) {
  const spy = vi.spyOn(console, "log").mockImplementation(() => {});
  try {
    await trimTradesHandler({}, { sql }, caps);
  } finally {
    spy.mockRestore();
  }
}

/** Return all rows remaining in trading_trades via the fake table map. */
function allRows(fakeDb) {
  return fakeDb.tables.get("trading_trades") ?? [];
}

// ─── sql=null guard ───────────────────────────────────────────────────────────

describe("trimTradesHandler — sql=null", () => {
  it("returns without throwing when sql is null", async () => {
    const logSpy = vi.spyOn(console, "log").mockImplementation(() => {});
    await expect(trimTradesHandler({}, { sql: null })).resolves.toBeUndefined();
    logSpy.mockRestore();
  });
});

// ─── per-user trim ────────────────────────────────────────────────────────────

describe("trimTradesHandler — per-user trim", () => {
  it("removes rows beyond cap, keeping newest", async () => {
    const { fakeDb, sql } = makeTestSql();
    // 5 trades for user 1, cap=3 → keeps ts=3,4,5 (newest 3), deletes ts=1,2.
    fakeDb.seed("trading_trades", makeTrades(1, 5));

    await runTrim(sql, { perUserCap: 3, globalCap: 1000 });

    const remaining = allRows(fakeDb);
    expect(remaining).toHaveLength(3);
    // Remaining should be the 3 rows with highest ts.
    const tsSorted = remaining.map((r) => r.ts).sort((a, b) => b - a);
    expect(tsSorted).toEqual([5, 4, 3]);
  });

  it("leaves users with rows <= cap untouched", async () => {
    const { fakeDb, sql } = makeTestSql();
    // User 2 has 2 trades, cap=3 → none deleted.
    fakeDb.seed("trading_trades", makeTrades(2, 2));

    await runTrim(sql, { perUserCap: 3, globalCap: 1000 });

    expect(allRows(fakeDb)).toHaveLength(2);
  });

  it("handles exactly cap rows — no deletion", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", makeTrades(1, 3));

    await runTrim(sql, { perUserCap: 3, globalCap: 1000 });

    expect(allRows(fakeDb)).toHaveLength(3);
  });

  it("trims multiple users independently", async () => {
    const { fakeDb, sql } = makeTestSql();
    // User 1: 5 trades (cap 3 → keep 3). User 2: 2 trades (cap 3 → keep 2).
    fakeDb.seed("trading_trades", [...makeTrades(1, 5, 0), ...makeTrades(2, 2, 5)]);

    await runTrim(sql, { perUserCap: 3, globalCap: 1000 });

    const remaining = allRows(fakeDb);
    // 3 from user1 + 2 from user2 = 5.
    expect(remaining).toHaveLength(5);
    const user1 = remaining.filter((r) => r.user_id === 1);
    const user2 = remaining.filter((r) => r.user_id === 2);
    expect(user1).toHaveLength(3);
    expect(user2).toHaveLength(2);
  });
});

// ─── global FIFO trim ─────────────────────────────────────────────────────────

describe("trimTradesHandler — global trim", () => {
  it("keeps only globalCap newest rows across all users", async () => {
    const { fakeDb, sql } = makeTestSql();
    // 15 trades total: user1=8, user2=7. globalCap=10 → keep 10 newest.
    const trades = [...makeTrades(1, 8, 0), ...makeTrades(2, 7, 8)];
    // Re-assign ts values to be globally unique so ordering is deterministic.
    trades.forEach((t, i) => {
      t.ts = i + 1;
    });
    fakeDb.seed("trading_trades", trades);

    await runTrim(sql, { perUserCap: 1000, globalCap: 10 });

    expect(allRows(fakeDb)).toHaveLength(10);
    // All remaining rows should have the top-10 ts values (6..15).
    const tsSorted = allRows(fakeDb)
      .map((r) => r.ts)
      .sort((a, b) => a - b);
    expect(tsSorted).toEqual([6, 7, 8, 9, 10, 11, 12, 13, 14, 15]);
  });

  it("leaves rows untouched when count <= globalCap", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", makeTrades(1, 5));

    await runTrim(sql, { perUserCap: 1000, globalCap: 10 });

    expect(allRows(fakeDb)).toHaveLength(5);
  });
});

// ─── combined pass ────────────────────────────────────────────────────────────

describe("trimTradesHandler — combined per-user + global", () => {
  it("applies per-user first, then global", async () => {
    const { fakeDb, sql } = makeTestSql();
    // User 1: 6 trades (perUserCap=4 → trim to 4).
    // User 2: 6 trades (perUserCap=4 → trim to 4).
    // After per-user: 8 rows total.
    // globalCap=6 → trim to 6.
    const trades = [...makeTrades(1, 6, 0), ...makeTrades(2, 6, 6)];
    // Assign globally monotone ts values.
    trades.forEach((t, i) => {
      t.ts = i + 1;
    });
    fakeDb.seed("trading_trades", trades);

    await runTrim(sql, { perUserCap: 4, globalCap: 6 });

    expect(allRows(fakeDb)).toHaveLength(6);
  });
});

// ─── idempotence ──────────────────────────────────────────────────────────────

describe("trimTradesHandler — idempotence", () => {
  it("second run is a no-op when already within caps", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", makeTrades(1, 5));

    await runTrim(sql, { perUserCap: 3, globalCap: 10 });
    const afterFirst = allRows(fakeDb).length;

    await runTrim(sql, { perUserCap: 3, globalCap: 10 });
    const afterSecond = allRows(fakeDb).length;

    expect(afterFirst).toBe(afterSecond);
  });
});
