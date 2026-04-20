import { describe, expect, it } from "vitest";
import { findChampion } from "../../../src/modules/loldle/lookup.js";

const champions = [
  { id: "Aatrox", name: "Aatrox" },
  { id: "Ahri", name: "Ahri" },
  { id: "KhaZix", name: "Kha'Zix" },
  { id: "MissFortune", name: "Miss Fortune" },
];

describe("findChampion", () => {
  it("matches by exact id (case-insensitive)", () => {
    expect(findChampion(champions, "aatrox").id).toBe("Aatrox");
    expect(findChampion(champions, "AATROX").id).toBe("Aatrox");
  });

  it("normalizes punctuation and spaces", () => {
    expect(findChampion(champions, "kha'zix").id).toBe("KhaZix");
    expect(findChampion(champions, "miss fortune").id).toBe("MissFortune");
    expect(findChampion(champions, "MissFortune").id).toBe("MissFortune");
  });

  it("returns null for non-matching input", () => {
    expect(findChampion(champions, "zzz")).toBeNull();
    expect(findChampion(champions, "")).toBeNull();
  });

  it("falls back to unique prefix match", () => {
    expect(findChampion(champions, "aat").id).toBe("Aatrox");
  });

  it("prefix match returns null on ambiguity", () => {
    const ambig = [...champions, { id: "Aatrox2", name: "Aatrox 2" }];
    expect(findChampion(ambig, "aa")).toBeNull();
  });
});
