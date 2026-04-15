import { beforeEach, describe, expect, it, vi } from "vitest";
import { dispatchScheduled } from "../../src/modules/cron-dispatcher.js";
import { makeFakeKv } from "../fakes/fake-kv-namespace.js";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeFakeRegistry(entries) {
  return { crons: entries };
}

function makeModule(name) {
  return { name };
}

function makeCronEntry(moduleName, schedule, name, handler) {
  return { module: makeModule(moduleName), schedule, name, handler };
}

function makeFakeCtx() {
  const promises = [];
  return {
    ctx: { waitUntil: (p) => promises.push(p) },
    flush: () => Promise.all(promises),
  };
}

function makeFakeEnv() {
  return { KV: makeFakeKv(), DB: null, MODULES: "test" };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("dispatchScheduled", () => {
  it("calls handler for matching schedule", async () => {
    const called = [];
    const handler = vi.fn(async () => called.push("ran"));
    const reg = makeFakeRegistry([makeCronEntry("mod", "0 1 * * *", "nightly", handler)]);
    const { ctx, flush } = makeFakeCtx();

    dispatchScheduled({ cron: "0 1 * * *" }, makeFakeEnv(), ctx, reg);
    await flush();

    expect(handler).toHaveBeenCalledOnce();
    expect(called).toEqual(["ran"]);
  });

  it("does NOT call handler when schedule does not match", async () => {
    const handler = vi.fn();
    const reg = makeFakeRegistry([makeCronEntry("mod", "0 2 * * *", "other", handler)]);
    const { ctx, flush } = makeFakeCtx();

    dispatchScheduled({ cron: "0 1 * * *" }, makeFakeEnv(), ctx, reg);
    await flush();

    expect(handler).not.toHaveBeenCalled();
  });

  it("fan-out: two modules sharing same schedule both fire", async () => {
    const callLog = [];
    const handlerA = async () => callLog.push("a");
    const handlerB = async () => callLog.push("b");
    const reg = makeFakeRegistry([
      makeCronEntry("mod-a", "*/5 * * * *", "tick-a", handlerA),
      makeCronEntry("mod-b", "*/5 * * * *", "tick-b", handlerB),
    ]);
    const { ctx, flush } = makeFakeCtx();

    dispatchScheduled({ cron: "*/5 * * * *" }, makeFakeEnv(), ctx, reg);
    await flush();

    expect(callLog.sort()).toEqual(["a", "b"]);
  });

  it("error isolation: one handler throwing does not prevent others", async () => {
    const callLog = [];
    const failing = async () => {
      throw new Error("boom");
    };
    const surviving = async () => callLog.push("survived");
    const reg = makeFakeRegistry([
      makeCronEntry("mod-a", "0 0 * * *", "fail", failing),
      makeCronEntry("mod-b", "0 0 * * *", "ok", surviving),
    ]);
    const { ctx, flush } = makeFakeCtx();
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    dispatchScheduled({ cron: "0 0 * * *" }, makeFakeEnv(), ctx, reg);
    await flush();

    expect(callLog).toEqual(["survived"]);
    expect(consoleSpy).toHaveBeenCalledOnce();
    expect(consoleSpy.mock.calls[0][0]).toMatch(/cron.*fail.*mod-a/);
    consoleSpy.mockRestore();
  });

  it("passes event and { db, sql, env } to handler", async () => {
    let received;
    const handler = async (event, handlerCtx) => {
      received = { event, handlerCtx };
    };
    const env = makeFakeEnv();
    const reg = makeFakeRegistry([makeCronEntry("mod", "0 3 * * *", "ctx-check", handler)]);
    const { ctx, flush } = makeFakeCtx();
    const fakeEvent = { cron: "0 3 * * *", scheduledTime: 123 };

    dispatchScheduled(fakeEvent, env, ctx, reg);
    await flush();

    expect(received.event).toBe(fakeEvent);
    expect(typeof received.handlerCtx.db).toBe("object");
    expect(typeof received.handlerCtx.sql).toBe("object");
    expect(received.handlerCtx.env).toBe(env);
  });

  it("no-op when registry has no cron entries", async () => {
    const reg = makeFakeRegistry([]);
    const { ctx, flush } = makeFakeCtx();
    // Should not throw.
    expect(() => dispatchScheduled({ cron: "0 1 * * *" }, makeFakeEnv(), ctx, reg)).not.toThrow();
    await flush();
  });
});
