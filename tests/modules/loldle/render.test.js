import { describe, expect, it } from "vitest";
import { renderBoard, renderGuess } from "../../../src/modules/loldle/render.js";

/** Mirror the shape compareChampions returns — only the fields render.js reads. */
const sampleResults = [
  { key: "gender", label: "Gender", result: "correct", guessValue: "Male" },
  { key: "species", label: "Species", result: "correct", guessValue: "Human, Darkin" },
  { key: "attackType", label: "Range", result: "correct", guessValue: "Ranged" },
  { key: "resource", label: "Resource", result: "correct", guessValue: "Mana" },
  { key: "region", label: "Region", result: "correct", guessValue: "Runeterra" },
  { key: "lane", label: "Lane", result: "partial", guessValue: "Jungle, Support" },
  {
    key: "release_date",
    label: "Year",
    result: "wrong",
    direction: "up",
    guessValue: "2011",
  },
];

describe("renderGuess", () => {
  it("wraps output in <pre> for Telegram HTML monospace", () => {
    const out = renderGuess("Brand", sampleResults);
    expect(out.startsWith("<pre>")).toBe(true);
    expect(out.endsWith("</pre>")).toBe(true);
  });

  it("uppercases champion name on the 🎯 row", () => {
    const out = renderGuess("Rakan", sampleResults);
    expect(out).toContain("🎯 Name");
    expect(out).toContain("RAKAN");
    expect(out).not.toContain(" Rakan");
  });

  it("auto-widths label column to the longest label (Resource = 8)", () => {
    const out = renderGuess("Brand", sampleResults);
    // "Name" (4) padded to 8 → 4 trailing spaces + the single separator space.
    expect(out).toContain("🎯 Name     BRAND");
    // "Resource" (8) = exact width, single separator space before value.
    expect(out).toContain("✅ Resource Mana");
    // "Gender" (6) padded to 8 → 2 trailing + 1 separator = 3 spaces before value.
    expect(out).toContain("✅ Gender   Male");
  });

  it("appends ⬆️ / ⬇️ year direction hints only when wrong", () => {
    const up = renderGuess("Brand", sampleResults);
    expect(up).toContain("❌ Year");
    expect(up).toContain("2011 ⬆️");

    const correctYear = sampleResults.map((r) =>
      r.key === "release_date" ? { ...r, result: "correct", direction: undefined } : r,
    );
    const out = renderGuess("Brand", correctYear);
    expect(out).not.toContain("⬆️");
    expect(out).not.toContain("⬇️");
  });

  it("HTML-escapes values so < and > render literally", () => {
    const evil = [{ key: "region", label: "Region", result: "wrong", guessValue: "<script>" }];
    const out = renderGuess("Foo", evil);
    expect(out).toContain("&lt;script&gt;");
    expect(out).not.toContain("<script>");
  });
});

describe("renderBoard", () => {
  it("returns an empty-state prompt when there are no guesses", () => {
    expect(renderBoard([])).toContain("No guesses yet");
  });

  it("stacks multiple guesses in one <pre> with aligned labels", () => {
    const out = renderBoard([
      { champion: "Ahri", results: sampleResults },
      { champion: "Brand", results: sampleResults },
    ]);
    expect(out.startsWith("<pre>")).toBe(true);
    expect(out.endsWith("</pre>")).toBe(true);
    expect(out).toContain("🎯 Name     AHRI");
    expect(out).toContain("🎯 Name     BRAND");
  });
});
