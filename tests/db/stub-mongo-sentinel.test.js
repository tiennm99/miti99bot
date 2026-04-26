/**
 * @file stub-mongo-sentinel.test.js — asserts that MongoClient.prototype.connect
 * is NEVER called when STUB_SENTINEL flows through the store factories.
 *
 * This is the regression test for code-reviewer finding #2: deploy-time
 * register.js must not attempt an Atlas connection.
 *
 * Covers every flag combination from the matrix where MONGODB_URI is set to
 * STUB_SENTINEL — all should return CF-only stores and never touch MongoClient.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { STUB_SENTINEL } from "../../scripts/stub-kv.js";
import { createSqlStore } from "../../src/db/create-sql-store.js";
import { createStore } from "../../src/db/create-store.js";
import { makeFakeD1 } from "../fakes/fake-d1.js";
import { makeFakeKv } from "../fakes/fake-kv-namespace.js";

// ---------------------------------------------------------------------------
// Spy on MongoClient.prototype.connect
// ---------------------------------------------------------------------------

let connectSpy;

beforeEach(async () => {
  const { MongoClient } = await import("mongodb");
  connectSpy = vi.spyOn(MongoClient.prototype, "connect").mockResolvedValue(undefined);
});

afterEach(() => {
  connectSpy?.mockRestore();
});

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

function makeEnv(overrides = {}) {
  return {
    KV: makeFakeKv(),
    DB: makeFakeD1(),
    MONGODB_URI: STUB_SENTINEL,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// createStore — all flag combos with STUB_SENTINEL
// ---------------------------------------------------------------------------

describe("createStore with STUB_SENTINEL — zero MongoClient.connect calls", () => {
  it("STORAGE_PRIMARY=kv, DUAL_WRITE=1 → CFKVStore only", () => {
    const env = makeEnv({ STORAGE_PRIMARY: "kv", DUAL_WRITE: "1" });
    const store = createStore("wordle", env);
    expect(store).toBeDefined();
    expect(connectSpy).not.toHaveBeenCalled();
    // Not a dual store — _kind should be undefined on the wrapper.
    expect(store._kind).not.toBe("dual");
  });

  it("STORAGE_PRIMARY=kv, DUAL_WRITE=0 → CFKVStore only", () => {
    const env = makeEnv({ STORAGE_PRIMARY: "kv", DUAL_WRITE: "0" });
    createStore("wordle", env);
    expect(connectSpy).not.toHaveBeenCalled();
  });

  it("STORAGE_PRIMARY=mongo, DUAL_WRITE=1 → CFKVStore only (sentinel short-circuits)", () => {
    const env = makeEnv({ STORAGE_PRIMARY: "mongo", DUAL_WRITE: "1" });
    createStore("wordle", env);
    expect(connectSpy).not.toHaveBeenCalled();
  });

  it("STORAGE_PRIMARY=mongo, DUAL_WRITE=0 → CFKVStore only (sentinel short-circuits)", () => {
    const env = makeEnv({ STORAGE_PRIMARY: "mongo", DUAL_WRITE: "0" });
    createStore("wordle", env);
    expect(connectSpy).not.toHaveBeenCalled();
  });

  it("MONGODB_URI unset → CFKVStore only", () => {
    const env = makeEnv({ MONGODB_URI: undefined });
    createStore("wordle", env);
    expect(connectSpy).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// createSqlStore — all flag combos with STUB_SENTINEL
// ---------------------------------------------------------------------------

describe("createSqlStore with STUB_SENTINEL — zero MongoClient.connect calls", () => {
  it("STORAGE_PRIMARY=kv, DUAL_WRITE=1 → CFSqlStore only", () => {
    const env = makeEnv({ STORAGE_PRIMARY: "kv", DUAL_WRITE: "1" });
    const sql = createSqlStore("trading", env);
    expect(sql).not.toBeNull();
    expect(connectSpy).not.toHaveBeenCalled();
  });

  it("STORAGE_PRIMARY=kv, DUAL_WRITE=0 → CFSqlStore only", () => {
    const env = makeEnv({ STORAGE_PRIMARY: "kv", DUAL_WRITE: "0" });
    createSqlStore("trading", env);
    expect(connectSpy).not.toHaveBeenCalled();
  });

  it("STORAGE_PRIMARY=mongo, DUAL_WRITE=1 → CFSqlStore only (sentinel short-circuits)", () => {
    const env = makeEnv({ STORAGE_PRIMARY: "mongo", DUAL_WRITE: "1" });
    createSqlStore("trading", env);
    expect(connectSpy).not.toHaveBeenCalled();
  });

  it("STORAGE_PRIMARY=mongo, DUAL_WRITE=0 → CFSqlStore only (sentinel short-circuits)", () => {
    const env = makeEnv({ STORAGE_PRIMARY: "mongo", DUAL_WRITE: "0" });
    createSqlStore("trading", env);
    expect(connectSpy).not.toHaveBeenCalled();
  });

  it("MONGODB_URI unset → CFSqlStore only, null when DB absent", () => {
    // No DB — should return null, no Mongo.
    const env = { KV: makeFakeKv(), MONGODB_URI: undefined };
    const sql = createSqlStore("trading", env);
    expect(sql).toBeNull();
    expect(connectSpy).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// STUB_SENTINEL constant value
// ---------------------------------------------------------------------------

describe("STUB_SENTINEL constant", () => {
  it("is a non-empty string", () => {
    expect(typeof STUB_SENTINEL).toBe("string");
    expect(STUB_SENTINEL.length).toBeGreaterThan(0);
  });

  it("equals the sentinel used inside the factories", () => {
    // The factories hardcode "__stub_mongo__" — must match the exported constant.
    expect(STUB_SENTINEL).toBe("__stub_mongo__");
  });
});

// ---------------------------------------------------------------------------
// Flag matrix — non-sentinel creates DualKVStore (and does NOT call connect here)
// ---------------------------------------------------------------------------

describe("createStore with real URI — returns DualKVStore (_kind=dual)", () => {
  it("STORAGE_PRIMARY=kv, DUAL_WRITE=1, real URI → dual store", () => {
    const env = {
      KV: makeFakeKv(),
      MONGODB_URI: "mongodb://fake",
      STORAGE_PRIMARY: "kv",
      DUAL_WRITE: "1",
    };
    const store = createStore("wordle", env);
    // The wrapper object itself is plain, but the underlying DualKVStore has _kind.
    // Access via the wrapper's _kind (forwarded in withPrefix).
    expect(store._kind).toBe("dual");
  });

  it("STORAGE_PRIMARY=kv, DUAL_WRITE=0, real URI → CF-only (rollback path)", () => {
    const env = {
      KV: makeFakeKv(),
      MONGODB_URI: "mongodb://fake",
      STORAGE_PRIMARY: "kv",
      DUAL_WRITE: "0",
    };
    const store = createStore("wordle", env);
    expect(store._kind).not.toBe("dual");
  });

  it("STORAGE_PRIMARY=mongo, DUAL_WRITE=1, real URI → dual store (Mongo primary)", () => {
    const env = {
      KV: makeFakeKv(),
      MONGODB_URI: "mongodb://fake",
      STORAGE_PRIMARY: "mongo",
      DUAL_WRITE: "1",
    };
    const store = createStore("wordle", env);
    expect(store._kind).toBe("dual");
  });

  it("STORAGE_PRIMARY=mongo, DUAL_WRITE=0, real URI → MongoKVStore only", () => {
    const env = {
      KV: makeFakeKv(),
      MONGODB_URI: "mongodb://fake",
      STORAGE_PRIMARY: "mongo",
      DUAL_WRITE: "0",
    };
    const store = createStore("wordle", env);
    // MongoKVStore has no _kind; wrapper forwards undefined.
    expect(store._kind).toBeUndefined();
  });
});
