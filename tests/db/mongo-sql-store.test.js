/**
 * @file Tests for MongoSqlStore shim — contract compliance for SqlStore interface.
 *
 * Key assertion: `run` INSERT returns `{ changes: 1, last_row_id: 0 }` where
 * `last_row_id` is the NUMBER 0, not a hex string. This satisfies the contract
 * checked by tests/db/create-sql-store.test.js:48-52.
 */

import { describe, expect, it, vi } from "vitest";
import { MongoSqlStore } from "../../src/db/mongo-sql-store.js";
import { MongoTradesStore } from "../../src/db/mongo-trades-store.js";
import { makeFakeMongo } from "../fakes/fake-mongo.js";

// ─── helpers ──────────────────────────────────────────────────────────────────

/** Build a MongoSqlStore backed by a fresh fake MongoTradesStore. */
function makeShim() {
  const fakeDb = makeFakeMongo();
  const tradesStore = new MongoTradesStore({}, fakeDb);
  const shim = new MongoSqlStore({}, "trading", tradesStore);
  return { fakeDb, tradesStore, shim };
}

// ─── tablePrefix ──────────────────────────────────────────────────────────────

describe("MongoSqlStore — tablePrefix", () => {
  it("exposes tablePrefix as moduleName + underscore", () => {
    const { shim } = makeShim();
    expect(shim.tablePrefix).toBe("trading_");
  });
});

// ─── run — INSERT ─────────────────────────────────────────────────────────────

describe("MongoSqlStore.run — INSERT", () => {
  it("returns { changes: 1, last_row_id: 0 } for INSERT INTO trading_trades", async () => {
    const { shim } = makeShim();
    const result = await shim.run(
      "INSERT INTO trading_trades (user_id, symbol, side, qty, price_vnd, ts) VALUES (?, ?, ?, ?, ?, ?)",
      1,
      "TCB",
      "buy",
      100,
      25000,
      1000,
    );
    expect(result).toEqual({ changes: 1, last_row_id: 0 });
  });

  it("last_row_id is a NUMBER (not hex string) — satisfies create-sql-store.test.js:48-52", async () => {
    const { shim } = makeShim();
    const result = await shim.run(
      "INSERT INTO trading_trades (user_id, symbol, side, qty, price_vnd, ts) VALUES (?, ?, ?, ?, ?, ?)",
      1,
      "VNM",
      "sell",
      50,
      80000,
      2000,
    );
    expect(typeof result.last_row_id).toBe("number");
    expect(result.last_row_id).toBe(0);
    expect(typeof result.changes).toBe("number");
  });

  it("actually inserts the document into the store", async () => {
    const { fakeDb, shim } = makeShim();
    await shim.run(
      "INSERT INTO trading_trades (user_id, symbol, side, qty, price_vnd, ts) VALUES (?, ?, ?, ?, ?, ?)",
      1,
      "TCB",
      "buy",
      100,
      25000,
      1000,
    );
    const docs = await fakeDb.collection("trading_trades").find({}).toArray();
    expect(docs).toHaveLength(1);
    expect(docs[0].symbol).toBe("TCB");
  });

  it("throws for unrecognised run query", async () => {
    const { shim } = makeShim();
    await expect(shim.run("UPDATE trading_trades SET qty = ? WHERE id = ?", 5, 1)).rejects.toThrow(
      "MongoSqlStore: unsupported query",
    );
  });
});

// ─── run — DELETE by ids ──────────────────────────────────────────────────────

describe("MongoSqlStore.run — DELETE WHERE id IN", () => {
  it("delegates delete and returns changes count", async () => {
    const { shim, tradesStore } = makeShim();
    // Insert two trades so we have ids to delete.
    await tradesStore.insert({
      userId: 1,
      symbol: "TCB",
      side: "buy",
      qty: 1,
      priceVnd: 100,
      ts: 1,
    });
    await tradesStore.insert({
      userId: 1,
      symbol: "TCB",
      side: "buy",
      qty: 1,
      priceVnd: 100,
      ts: 2,
    });
    const oldIds = await tradesStore.oldRows(1); // 1 excess
    expect(oldIds).toHaveLength(1);

    const result = await shim.run("DELETE FROM trading_trades WHERE id IN (?)", ...oldIds);
    expect(result.changes).toBe(1);
    expect(result.last_row_id).toBe(0);
  });
});

// ─── all ──────────────────────────────────────────────────────────────────────

describe("MongoSqlStore.all", () => {
  it("SELECT DISTINCT user_id → returns array of { user_id } objects", async () => {
    const { shim, tradesStore } = makeShim();
    await tradesStore.insert({ userId: 1, symbol: "TCB", side: "buy", qty: 1, priceVnd: 1, ts: 1 });
    await tradesStore.insert({ userId: 2, symbol: "TCB", side: "buy", qty: 1, priceVnd: 1, ts: 2 });
    await tradesStore.insert({ userId: 1, symbol: "TCB", side: "buy", qty: 1, priceVnd: 1, ts: 3 });

    const rows = await shim.all("SELECT DISTINCT user_id FROM trading_trades");
    expect(rows.map((r) => r.user_id).sort()).toEqual([1, 2]);
  });

  it("SELECT id … WHERE user_id = ? … OFFSET ? → returns old row ids for user", async () => {
    const { shim, tradesStore } = makeShim();
    for (let i = 1; i <= 5; i++) {
      await tradesStore.insert({
        userId: 1,
        symbol: "TCB",
        side: "buy",
        qty: 1,
        priceVnd: 1,
        ts: i,
      });
    }
    const ids = await shim.all(
      "SELECT id FROM trading_trades WHERE user_id = ? ORDER BY ts DESC LIMIT -1 OFFSET ?",
      1,
      3,
    );
    expect(ids).toHaveLength(2);
  });

  it("SELECT id … ORDER BY ts DESC … OFFSET ? → returns global old row ids", async () => {
    const { shim, tradesStore } = makeShim();
    for (let i = 1; i <= 5; i++) {
      await tradesStore.insert({
        userId: 1,
        symbol: "TCB",
        side: "buy",
        qty: 1,
        priceVnd: 1,
        ts: i,
      });
    }
    const ids = await shim.all(
      "SELECT id FROM trading_trades ORDER BY ts DESC LIMIT -1 OFFSET ?",
      3,
    );
    expect(ids).toHaveLength(2);
  });

  it("SELECT … WHERE user_id = ? … LIMIT ? → returns byUser results", async () => {
    const { shim, tradesStore } = makeShim();
    await tradesStore.insert({
      userId: 1,
      symbol: "TCB",
      side: "buy",
      qty: 5,
      priceVnd: 1000,
      ts: 100,
    });
    await tradesStore.insert({
      userId: 1,
      symbol: "VNM",
      side: "sell",
      qty: 2,
      priceVnd: 2000,
      ts: 200,
    });

    const rows = await shim.all(
      "SELECT id, user_id, symbol, side, qty, price_vnd, ts FROM trading_trades WHERE user_id = ? ORDER BY ts DESC LIMIT ?",
      1,
      10,
    );
    expect(rows).toHaveLength(2);
    expect(rows[0].ts).toBe(200); // newest first
  });

  it("throws for unrecognised all query", async () => {
    const { shim } = makeShim();
    await expect(shim.all("SELECT * FROM trading_trades")).rejects.toThrow(
      "MongoSqlStore: unsupported query",
    );
  });
});

// ─── first ────────────────────────────────────────────────────────────────────

describe("MongoSqlStore.first", () => {
  it("returns the first row from a valid query", async () => {
    const { shim, tradesStore } = makeShim();
    await tradesStore.insert({ userId: 1, symbol: "TCB", side: "buy", qty: 1, priceVnd: 1, ts: 1 });
    const row = await shim.first(
      "SELECT id, user_id, symbol, side, qty, price_vnd, ts FROM trading_trades WHERE user_id = ? ORDER BY ts DESC LIMIT ?",
      1,
      10,
    );
    expect(row).not.toBeNull();
    expect(row.userId).toBe(1);
  });

  it("returns null when no rows match", async () => {
    const { shim } = makeShim();
    const row = await shim.first(
      "SELECT id, user_id, symbol, side, qty, price_vnd, ts FROM trading_trades WHERE user_id = ? ORDER BY ts DESC LIMIT ?",
      99,
      10,
    );
    expect(row).toBeNull();
  });
});

// ─── prepare / batch ─────────────────────────────────────────────────────────

describe("MongoSqlStore — prepare / batch throw", () => {
  it("prepare throws 'unsupported in MongoSqlStore'", () => {
    const { shim } = makeShim();
    expect(() => shim.prepare("SELECT 1")).toThrow("unsupported in MongoSqlStore");
  });

  it("batch throws 'unsupported in MongoSqlStore'", () => {
    const { shim } = makeShim();
    expect(() => shim.batch([])).toThrow("unsupported in MongoSqlStore");
  });
});
