/**
 * @file Tests for MongoTradesStore — all 6 methods, edge cases, ordering.
 *
 * Injects fake-mongo via constructor (dbOverride), no real MongoDB connection.
 */

import { ObjectId } from "mongodb";
import { beforeEach, describe, expect, it } from "vitest";
import { MongoTradesStore } from "../../src/db/mongo-trades-store.js";
import { makeFakeMongo } from "../fakes/fake-mongo.js";

// ─── helpers ──────────────────────────────────────────────────────────────────

/** Build a fresh store backed by a new fake Mongo db. */
function makeStore() {
  const fakeDb = makeFakeMongo();
  const store = new MongoTradesStore({}, fakeDb);
  return { fakeDb, store };
}

/** Minimal trade payload for insert. */
const TRADE = {
  userId: 1,
  symbol: "TCB",
  side: /** @type {"buy"} */ ("buy"),
  qty: 100,
  priceVnd: 25000,
  ts: 1000,
};

// ─── insert ───────────────────────────────────────────────────────────────────

describe("MongoTradesStore.insert", () => {
  it("inserts a document and returns { changes: 1, last_row_id: 0 }", async () => {
    const { store } = makeStore();
    const result = await store.insert(TRADE);
    expect(result).toEqual({ changes: 1, last_row_id: 0 });
  });

  it("last_row_id is the number 0, not a hex string", async () => {
    const { store } = makeStore();
    const result = await store.insert(TRADE);
    expect(typeof result.last_row_id).toBe("number");
    expect(result.last_row_id).toBe(0);
  });

  it("stores legacy_id as null for new runtime trades", async () => {
    const { fakeDb, store } = makeStore();
    await store.insert(TRADE);
    const coll = fakeDb.collection("trading_trades");
    const docs = await coll.find({}).toArray();
    expect(docs).toHaveLength(1);
    expect(docs[0].legacy_id).toBeNull();
  });

  it("stores field shape correctly (snake_case in db)", async () => {
    const { fakeDb, store } = makeStore();
    await store.insert(TRADE);
    const coll = fakeDb.collection("trading_trades");
    const docs = await coll.find({}).toArray();
    const doc = docs[0];
    expect(doc.user_id).toBe(1);
    expect(doc.symbol).toBe("TCB");
    expect(doc.side).toBe("buy");
    expect(doc.qty).toBe(100);
    expect(doc.price_vnd).toBe(25000);
    expect(doc.ts).toBe(1000);
  });

  it("uses provided ts when given", async () => {
    const { fakeDb, store } = makeStore();
    await store.insert({ ...TRADE, ts: 9999 });
    const coll = fakeDb.collection("trading_trades");
    const docs = await coll.find({}).toArray();
    expect(docs[0].ts).toBe(9999);
  });

  it("falls back to Date.now() when ts is not provided", async () => {
    const { fakeDb, store } = makeStore();
    const before = Date.now();
    const { ts: _, ...tradeWithoutTs } = TRADE;
    await store.insert(tradeWithoutTs);
    const after = Date.now();
    const coll = fakeDb.collection("trading_trades");
    const docs = await coll.find({}).toArray();
    expect(docs[0].ts).toBeGreaterThanOrEqual(before);
    expect(docs[0].ts).toBeLessThanOrEqual(after);
  });
});

// ─── byUser ───────────────────────────────────────────────────────────────────

describe("MongoTradesStore.byUser", () => {
  it("returns [] when collection is empty", async () => {
    const { store } = makeStore();
    expect(await store.byUser(1, 10)).toEqual([]);
  });

  it("returns only trades for the requested user", async () => {
    const { store } = makeStore();
    await store.insert({ ...TRADE, userId: 1, ts: 1 });
    await store.insert({ ...TRADE, userId: 2, ts: 2 });
    const trades = await store.byUser(1, 10);
    expect(trades).toHaveLength(1);
    expect(trades[0].userId).toBe(1);
  });

  it("returns trades newest-first (sorted by ts DESC)", async () => {
    const { store } = makeStore();
    await store.insert({ ...TRADE, ts: 100 });
    await store.insert({ ...TRADE, ts: 300 });
    await store.insert({ ...TRADE, ts: 200 });
    const trades = await store.byUser(1, 10);
    expect(trades.map((t) => t.ts)).toEqual([300, 200, 100]);
  });

  it("respects the limit", async () => {
    const { store } = makeStore();
    for (let i = 1; i <= 5; i++) {
      await store.insert({ ...TRADE, ts: i });
    }
    const trades = await store.byUser(1, 3);
    expect(trades).toHaveLength(3);
    // Must be the 3 newest.
    expect(trades.map((t) => t.ts)).toEqual([5, 4, 3]);
  });

  it("maps document fields to camelCase Trade shape", async () => {
    const { store } = makeStore();
    await store.insert(TRADE);
    const trades = await store.byUser(1, 1);
    expect(trades[0]).toMatchObject({
      userId: 1,
      symbol: "TCB",
      side: "buy",
      qty: 100,
      priceVnd: 25000,
      ts: 1000,
    });
    // id must be present (ObjectId from _id).
    expect(trades[0].id).toBeDefined();
  });
});

// ─── distinctUsers ────────────────────────────────────────────────────────────

describe("MongoTradesStore.distinctUsers", () => {
  it("returns [] when collection is empty", async () => {
    const { store } = makeStore();
    expect(await store.distinctUsers()).toEqual([]);
  });

  it("returns each user_id exactly once", async () => {
    const { store } = makeStore();
    await store.insert({ ...TRADE, userId: 1 });
    await store.insert({ ...TRADE, userId: 2 });
    await store.insert({ ...TRADE, userId: 1 }); // duplicate
    const users = await store.distinctUsers();
    expect(users.sort()).toEqual([1, 2]);
  });
});

// ─── oldRowsForUser ───────────────────────────────────────────────────────────

describe("MongoTradesStore.oldRowsForUser", () => {
  it("returns [] when user has fewer rows than keepN", async () => {
    const { store } = makeStore();
    await store.insert({ ...TRADE, ts: 1 });
    await store.insert({ ...TRADE, ts: 2 });
    expect(await store.oldRowsForUser(1, 5)).toEqual([]);
  });

  it("returns ids of rows beyond keepN (oldest rows)", async () => {
    const { store } = makeStore();
    // Insert 5 rows; keepN=3 → should return ids of the 2 oldest (ts=1,2).
    const insertedIds = [];
    for (let i = 1; i <= 5; i++) {
      await store.insert({ ...TRADE, ts: i });
    }
    // Fetch all to get their _ids in ts order.
    const all = await store.byUser(1, 10); // newest first: ts=5,4,3,2,1
    const oldIds = await store.oldRowsForUser(1, 3);
    // The 2 oldest (ts=1, ts=2) should be in oldIds.
    expect(oldIds).toHaveLength(2);
    // all[3].id = ts=2, all[4].id = ts=1 (index 3 and 4 in newest-first order).
    const expectedIds = [all[3].id, all[4].id].map(String).sort();
    const actualIds = oldIds.map(String).sort();
    expect(actualIds).toEqual(expectedIds);
  });

  it("only returns ids for the specified user, not others", async () => {
    const { store } = makeStore();
    // User 1: 4 rows, keepN=2 → 2 excess.
    for (let i = 1; i <= 4; i++) {
      await store.insert({ ...TRADE, userId: 1, ts: i });
    }
    // User 2: 4 rows, keepN=2 → should not appear.
    for (let i = 1; i <= 4; i++) {
      await store.insert({ ...TRADE, userId: 2, ts: i });
    }
    const oldIds = await store.oldRowsForUser(1, 2);
    expect(oldIds).toHaveLength(2);
    // Verify by fetching user1 docs to check _ids.
    const user1Docs = await store.byUser(1, 10); // newest first
    const expectedIds = [user1Docs[2].id, user1Docs[3].id].map(String).sort();
    expect(oldIds.map(String).sort()).toEqual(expectedIds);
  });
});

// ─── oldRows ──────────────────────────────────────────────────────────────────

describe("MongoTradesStore.oldRows", () => {
  it("returns [] when total rows <= keepN", async () => {
    const { store } = makeStore();
    await store.insert({ ...TRADE, ts: 1 });
    expect(await store.oldRows(5)).toEqual([]);
  });

  it("returns ids of rows beyond keepN globally", async () => {
    const { store } = makeStore();
    for (let i = 1; i <= 5; i++) {
      await store.insert({ ...TRADE, ts: i });
    }
    const oldIds = await store.oldRows(3);
    // 5 rows, keepN=3 → 2 excess (ts=1 and ts=2).
    expect(oldIds).toHaveLength(2);
  });

  it("spans multiple users", async () => {
    const { store } = makeStore();
    for (let i = 1; i <= 3; i++) {
      await store.insert({ ...TRADE, userId: 1, ts: i });
    }
    for (let i = 4; i <= 6; i++) {
      await store.insert({ ...TRADE, userId: 2, ts: i });
    }
    // 6 rows total, keepN=4 → 2 oldest excess (ts=1,2).
    const oldIds = await store.oldRows(4);
    expect(oldIds).toHaveLength(2);
  });
});

// ─── deleteByIds ──────────────────────────────────────────────────────────────

describe("MongoTradesStore.deleteByIds", () => {
  it("returns { deletedCount: 0 } for empty ids array (no db call)", async () => {
    const { store } = makeStore();
    const result = await store.deleteByIds([]);
    expect(result).toEqual({ deletedCount: 0 });
  });

  it("deletes the specified documents by _id", async () => {
    const { store } = makeStore();
    await store.insert({ ...TRADE, ts: 1 });
    await store.insert({ ...TRADE, ts: 2 });
    await store.insert({ ...TRADE, ts: 3 });

    const oldIds = await store.oldRows(2); // 1 excess (ts=1)
    expect(oldIds).toHaveLength(1);

    const result = await store.deleteByIds(oldIds);
    expect(result.deletedCount).toBe(1);

    // Only 2 rows should remain.
    const remaining = await store.byUser(1, 10);
    expect(remaining).toHaveLength(2);
    expect(remaining.map((t) => t.ts)).toEqual([3, 2]);
  });

  it("handles a mix of trades with and without legacy_id", async () => {
    const { fakeDb, store } = makeStore();

    // Seed one legacy doc (as if backfilled) with an ObjectId _id.
    const legacyId = new ObjectId();
    const coll = fakeDb.collection("trading_trades");
    await coll.insertOne({
      _id: legacyId,
      legacy_id: 42,
      user_id: 1,
      symbol: "VNM",
      side: "buy",
      qty: 50,
      price_vnd: 80000,
      ts: 500,
    });

    // Insert a runtime trade (ts=1000, legacy_id=null).
    await store.insert({ ...TRADE, ts: 1000 });

    // oldRows(1) → the legacy doc (ts=500) is older → its _id returned.
    const oldIds = await store.oldRows(1);
    expect(oldIds).toHaveLength(1);
    expect(String(oldIds[0])).toBe(String(legacyId));

    await store.deleteByIds(oldIds);

    const remaining = await store.byUser(1, 10);
    expect(remaining).toHaveLength(1);
    expect(remaining[0].ts).toBe(1000);
  });
});
