import { describe, expect, it } from "vitest";
import { findChampion } from "../../../src/modules/loldle-ability/lookup.js";

const pool = [
  { championName: "Ahri", abilities: [{ slot: "Q", name: "Orb of Deception", icon: "x" }] },
  { championName: "Akali", abilities: [{ slot: "R", name: "Perfect Execution", icon: "x" }] },
  { championName: "Miss Fortune", abilities: [{ slot: "P", name: "Love Tap", icon: "x" }] },
];

describe("findChampion (ability pool)", () => {
  it("matches case-insensitively", () => {
    expect(findChampion(pool, "ahri").championName).toBe("Ahri");
  });

  it("normalizes punctuation and spaces", () => {
    expect(findChampion(pool, "MissFortune").championName).toBe("Miss Fortune");
    expect(findChampion(pool, "miss fortune").championName).toBe("Miss Fortune");
  });

  it("unique prefix resolves", () => {
    expect(findChampion(pool, "mi").championName).toBe("Miss Fortune");
    expect(findChampion(pool, "ak").championName).toBe("Akali");
  });

  it("returns null on empty / no-match / ambiguous", () => {
    expect(findChampion(pool, "")).toBeNull();
    expect(findChampion(pool, "zzz")).toBeNull();
    expect(findChampion(pool, "a")).toBeNull();
  });
});
