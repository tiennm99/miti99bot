import { describe, expect, it } from "vitest";
import {
  formatBoard,
  formatGiveup,
  formatIntro,
  formatStats,
  formatTurnReply,
} from "../../../src/modules/twentyq/render.js";

const baseState = (overrides = {}) => ({
  category: "instrument",
  target: "organ",
  initialHint: "uses wind through pipes",
  startedAt: 1,
  solved: false,
  turns: [],
  ...overrides,
});

describe("twentyq/render", () => {
  describe("formatIntro", () => {
    it("includes category and initial hint", () => {
      const out = formatIntro(baseState());
      expect(out).toContain("instrument");
      expect(out).toContain("uses wind through pipes");
    });

    it("HTML-escapes user-derived strings", () => {
      const out = formatIntro(baseState({ category: "<script>", initialHint: "<i>" }));
      expect(out).toContain("&lt;script&gt;");
      expect(out).toContain("&lt;i&gt;");
    });
  });

  describe("formatTurnReply", () => {
    it("renders a winning solve message", () => {
      const turn = { text: "is it an organ?", isGuess: true, answer: "yes", hint: "x", ts: 1 };
      const out = formatTurnReply({ turn, solved: true, target: "organ", turnCount: 4 });
      expect(out).toContain("Correct");
      expect(out).toContain("organ");
      expect(out).toContain("4");
    });

    it("renders a wrong-guess miss with hint", () => {
      const turn = {
        text: "is it a piano?",
        isGuess: true,
        answer: "no",
        hint: "metal pipes",
        ts: 1,
      };
      const out = formatTurnReply({ turn, solved: false, target: "organ", turnCount: 2 });
      expect(out).toContain("Not quite");
      expect(out).toContain("metal pipes");
    });

    it("renders a regular yes turn", () => {
      const turn = { text: "is it big?", isGuess: false, answer: "yes", hint: "very large", ts: 1 };
      const out = formatTurnReply({ turn, solved: false, target: "organ", turnCount: 1 });
      expect(out).toContain("Yes");
      expect(out).toContain("very large");
    });

    it("escapes hint content", () => {
      const turn = { text: "is it big?", isGuess: false, answer: "no", hint: "<bad>", ts: 1 };
      const out = formatTurnReply({ turn, solved: false, target: "organ", turnCount: 1 });
      expect(out).toContain("&lt;bad&gt;");
    });
  });

  describe("formatBoard", () => {
    it("shows 'no questions' message when turns empty", () => {
      const out = formatBoard(baseState());
      expect(out).toMatch(/no questions/i);
    });

    it("renders numbered Q/A pairs", () => {
      const turns = [
        { text: "is it big?", isGuess: false, answer: "yes", hint: "very", ts: 1 },
        { text: "is it loud?", isGuess: false, answer: "yes", hint: "loud", ts: 2 },
      ];
      const out = formatBoard(baseState({ turns }));
      expect(out).toContain("is it big?");
      expect(out).toContain("is it loud?");
    });
  });

  describe("formatGiveup", () => {
    it("reveals target with HTML escape", () => {
      const out = formatGiveup(baseState({ target: "<x>" }));
      expect(out).toContain("&lt;x&gt;");
      expect(out).toContain("Gave up");
    });
  });

  describe("formatStats", () => {
    it("returns 'no games' message when played=0", () => {
      const out = formatStats({
        played: 0,
        solved: 0,
        totalTurns: 0,
        bestTurnCount: null,
        lastResultAt: null,
      });
      expect(out).toMatch(/no.*games/i);
    });

    it("computes solve rate and avg", () => {
      const out = formatStats({
        played: 4,
        solved: 2,
        totalTurns: 20,
        bestTurnCount: 3,
        lastResultAt: 1,
      });
      expect(out).toContain("50%");
      expect(out).toContain("3");
      expect(out).toContain("5"); // avg = 20/4 = 5
    });
  });
});
