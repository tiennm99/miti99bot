import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { startTiming, takeColdFlag } from "../../src/util/timing.js";

// ── startTiming ──────────────────────────────────────────────────────────────

describe("startTiming", () => {
  let consoleSpy;

  beforeEach(() => {
    consoleSpy = vi.spyOn(console, "log").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("end() emits a cmd_timing JSON line via console.log", () => {
    const t = startTiming("/wordle");
    t.end();

    expect(consoleSpy).toHaveBeenCalledOnce();
    const raw = consoleSpy.mock.calls[0][0];
    const obj = JSON.parse(raw);
    expect(obj.event).toBe("cmd_timing");
    expect(obj.cmd).toBe("/wordle");
    expect(typeof obj.total).toBe("number");
    expect(obj.total).toBeGreaterThanOrEqual(0);
    expect(Array.isArray(obj.marks)).toBe(true);
  });

  it("end() merges extra fields into the log object", () => {
    const t = startTiming("/loldle");
    t.end({ cold: true, isolateAgeMs: 42 });

    const obj = JSON.parse(consoleSpy.mock.calls[0][0]);
    expect(obj.cold).toBe(true);
    expect(obj.isolateAgeMs).toBe(42);
  });

  it("mark() records named checkpoints with dt >= 0", () => {
    const t = startTiming("/help");
    t.mark("mongo-read");
    t.mark("handler-done");
    t.end();

    const obj = JSON.parse(consoleSpy.mock.calls[0][0]);
    expect(obj.marks).toHaveLength(2);
    expect(obj.marks[0].label).toBe("mongo-read");
    expect(obj.marks[1].label).toBe("handler-done");
    expect(obj.marks[0].dt).toBeGreaterThanOrEqual(0);
    expect(obj.marks[1].dt).toBeGreaterThanOrEqual(obj.marks[0].dt);
  });

  it("marks array is empty when no mark() calls made", () => {
    const t = startTiming("/ping");
    t.end();
    const obj = JSON.parse(consoleSpy.mock.calls[0][0]);
    expect(obj.marks).toEqual([]);
  });

  it("marks is declared at top — no ReferenceError if mark() called before any await", () => {
    // This would throw if `marks` were declared after a conditional or inside a closure.
    expect(() => {
      const t = startTiming("/fortytwo");
      t.mark("immediate");
      t.end();
    }).not.toThrow();
  });

  it("total reflects elapsed time (>= 0ms)", () => {
    const t = startTiming("/trade");
    t.end();
    const obj = JSON.parse(consoleSpy.mock.calls[0][0]);
    expect(obj.total).toBeGreaterThanOrEqual(0);
  });
});

// ── takeColdFlag ─────────────────────────────────────────────────────────────
// NOTE: takeColdFlag uses module-scope state that persists across tests in the
// same vitest worker. We cannot reset it between tests — so we capture the
// baseline once and assert relative behaviour only.

describe("takeColdFlag", () => {
  it("returns an object with cold (boolean) and isolateAgeMs (number)", () => {
    const result = takeColdFlag();
    expect(typeof result.cold).toBe("boolean");
    expect(typeof result.isolateAgeMs).toBe("number");
    expect(result.isolateAgeMs).toBeGreaterThanOrEqual(0);
  });

  it("second call in same isolate returns cold=false", () => {
    // The first call may have already flipped the flag in a prior test.
    // We call it once here just to ensure it returns cold=false on subsequent invocations.
    takeColdFlag(); // may be first or subsequent
    const second = takeColdFlag();
    expect(second.cold).toBe(false);
  });

  it("isolateAgeMs is non-decreasing across successive calls", () => {
    const a = takeColdFlag();
    const b = takeColdFlag();
    expect(b.isolateAgeMs).toBeGreaterThanOrEqual(a.isolateAgeMs);
  });
});
