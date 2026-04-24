import { describe, expect, it } from "vitest";
import { SEEDS, getRandomSeed } from "../../../src/modules/twentyq/seeds.js";

describe("twentyq/seeds", () => {
  it("every seed has non-empty category, target, initialHint", () => {
    for (const s of SEEDS) {
      expect(s.category).toBeTruthy();
      expect(s.target).toBeTruthy();
      expect(s.initialHint).toBeTruthy();
      expect(typeof s.category).toBe("string");
      expect(typeof s.target).toBe("string");
      expect(typeof s.initialHint).toBe("string");
    }
  });

  it("all targets are lowercase", () => {
    for (const s of SEEDS) {
      expect(s.target).toBe(s.target.toLowerCase());
    }
  });

  it("initialHint never contains the target word", () => {
    for (const s of SEEDS) {
      const hintLower = s.initialHint.toLowerCase();
      const re = new RegExp(`\\b${s.target}\\b`, "i");
      expect(re.test(hintLower)).toBe(false);
    }
  });

  it("getRandomSeed returns a member of SEEDS", () => {
    const s = getRandomSeed(() => 0.5);
    expect(SEEDS).toContain(s);
  });

  it("getRandomSeed deterministic with stub rng", () => {
    const a = getRandomSeed(() => 0);
    const b = getRandomSeed(() => 0);
    expect(a).toBe(b);
    expect(a).toBe(SEEDS[0]);
  });

  it("rng returning 0.999... still indexes within bounds", () => {
    const s = getRandomSeed(() => 0.99999);
    expect(s).toBe(SEEDS[SEEDS.length - 1]);
  });
});
