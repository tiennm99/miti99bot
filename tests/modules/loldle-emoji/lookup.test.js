import { describe, expect, it } from "vitest";
import { findChampion } from "../../../src/modules/loldle-emoji/lookup.js";

const pool = [
  { championName: "Aatrox", emojis: "⚔️ 🌍 💪" },
  { championName: "Ahri", emojis: "🦊 🏯 🔮" },
  { championName: "Kha'Zix", emojis: "👾 👾 💪" },
  { championName: "Miss Fortune", emojis: "🧝 ⚓ 🔮" },
];

describe("findChampion (emoji pool)", () => {
  it("matches case-insensitively", () => {
    expect(findChampion(pool, "aatrox").championName).toBe("Aatrox");
    expect(findChampion(pool, "AHRI").championName).toBe("Ahri");
  });

  it("normalizes punctuation and spaces", () => {
    expect(findChampion(pool, "kha'zix").championName).toBe("Kha'Zix");
    expect(findChampion(pool, "khazix").championName).toBe("Kha'Zix");
    expect(findChampion(pool, "miss fortune").championName).toBe("Miss Fortune");
    expect(findChampion(pool, "MissFortune").championName).toBe("Miss Fortune");
  });

  it("returns null for empty input or no match", () => {
    expect(findChampion(pool, "")).toBeNull();
    expect(findChampion(pool, "zzz")).toBeNull();
  });

  it("unique prefix resolves; ambiguous prefix returns null", () => {
    expect(findChampion(pool, "aat").championName).toBe("Aatrox");
    const ambig = [...pool, { championName: "Aatrox Prime", emojis: "⚔️" }];
    expect(findChampion(ambig, "aa")).toBeNull();
  });
});
