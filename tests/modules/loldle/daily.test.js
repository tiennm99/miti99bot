import { describe, expect, it } from "vitest";
import { pickDaily, pickRandom, todayUtc } from "../../../src/modules/loldle/daily.js";

describe("picker", () => {
  it("todayUtc returns YYYY-MM-DD", () => {
    expect(todayUtc(new Date("2026-04-20T23:30:00Z"))).toBe("2026-04-20");
    expect(todayUtc(new Date("2026-01-01T00:00:00Z"))).toBe("2026-01-01");
  });

  it("pickDaily is deterministic for same seed", () => {
    const champions = [{ id: "A" }, { id: "B" }, { id: "C" }, { id: "D" }];
    const a = pickDaily(champions, "2026-04-20");
    const b = pickDaily(champions, "2026-04-20");
    expect(a).toBe(b);
  });

  it("pickDaily produces different picks over time", () => {
    const champions = Array.from({ length: 100 }, (_, i) => ({ id: `C${i}` }));
    const picks = new Set();
    for (let d = 1; d <= 30; d++) {
      picks.add(pickDaily(champions, `2026-04-${String(d).padStart(2, "0")}`).id);
    }
    expect(picks.size).toBeGreaterThan(5);
  });

  it("pickDaily throws on empty list", () => {
    expect(() => pickDaily([], "x")).toThrow();
  });

  it("pickRandom honors injected rng", () => {
    const champions = [{ id: "A" }, { id: "B" }, { id: "C" }, { id: "D" }];
    expect(pickRandom(champions, () => 0).id).toBe("A");
    expect(pickRandom(champions, () => 0.999).id).toBe("D");
    expect(pickRandom(champions, () => 0.5).id).toBe("C");
  });

  it("pickRandom throws on empty list", () => {
    expect(() => pickRandom([])).toThrow();
  });
});
