/**
 * @file mongo-kv-store.test.js — unit tests for MongoKVStore.
 *
 * Injection pattern: MongoKVStore constructor accepts an optional `dbOverride`
 * parameter. Tests pass a `makeFakeMongo()` db so no real Atlas connection
 * is made. The same fake is used to test mongo-client.js connect-reject retry.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MongoKVStore } from "../../src/db/mongo-kv-store.js";
import { makeFakeMongo } from "../fakes/fake-mongo.js";

// ─── helpers ────────────────────────────────────────────────────────────────

/** Build a store + fake db pair. collectionName defaults to "test". */
function makeStore(collectionName = "test") {
  const fakeDb = makeFakeMongo();
  const store = new MongoKVStore({}, collectionName, fakeDb);
  return { store, fakeDb };
}

// ─── constructor ─────────────────────────────────────────────────────────────

describe("MongoKVStore constructor", () => {
  it("throws when collectionName is missing", () => {
    expect(() => new MongoKVStore({}, "")).toThrow(/required/);
  });

  it("normalizes collection name: replaces - with _", () => {
    const fakeDb = makeFakeMongo();
    const store = new MongoKVStore({}, "loldle-emoji", fakeDb);
    expect(store._collName).toBe("loldle_emoji");
  });
});

// ─── get / put / delete ───────────────────────────────────────────────────────

describe("get / put / delete", () => {
  it("get returns null for missing key", async () => {
    const { store } = makeStore();
    expect(await store.get("missing")).toBeNull();
  });

  it("put → get round-trip", async () => {
    const { store } = makeStore();
    await store.put("k", "hello");
    expect(await store.get("k")).toBe("hello");
  });

  it("put overwrites existing value", async () => {
    const { store } = makeStore();
    await store.put("k", "first");
    await store.put("k", "second");
    expect(await store.get("k")).toBe("second");
  });

  it("delete removes the key", async () => {
    const { store } = makeStore();
    await store.put("k", "v");
    await store.delete("k");
    expect(await store.get("k")).toBeNull();
  });

  it("delete is idempotent (no-op on missing key)", async () => {
    const { store } = makeStore();
    await expect(store.delete("nope")).resolves.toBeUndefined();
  });
});

// ─── TTL / expiresAt ─────────────────────────────────────────────────────────

describe("TTL / expiresAt field", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("put with expirationTtl writes expiresAt field", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2025-01-01T00:00:00.000Z"));
    const { store, fakeDb } = makeStore();
    await store.put("k", "v", { expirationTtl: 60 });
    const col = fakeDb.collection("test");
    const doc = await col.findOne({ _id: "k" });
    expect(doc.expiresAt).toBeInstanceOf(Date);
    expect(doc.expiresAt.getTime()).toBe(new Date("2025-01-01T00:01:00.000Z").getTime());
  });

  it("put without TTL clears any existing expiresAt", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2025-01-01T00:00:00.000Z"));
    const { store, fakeDb } = makeStore();
    // First write with TTL
    await store.put("k", "v", { expirationTtl: 60 });
    // Second write without TTL — must remove expiresAt
    await store.put("k", "updated");
    const col = fakeDb.collection("test");
    const doc = await col.findOne({ _id: "k" });
    expect(doc.expiresAt).toBeUndefined();
  });

  it("TTL stale-read regression: expired doc returns null before sweeper runs", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2025-01-01T00:00:00.000Z"));
    const { store } = makeStore();
    await store.put("k", "v", { expirationTtl: 1 }); // expires in 1s

    // Advance clock 2 seconds — doc is now expired but sweeper hasn't run
    vi.setSystemTime(new Date("2025-01-01T00:00:02.000Z"));

    expect(await store.get("k")).toBeNull();
  });

  it("non-expired doc is still readable before TTL elapses", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2025-01-01T00:00:00.000Z"));
    const { store } = makeStore();
    await store.put("k", "v", { expirationTtl: 60 });

    // Only 10s later — still live
    vi.setSystemTime(new Date("2025-01-01T00:00:10.000Z"));

    expect(await store.get("k")).toBe("v");
  });
});

// ─── getJSON / putJSON ───────────────────────────────────────────────────────

describe("getJSON / putJSON", () => {
  it("putJSON → getJSON round-trip", async () => {
    const { store } = makeStore();
    await store.putJSON("k", { a: 1, b: [2, 3] });
    expect(await store.getJSON("k")).toEqual({ a: 1, b: [2, 3] });
  });

  it("getJSON returns null on missing key", async () => {
    const { store } = makeStore();
    expect(await store.getJSON("missing")).toBeNull();
  });

  it("getJSON returns null on corrupt JSON and logs a warning", async () => {
    const { store, fakeDb } = makeStore();
    // Seed corrupt document directly into the fake collection
    await fakeDb.collection("test").insertOne({ _id: "bad", value: "{not json" });
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    expect(await store.getJSON("bad")).toBeNull();
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });

  it("putJSON throws on undefined value", async () => {
    const { store } = makeStore();
    await expect(store.putJSON("k", undefined)).rejects.toThrow(/undefined/);
  });

  it("putJSON throws on circular reference", async () => {
    const { store } = makeStore();
    const obj = {};
    obj.self = obj;
    await expect(store.putJSON("k", obj)).rejects.toThrow();
  });

  it("putJSON passes expirationTtl through to put", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2025-01-01T00:00:00.000Z"));
    const { store, fakeDb } = makeStore();
    await store.putJSON("k", { x: 1 }, { expirationTtl: 120 });
    const doc = await fakeDb.collection("test").findOne({ _id: "k" });
    expect(doc.expiresAt).toBeInstanceOf(Date);
    expect(doc.expiresAt.getTime()).toBe(new Date("2025-01-01T00:02:00.000Z").getTime());
    vi.useRealTimers();
  });
});

// ─── list ────────────────────────────────────────────────────────────────────

describe("list()", () => {
  it("returns empty result when store is empty", async () => {
    const { store } = makeStore();
    const res = await store.list();
    expect(res.keys).toEqual([]);
    expect(res.done).toBe(true);
    expect(res.cursor).toBeUndefined();
  });

  it("returns all keys when no prefix given", async () => {
    const { store } = makeStore();
    await store.put("a:1", "x");
    await store.put("b:2", "y");
    const res = await store.list();
    expect(res.keys.sort()).toEqual(["a:1", "b:2"]);
    expect(res.done).toBe(true);
  });

  it("returns keys WITH prefix preserved (not stripped)", async () => {
    const { store } = makeStore();
    await store.put("wordle:games:1", "a");
    await store.put("wordle:games:2", "b");
    await store.put("wordle:other:3", "c");
    const res = await store.list({ prefix: "wordle:games:" });
    expect(res.keys.sort()).toEqual(["wordle:games:1", "wordle:games:2"]);
    expect(res.done).toBe(true);
  });

  it("2-level prefix regression: only matching keys returned with prefix preserved", async () => {
    const { store } = makeStore();
    await store.put("wordle:games:1", "a");
    await store.put("wordle:games:2", "b");
    await store.put("wordle:other:3", "c");
    const res = await store.list({ prefix: "wordle:games:" });
    // Must be exactly 2 keys, both with prefix intact
    expect(res.keys).toHaveLength(2);
    expect(res.keys).toContain("wordle:games:1");
    expect(res.keys).toContain("wordle:games:2");
    expect(res.keys).not.toContain("wordle:other:3");
  });

  it("prefix with regex special chars is escaped", async () => {
    const { store } = makeStore();
    await store.put("a.b:1", "x");
    await store.put("a_b:1", "y"); // should NOT match prefix "a.b:"
    const res = await store.list({ prefix: "a.b:" });
    expect(res.keys).toEqual(["a.b:1"]);
  });

  it("list() cursor pagination — limit(N+1) strategy", async () => {
    const { store } = makeStore();
    for (let i = 1; i <= 5; i++) await store.put(`k${i}`, String(i));

    const page1 = await store.list({ limit: 2 });
    expect(page1.keys).toHaveLength(2);
    expect(page1.done).toBe(false);
    expect(page1.cursor).toBeTruthy();

    const page2 = await store.list({ limit: 2, cursor: page1.cursor });
    expect(page2.keys).toHaveLength(2);
    expect(page2.done).toBe(false);
    expect(page2.cursor).toBeTruthy();

    const page3 = await store.list({ limit: 2, cursor: page2.cursor });
    expect(page3.keys).toHaveLength(1);
    expect(page3.done).toBe(true);
    expect(page3.cursor).toBeUndefined();

    // All keys across pages must be unique and sorted
    const allKeys = [...page1.keys, ...page2.keys, ...page3.keys];
    expect(allKeys).toHaveLength(5);
    expect(allKeys).toEqual([...allKeys].sort());
  });

  it("list done=true when exactly limit keys remain (no extra page)", async () => {
    const { store } = makeStore();
    await store.put("k1", "a");
    await store.put("k2", "b");
    const res = await store.list({ limit: 2 });
    expect(res.keys).toHaveLength(2);
    expect(res.done).toBe(true);
    expect(res.cursor).toBeUndefined();
  });
});

// ─── mongo-client connect-reject retry regression ────────────────────────────

describe("mongo-client connect-reject retry regression", () => {
  it("nulls client and connectPromise on connect() rejection so next call retries", async () => {
    // Import the module fresh — we'll call it with a mock factory
    const { getDb, closeMongo } = await import("../../src/db/mongo-client.js");

    // Ensure clean state before test
    await closeMongo();

    // Patch MongoClient.prototype.connect to reject once, then succeed
    const { MongoClient } = await import("mongodb");
    let callCount = 0;
    const originalConnect = MongoClient.prototype.connect;
    MongoClient.prototype.connect = vi.fn(async function () {
      callCount++;
      if (callCount === 1) {
        // First call: reject to simulate transient failure
        throw new Error("transient connection error");
      }
      // Second call: succeed (but return void, the client is now "connected")
      return this;
    });

    try {
      // First call — must reject
      await expect(getDb({ MONGODB_URI: "mongodb://localhost:27017" })).rejects.toThrow(
        "transient connection error",
      );

      // Second call — must NOT reuse the dead client; must retry
      // It will succeed on second connect() call
      const db = await getDb({ MONGODB_URI: "mongodb://localhost:27017" });
      expect(db).toBeTruthy();
      expect(callCount).toBe(2);
    } finally {
      MongoClient.prototype.connect = originalConnect;
      await closeMongo();
    }
  });

  it("logs structured warning on MongoServerSelectionError", async () => {
    const { getDb, closeMongo } = await import("../../src/db/mongo-client.js");
    await closeMongo();

    const { MongoClient } = await import("mongodb");
    const originalConnect = MongoClient.prototype.connect;
    MongoClient.prototype.connect = vi.fn(async () => {
      const err = new Error("server selection timeout");
      err.name = "MongoServerSelectionError";
      throw err;
    });

    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});

    try {
      await expect(getDb({ MONGODB_URI: "mongodb://localhost:27017" })).rejects.toThrow();
      expect(warn).toHaveBeenCalledOnce();
      const logged = JSON.parse(warn.mock.calls[0][0]);
      expect(logged.event).toBe("mongo_server_selection_failed");
      expect(logged.note).toMatch(/503/);
    } finally {
      MongoClient.prototype.connect = originalConnect;
      warn.mockRestore();
      await closeMongo();
    }
  });
});
