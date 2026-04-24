import { describe, expect, it } from "vitest";
import { findChampion } from "../../../src/modules/loldle-quote/lookup.js";

const pool = [
  { championName: "Garen", quote: "the Might of Demacia" },
  { championName: "Gangplank", quote: "the Saltwater Scourge" },
  { championName: "Miss Fortune", quote: "the Bounty Hunter" },
];

describe("findChampion (quote pool)", () => {
  it("matches case- and punctuation-insensitively", () => {
    expect(findChampion(pool, "garen").championName).toBe("Garen");
    expect(findChampion(pool, "miss fortune").championName).toBe("Miss Fortune");
    expect(findChampion(pool, "MissFortune").championName).toBe("Miss Fortune");
  });

  it("ambiguous prefix returns null", () => {
    expect(findChampion(pool, "ga")).toBeNull();
  });

  it("unique prefix resolves", () => {
    expect(findChampion(pool, "gar").championName).toBe("Garen");
    expect(findChampion(pool, "gang").championName).toBe("Gangplank");
  });
});
