/**
 * @file dual-kv-store.test.js — unit tests for DualKVStore.
 *
 * Tests the four behavioral contracts:
 *   1. Write succeeds when both primary and secondary succeed.
 *   2. Write succeeds when secondary fails: logs + enqueues to retry queue.
 *   3. Write fails (throws) when primary fails.
 *   4. Reads always come from primary only.
 *   5. `_kind === "dual"` sentinel is present on the instance.
 */

import { beforeEach, describe, expect, it, vi } from "vitest";
import { CFKVStore } from "../../src/db/cf-kv-store.js";
import { DualKVStore } from "../../src/db/dual-kv-store.js";
import { makeFakeKv } from "../fakes/fake-kv-namespace.js";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeStores() {
  const primaryKv = makeFakeKv();
  const secondaryKv = makeFakeKv();
  const retryQueueKv = makeFakeKv();
  const primary = new CFKVStore(primaryKv);
  const secondary = new CFKVStore(secondaryKv);
  const logger = { warn: vi.fn(), error: vi.fn(), log: vi.fn() };
  const dual = new DualKVStore(primary, secondary, retryQueueKv, logger);
  return { primaryKv, secondaryKv, retryQueueKv, primary, secondary, dual, logger };
}

// ---------------------------------------------------------------------------
// Constructor validation
// ---------------------------------------------------------------------------

describe("DualKVStore constructor", () => {
  it("throws when primary is missing", () => {
    const kv = makeFakeKv();
    expect(() => new DualKVStore(null, new CFKVStore(kv), kv)).toThrow(/primary/);
  });

  it("throws when secondary is missing", () => {
    const kv = makeFakeKv();
    expect(() => new DualKVStore(new CFKVStore(kv), null, kv)).toThrow(/secondary/);
  });

  it("throws when rawKv is missing", () => {
    const kv = makeFakeKv();
    const store = new CFKVStore(kv);
    expect(() => new DualKVStore(store, store, null)).toThrow(/rawKv/);
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
// Read operations — primary only
// ---------------------------------------------------------------------------

describe("reads from primary only", () => {
  it("get() returns primary value even when secondary differs", async () => {
    const { primaryKv, secondaryKv, dual } = makeStores();
    primaryKv.store.set("k", "from-primary");
    secondaryKv.store.set("k", "from-secondary");
    expect(await dual.get("k")).toBe("from-primary");
  });

  it("getJSON() returns primary parsed value", async () => {
    const { primaryKv, secondaryKv, dual } = makeStores();
    primaryKv.store.set("k", JSON.stringify({ x: 1 }));
    secondaryKv.store.set("k", JSON.stringify({ x: 99 }));
    expect(await dual.getJSON("k")).toEqual({ x: 1 });
  });

  it("list() returns primary keys", async () => {
    const { primaryKv, secondaryKv, dual } = makeStores();
    primaryKv.store.set("a:1", "x");
    primaryKv.store.set("a:2", "y");
    secondaryKv.store.set("b:1", "other-module");
    const result = await dual.list({ prefix: "a:" });
    expect(result.keys.sort()).toEqual(["a:1", "a:2"]);
  });
});

// ---------------------------------------------------------------------------
// Write operations — both succeed
// ---------------------------------------------------------------------------

describe("writes to both when both succeed", () => {
  it("put() writes to primary and secondary", async () => {
    const { primaryKv, secondaryKv, dual } = makeStores();
    await dual.put("k", "v");
    expect(primaryKv.store.get("k")).toBe("v");
    expect(secondaryKv.store.get("k")).toBe("v");
  });

  it("putJSON() serialises to both", async () => {
    const { primaryKv, secondaryKv, dual } = makeStores();
    await dual.putJSON("k", { n: 42 });
    expect(primaryKv.store.get("k")).toBe('{"n":42}');
    expect(secondaryKv.store.get("k")).toBe('{"n":42}');
  });

  it("delete() removes from both", async () => {
    const { primaryKv, secondaryKv, dual } = makeStores();
    primaryKv.store.set("k", "v");
    secondaryKv.store.set("k", "v");
    await dual.delete("k");
    expect(primaryKv.store.has("k")).toBe(false);
    expect(secondaryKv.store.has("k")).toBe(false);
  });

  it("no retry entry is enqueued when both succeed", async () => {
    const { retryQueueKv, dual } = makeStores();
    await dual.put("k", "v");
    expect(retryQueueKv.store.size).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Secondary failure — caller still succeeds, retry enqueued
// ---------------------------------------------------------------------------

describe("secondary failure — caller succeeds, retry enqueued", () => {
  it("put() succeeds when secondary throws, logs warning, enqueues retry", async () => {
    const { primaryKv, secondaryKv, retryQueueKv, logger, dual } = makeStores();

    // Make secondary put throw.
    vi.spyOn(secondaryKv, "put").mockRejectedValueOnce(new Error("network error"));

    await expect(dual.put("key1", "value1")).resolves.not.toThrow();

    // Primary was written.
    expect(primaryKv.store.get("key1")).toBe("value1");
    // Warning logged — contains key and error info, NOT the value.
    expect(logger.warn).toHaveBeenCalledOnce();
    const warnArg = logger.warn.mock.calls[0][1];
    expect(warnArg.key).toBe("key1");
    expect(warnArg.err).toContain("network error");
    // Value must NOT appear in the log.
    expect(JSON.stringify(warnArg)).not.toContain("value1");
    // Retry entry enqueued.
    expect(retryQueueKv.store.size).toBe(1);
    const [retryKey] = [...retryQueueKv.store.keys()];
    expect(retryKey).toMatch(/^__retry:mongo-failed:/);
  });

  it("putJSON() succeeds when secondary throws, enqueues retry", async () => {
    const { primaryKv, secondaryKv, retryQueueKv, dual } = makeStores();
    vi.spyOn(secondaryKv, "put").mockRejectedValueOnce(new Error("timeout"));

    await expect(dual.putJSON("k2", { val: 7 })).resolves.not.toThrow();

    expect(primaryKv.store.get("k2")).toBe('{"val":7}');
    expect(retryQueueKv.store.size).toBe(1);
  });

  it("delete() succeeds when secondary throws, enqueues retry", async () => {
    const { primaryKv, secondaryKv, retryQueueKv, dual } = makeStores();
    primaryKv.store.set("k3", "v");
    secondaryKv.store.set("k3", "v");
    vi.spyOn(secondaryKv, "delete").mockRejectedValueOnce(new Error("atlas down"));

    await expect(dual.delete("k3")).resolves.not.toThrow();

    expect(primaryKv.store.has("k3")).toBe(false);
    expect(retryQueueKv.store.size).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// Primary failure — caller throws
// ---------------------------------------------------------------------------

describe("primary failure — throws to caller", () => {
  it("put() throws when primary throws", async () => {
    const { primaryKv, dual } = makeStores();
    vi.spyOn(primaryKv, "put").mockRejectedValueOnce(new Error("primary dead"));

    await expect(dual.put("k", "v")).rejects.toThrow("primary dead");
  });

  it("putJSON() throws when primary throws", async () => {
    const { primaryKv, dual } = makeStores();
    vi.spyOn(primaryKv, "put").mockRejectedValueOnce(new Error("kv full"));

    await expect(dual.putJSON("k", { x: 1 })).rejects.toThrow("kv full");
  });

  it("delete() throws when primary throws", async () => {
    const { primaryKv, dual } = makeStores();
    vi.spyOn(primaryKv, "delete").mockRejectedValueOnce(new Error("kv gone"));

    await expect(dual.delete("k")).rejects.toThrow("kv gone");
  });
});
