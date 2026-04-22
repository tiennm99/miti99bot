import { describe, expect, it } from "vitest";
import { renderBoard, renderGuess } from "../../../src/modules/loldle/render.js";

/** Mirror the shape compareChampions returns — only the fields render.js reads. */
const sampleResults = [
  { key: "gender", label: "Gender", result: "correct", guessValue: "Male" },
  { key: "species", label: "Species", result: "correct", guessValue: "Human, Darkin" },
  { key: "range_type", label: "Range type", result: "correct", guessValue: "Ranged" },
  { key: "resource", label: "Resource", result: "correct", guessValue: "Mana" },
  { key: "regions", label: "Region(s)", result: "correct", guessValue: "Runeterra" },
  { key: "positions", label: "Position(s)", result: "partial", guessValue: "Jungle, Support" },
  {
    key: "release_date",
    label: "Release year",
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
    expect(out).toContain("🎯 Champion");
    expect(out).toContain("RAKAN");
    expect(out).not.toContain(" Rakan");
  });

  it("auto-widths label column to the longest label (Release year = 12)", () => {
    const out = renderGuess("Brand", sampleResults);
    // "Champion" (8) padded to 12 → 4 trailing + 1 separator = 5 spaces.
    expect(out).toContain("🎯 Champion     BRAND");
    // "Release year" (12) = exact width, single separator.
    expect(out).toContain("❌ Release year 2011");
    // "Gender" (6) padded to 12 → 6 trailing + 1 separator = 7 spaces.
    expect(out).toContain("✅ Gender       Male");
  });

  it("appends ⬆️ / ⬇️ year direction hints only when wrong", () => {
    const up = renderGuess("Brand", sampleResults);
    expect(up).toContain("❌ Release year");
    expect(up).toContain("2011 ⬆️");

    const correctYear = sampleResults.map((r) =>
      r.key === "release_date" ? { ...r, result: "correct", direction: undefined } : r,
    );
    const out = renderGuess("Brand", correctYear);
    expect(out).not.toContain("⬆️");
    expect(out).not.toContain("⬇️");
  });

  it("HTML-escapes values so < and > render literally", () => {
    const evil = [
      { key: "regions", label: "Region(s)", result: "wrong", guessValue: "<script>" },
    ];
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
    expect(out).toContain("🎯 Champion     AHRI");
    expect(out).toContain("🎯 Champion     BRAND");
  });
});
