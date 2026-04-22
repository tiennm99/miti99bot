import { describe, expect, it } from "vitest";
import { renderBoard, renderGuess } from "../../../src/modules/semantle/render.js";

describe("semantle/render", () => {
  describe("renderBoard", () => {
    it("shows round ready prompt when no guesses", () => {
      const result = renderBoard([]);
      expect(result).toContain("🎯 Semantle — 0 guesses");
      expect(result).toContain("🆕 Round ready");
      expect(result).toContain("/semantle");
    });

    it("shows singular 'guess' for exactly one guess", () => {
      const result = renderBoard([{ word: "test", canonical: "test", similarity: 0.5 }]);
      expect(result).toContain("1 guess");
      expect(result).not.toContain("guesses");
    });

    it("shows plural 'guesses' for multiple guesses", () => {
      const result = renderBoard([
        { word: "test", canonical: "test", similarity: 0.5 },
        { word: "best", canonical: "best", similarity: 0.6 },
      ]);
      expect(result).toContain("2 guesses");
    });

    it("sorts guesses by similarity descending", () => {
      const guesses = [
        { word: "low", canonical: "low", similarity: 0.2 },
        { word: "high", canonical: "high", similarity: 0.9 },
        { word: "mid", canonical: "mid", similarity: 0.5 },
      ];
      const result = renderBoard(guesses);

      const lines = result.split("\n");
      // Find index of highest and lowest in the pre block
      const highIdx = lines.findIndex((l) => l.includes("high"));
      const midIdx = lines.findIndex((l) => l.includes("mid"));
      const lowIdx = lines.findIndex((l) => l.includes("low"));

      expect(highIdx).toBeLessThan(midIdx);
      expect(midIdx).toBeLessThan(lowIdx);
    });

    it("caps display to top 15 guesses", () => {
      const guesses = Array.from({ length: 20 }, (_, i) => ({
        word: `word${i}`,
        canonical: `word${i}`,
        similarity: 1 - i * 0.05,
      }));
      const result = renderBoard(guesses);

      expect(result).toContain("5 older guesses hidden");
      expect(result).toContain("word0");
      expect(result).not.toContain("word15");
      expect(result).not.toContain("word19");
    });

    it("marks latest guess with arrow emoji", () => {
      const guesses = [
        { word: "old", canonical: "old", similarity: 0.7 },
        { word: "new", canonical: "new", similarity: 0.3 },
      ];
      const result = renderBoard(guesses, "new");

      const lines = result.split("\n");
      const newLine = lines.find((l) => l.includes("new"));
      expect(newLine).toMatch(/^➡️/);
    });

    it("shows plain marker for non-latest guesses", () => {
      const guesses = [
        { word: "old", canonical: "old", similarity: 0.7 },
        { word: "new", canonical: "new", similarity: 0.3 },
      ];
      const result = renderBoard(guesses, "new");

      // Extract lines from <pre>...</pre> block
      const preMatch = result.match(/<pre>([\s\S]*?)<\/pre>/);
      expect(preMatch).toBeTruthy();
      const preContent = preMatch[1];
      const lines = preContent.split("\n");

      // Old row should start with plain marker (two spaces)
      const oldLine = lines.find((l) => l.includes("old"));
      expect(oldLine).toMatch(/^ {2}/);
    });

    it("escapes HTML special characters in canonical", () => {
      const guesses = [{ word: "<script>", canonical: "<script>", similarity: 0.5 }];
      const result = renderBoard(guesses);

      expect(result).toContain("&lt;script&gt;");
      expect(result).not.toContain("<script>");
    });

    it("includes warmth emoji in each row", () => {
      // calibrate(0.85) ≈ 90 → 🎯, calibrate(0.55) ≈ 29 → 😐
      const guesses = [
        { word: "a", canonical: "a", similarity: 0.85 },
        { word: "b", canonical: "b", similarity: 0.55 },
      ];
      const result = renderBoard(guesses);

      expect(result).toContain("🎯");
      expect(result).toContain("😐");
    });

    it("shows hidden count with correct singular/plural", () => {
      const guesses20 = Array.from({ length: 20 }, (_, i) => ({
        word: `w${i}`,
        canonical: `w${i}`,
        similarity: 1 - i * 0.05,
      }));
      let result = renderBoard(guesses20);
      expect(result).toContain("5 older guesses");

      const guesses16 = Array.from({ length: 16 }, (_, i) => ({
        word: `w${i}`,
        canonical: `w${i}`,
        similarity: 1 - i * 0.05,
      }));
      result = renderBoard(guesses16);
      expect(result).toContain("1 older guess");
    });

    it("returns HTML-formatted pre block", () => {
      const guesses = [{ word: "test", canonical: "test", similarity: 0.5 }];
      const result = renderBoard(guesses);

      expect(result).toContain("<pre>");
      expect(result).toContain("</pre>");
    });

    it("shows no footer when exactly 15 guesses", () => {
      const guesses = Array.from({ length: 15 }, (_, i) => ({
        word: `w${i}`,
        canonical: `w${i}`,
        similarity: 1 - i * 0.07,
      }));
      const result = renderBoard(guesses);

      expect(result).not.toContain("older");
    });
  });

  describe("renderGuess", () => {
    it("renders single-line guess summary", () => {
      const guess = { word: "apple", canonical: "apple", similarity: 0.75 };
      const result = renderGuess(guess);

      expect(result).toContain("apple");
      // calibrate(0.75) ≈ 76
      expect(result).toContain("76");
      expect(result).toContain("🔥");
    });

    it("escapes HTML special characters in canonical", () => {
      const guess = { word: "<tag>", canonical: "<tag>", similarity: 0.5 };
      const result = renderGuess(guess);

      expect(result).toContain("&lt;tag&gt;");
      expect(result).not.toContain("<tag>");
    });

    it("wraps canonical in code tags", () => {
      const guess = { word: "test", canonical: "test", similarity: 0.5 };
      const result = renderGuess(guess);

      expect(result).toMatch(/<code>.*<\/code>/);
    });

    it("includes emoji matching similarity bucket", () => {
      expect(renderGuess({ word: "a", canonical: "a", similarity: 0.85 })).toContain("🎯");
      expect(renderGuess({ word: "b", canonical: "b", similarity: 0.15 })).toContain("🥶");
    });

    it("clips raw cosines below the calibration floor to 00", () => {
      // raw 0.05 and raw -0.2 are both well below FLOOR (0.4) → display "00"
      expect(renderGuess({ word: "a", canonical: "a", similarity: 0.05 })).toContain("00");
      expect(renderGuess({ word: "b", canonical: "b", similarity: -0.2 })).toContain("00");
    });
  });
});
