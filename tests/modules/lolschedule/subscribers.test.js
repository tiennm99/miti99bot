import { beforeEach, describe, expect, it } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  addSubscriber,
  listSubscribers,
  removeSubscriber,
} from "../../../src/modules/lolschedule/subscribers.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

describe("subscribers", () => {
  let db;
  beforeEach(() => {
    db = createStore("lolschedule", { KV: makeFakeKv() });
  });

  it("starts empty", async () => {
    expect(await listSubscribers(db)).toEqual([]);
  });

  it("adds a new subscriber and returns true", async () => {
    expect(await addSubscriber(db, 42)).toBe(true);
    expect(await listSubscribers(db)).toEqual([42]);
  });

  it("add is idempotent for an already-subscribed chat", async () => {
    await addSubscriber(db, 42);
    expect(await addSubscriber(db, 42)).toBe(false);
    expect(await listSubscribers(db)).toEqual([42]);
  });

  it("removes an existing subscriber and returns true", async () => {
    await addSubscriber(db, 1);
    await addSubscriber(db, 2);
    expect(await removeSubscriber(db, 1)).toBe(true);
    expect(await listSubscribers(db)).toEqual([2]);
  });

  it("remove on a non-subscriber returns false and is a no-op", async () => {
    await addSubscriber(db, 1);
    expect(await removeSubscriber(db, 99)).toBe(false);
    expect(await listSubscribers(db)).toEqual([1]);
  });
});
