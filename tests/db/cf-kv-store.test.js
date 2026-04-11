import { beforeEach, describe, expect, it, vi } from "vitest";
import { CFKVStore } from "../../src/db/cf-kv-store.js";
import { makeFakeKv } from "../fakes/fake-kv-namespace.js";

describe("CFKVStore", () => {
  let kv;
  let store;

  beforeEach(() => {
    kv = makeFakeKv();
    store = new CFKVStore(kv);
  });

  it("throws when given no binding", () => {
    expect(() => new CFKVStore(null)).toThrow(/required/);
  });

  it("round-trips get/put/delete", async () => {
    expect(await store.get("k")).toBeNull();
    await store.put("k", "v");
    expect(await store.get("k")).toBe("v");
    await store.delete("k");
    expect(await store.get("k")).toBeNull();
  });

  it("list() returns normalized shape with module keys stripped of wrappers", async () => {
    await store.put("a:1", "x");
    await store.put("a:2", "y");
    await store.put("b:1", "z");
    const res = await store.list({ prefix: "a:" });
    expect(res.keys.sort()).toEqual(["a:1", "a:2"]);
    expect(res.done).toBe(true);
  });

  it("list() pagination — cursor propagates through", async () => {
    for (let i = 0; i < 5; i++) await store.put(`k${i}`, String(i));
    const page1 = await store.list({ limit: 2 });
    expect(page1.keys.length).toBe(2);
    expect(page1.done).toBe(false);
    expect(page1.cursor).toBeTruthy();
    const page2 = await store.list({ limit: 2, cursor: page1.cursor });
    expect(page2.keys.length).toBe(2);
    expect(page2.done).toBe(false);
    const page3 = await store.list({ limit: 2, cursor: page2.cursor });
    expect(page3.keys.length).toBe(1);
    expect(page3.done).toBe(true);
    expect(page3.cursor).toBeUndefined();
  });

  it("put() with expirationTtl passes through to binding", async () => {
    await store.put("k", "v", { expirationTtl: 60 });
    expect(kv.putLog).toEqual([{ key: "k", value: "v", opts: { expirationTtl: 60 } }]);
  });

  it("put() without options does NOT pass an options object", async () => {
    await store.put("k", "v");
    expect(kv.putLog[0]).toEqual({ key: "k", value: "v", opts: undefined });
  });

  it("getJSON/putJSON round-trip", async () => {
    await store.putJSON("k", { a: 1, b: [2, 3] });
    expect(await store.getJSON("k")).toEqual({ a: 1, b: [2, 3] });
  });

  it("getJSON returns null on missing key", async () => {
    expect(await store.getJSON("missing")).toBeNull();
  });

  it("getJSON returns null on corrupt JSON and logs a warning", async () => {
    kv.store.set("bad", "{not json");
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    expect(await store.getJSON("bad")).toBeNull();
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });

  it("putJSON throws on undefined value", async () => {
    await expect(store.putJSON("k", undefined)).rejects.toThrow(/undefined/);
  });

  it("putJSON passes expirationTtl through", async () => {
    await store.putJSON("k", { a: 1 }, { expirationTtl: 120 });
    expect(kv.putLog[0].opts).toEqual({ expirationTtl: 120 });
  });
});
