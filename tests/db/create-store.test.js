import { beforeEach, describe, expect, it } from "vitest";
import { createStore } from "../../src/db/create-store.js";
import { makeFakeKv } from "../fakes/fake-kv-namespace.js";

describe("createStore (prefixing factory)", () => {
  let kv;
  let env;

  beforeEach(() => {
    kv = makeFakeKv();
    env = { KV: kv };
  });

  it("rejects invalid module names", () => {
    expect(() => createStore("", env)).toThrow();
    expect(() => createStore("BadName", env)).toThrow(/invalid/);
    expect(() => createStore("has:colon", env)).toThrow(/invalid/);
    expect(() => createStore("ok_name", env)).not.toThrow();
    expect(() => createStore("kebab-ok", env)).not.toThrow();
  });

  it("rejects missing env.KV", () => {
    expect(() => createStore("wordle", {})).toThrow(/KV/);
  });

  it("put() writes raw key with module: prefix", async () => {
    const store = createStore("wordle", env);
    await store.put("games_played", "42");
    expect(kv.store.get("wordle:games_played")).toBe("42");
  });

  it("get() reads through prefix", async () => {
    const store = createStore("wordle", env);
    await store.put("k", "v");
    expect(await store.get("k")).toBe("v");
  });

  it("delete() deletes the prefixed raw key", async () => {
    const store = createStore("wordle", env);
    await store.put("k", "v");
    await store.delete("k");
    expect(kv.store.has("wordle:k")).toBe(false);
  });

  it("list() combines module prefix + caller prefix and strips the module prefix on return", async () => {
    kv.store.set("wordle:games:1", "a");
    kv.store.set("wordle:games:2", "b");
    kv.store.set("wordle:stats", "c");
    kv.store.set("loldle:games:1", "other");

    const store = createStore("wordle", env);
    const res = await store.list({ prefix: "games:" });
    expect(res.keys.sort()).toEqual(["games:1", "games:2"]);
  });

  it("two stores are fully isolated — one cannot see the other's keys", async () => {
    const wordle = createStore("wordle", env);
    const loldle = createStore("loldle", env);
    await wordle.put("secret", "w");
    await loldle.put("secret", "l");

    expect(await wordle.get("secret")).toBe("w");
    expect(await loldle.get("secret")).toBe("l");

    const wList = await wordle.list();
    const lList = await loldle.list();
    expect(wList.keys).toEqual(["secret"]);
    expect(lList.keys).toEqual(["secret"]);
  });

  it("putJSON/getJSON through a prefixed store persist at <module>:<key>", async () => {
    const store = createStore("misc", env);
    await store.putJSON("last_ping", { at: 123 });
    expect(kv.store.get("misc:last_ping")).toBe('{"at":123}');
    expect(await store.getJSON("last_ping")).toEqual({ at: 123 });
  });
});
