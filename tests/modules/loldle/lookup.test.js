import { describe, expect, it } from "vitest";
import { findChampion } from "../../../src/modules/loldle/lookup.js";

const champions = [
  { championName: "Aatrox" },
  { championName: "Ahri" },
  { championName: "Kha'Zix" },
  { championName: "Miss Fortune" },
];

describe("findChampion", () => {
  it("matches championName (case-insensitive)", () => {
    expect(findChampion(champions, "aatrox").championName).toBe("Aatrox");
    expect(findChampion(champions, "AATROX").championName).toBe("Aatrox");
  });

  it("normalizes punctuation and spaces", () => {
    expect(findChampion(champions, "kha'zix").championName).toBe("Kha'Zix");
    expect(findChampion(champions, "khazix").championName).toBe("Kha'Zix");
    expect(findChampion(champions, "miss fortune").championName).toBe("Miss Fortune");
    expect(findChampion(champions, "MissFortune").championName).toBe("Miss Fortune");
  });

  it("returns null for non-matching input", () => {
    expect(findChampion(champions, "zzz")).toBeNull();
    expect(findChampion(champions, "")).toBeNull();
  });

  it("falls back to unique prefix match", () => {
    expect(findChampion(champions, "aat").championName).toBe("Aatrox");
  });

  it("prefix match returns null on ambiguity", () => {
    const ambig = [...champions, { championName: "Aatrox Prime" }];
    expect(findChampion(ambig, "aa")).toBeNull();
  });
});
