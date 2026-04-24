import { describe, expect, it } from "vitest";
import { SEEDS, getRandomSeed } from "../../../src/modules/twentyq/seeds.js";

describe("twentyq/seeds", () => {
  it("every seed is a non-empty lowercase string", () => {
    for (const s of SEEDS) {
      expect(typeof s).toBe("string");
      expect(s.length).toBeGreaterThan(0);
      expect(s).toBe(s.toLowerCase());
    }
  });

  it("no duplicate seeds", () => {
    expect(new Set(SEEDS).size).toBe(SEEDS.length);
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
