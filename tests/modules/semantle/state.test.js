import { beforeEach, describe, expect, it } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  clearGame,
  loadGame,
  loadStats,
  recordResult,
  saveGame,
} from "../../../src/modules/semantle/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

describe("semantle/state", () => {
  let db;

  beforeEach(() => {
    db = createStore("semantle", { KV: makeFakeKv() });
  });

  describe("saveGame / loadGame", () => {
    it("round-trips game state", async () => {
      const subject = 12345;
      const game = {
        target: "apple",
        startedAt: Date.now(),
        solved: false,
        guesses: [
          { word: "orange", canonical: "orange", similarity: 0.45 },
          { word: "banana", canonical: "banana", similarity: 0.32 },
        ],
      };

      await saveGame(db, subject, game);
      const loaded = await loadGame(db, subject);

      expect(loaded).toEqual(game);
      expect(loaded.guesses.length).toBe(2);
      expect(loaded.target).toBe("apple");
    });

    it("returns null for non-existent game", async () => {
      const loaded = await loadGame(db, 999);
      expect(loaded).toBeNull();
    });

    it("overwrites previous game on second save", async () => {
      const subject = 1;
      await saveGame(db, subject, { target: "a", startedAt: null, solved: false, guesses: [] });
      await saveGame(db, subject, { target: "b", startedAt: 100, solved: true, guesses: [] });

      const loaded = await loadGame(db, subject);
      expect(loaded.target).toBe("b");
      expect(loaded.solved).toBe(true);
    });

    it("preserves null startedAt", async () => {
      const subject = 1;
      const game = {
        target: "test",
        startedAt: null,
        solved: false,
        guesses: [],
      };
      await saveGame(db, subject, game);
      const loaded = await loadGame(db, subject);
      expect(loaded.startedAt).toBeNull();
    });
  });

  describe("clearGame", () => {
    it("removes game entry", async () => {
      const subject = 1;
      await saveGame(db, subject, {
        target: "x",
        startedAt: null,
        solved: false,
        guesses: [],
      });
      await clearGame(db, subject);
      const loaded = await loadGame(db, subject);
      expect(loaded).toBeNull();
    });

    it("is idempotent", async () => {
      const subject = 1;
      await clearGame(db, subject);
      await clearGame(db, subject);
      expect(await loadGame(db, subject)).toBeNull();
    });
  });

  describe("loadStats", () => {
    it("returns defaults when no stats exist", async () => {
      const stats = await loadStats(db, 999);
      expect(stats).toEqual({
        played: 0,
        solved: 0,
        totalGuesses: 0,
        bestGuessCount: null,
        lastResultAt: null,
      });
    });

    it("loads existing stats", async () => {
      const subject = 1;
      await db.putJSON("stats:1", {
        played: 5,
        solved: 3,
        totalGuesses: 12,
        bestGuessCount: 2,
        lastResultAt: 123456,
      });
      const stats = await loadStats(db, subject);
      expect(stats.played).toBe(5);
      expect(stats.solved).toBe(3);
    });
  });

  describe("recordResult", () => {
    it("increments played on every result", async () => {
      const subject = 1;
      await recordResult(db, subject, { solved: false, guessCount: 3 });
      let stats = await loadStats(db, subject);
      expect(stats.played).toBe(1);

      await recordResult(db, subject, { solved: false, guessCount: 2 });
      stats = await loadStats(db, subject);
      expect(stats.played).toBe(2);
    });

    it("increments totalGuesses with each result", async () => {
      const subject = 1;
      await recordResult(db, subject, { solved: false, guessCount: 3 });
      await recordResult(db, subject, { solved: true, guessCount: 5 });

      const stats = await loadStats(db, subject);
      expect(stats.totalGuesses).toBe(8);
    });

    it("increments solved only when solved=true", async () => {
      const subject = 1;
      await recordResult(db, subject, { solved: false, guessCount: 2 });
      await recordResult(db, subject, { solved: true, guessCount: 3 });
      await recordResult(db, subject, { solved: false, guessCount: 1 });

      const stats = await loadStats(db, subject);
      expect(stats.solved).toBe(1);
      expect(stats.played).toBe(3);
    });

    it("sets bestGuessCount to first solved result count", async () => {
      const subject = 1;
      await recordResult(db, subject, { solved: true, guessCount: 7 });

      const stats = await loadStats(db, subject);
      expect(stats.bestGuessCount).toBe(7);
    });

    it("updates bestGuessCount only if new count is lower", async () => {
      const subject = 1;
      await recordResult(db, subject, { solved: true, guessCount: 5 });
      await recordResult(db, subject, { solved: true, guessCount: 3 });
      await recordResult(db, subject, { solved: true, guessCount: 4 });

      const stats = await loadStats(db, subject);
      expect(stats.bestGuessCount).toBe(3);
    });

    it("does not update bestGuessCount when not solved", async () => {
      const subject = 1;
      await recordResult(db, subject, { solved: true, guessCount: 5 });
      await recordResult(db, subject, { solved: false, guessCount: 2 });

      const stats = await loadStats(db, subject);
      expect(stats.bestGuessCount).toBe(5);
    });

    it("keeps bestGuessCount as null if only losses", async () => {
      const subject = 1;
      await recordResult(db, subject, { solved: false, guessCount: 10 });
      await recordResult(db, subject, { solved: false, guessCount: 5 });

      const stats = await loadStats(db, subject);
      expect(stats.bestGuessCount).toBeNull();
    });

    it("records lastResultAt timestamp", async () => {
      const subject = 1;
      const before = Date.now();
      await recordResult(db, subject, { solved: true, guessCount: 1 });
      const after = Date.now();

      const stats = await loadStats(db, subject);
      expect(stats.lastResultAt).toBeGreaterThanOrEqual(before);
      expect(stats.lastResultAt).toBeLessThanOrEqual(after);
    });

    it("returns updated stats", async () => {
      const subject = 1;
      const returned = await recordResult(db, subject, { solved: true, guessCount: 2 });

      expect(returned.played).toBe(1);
      expect(returned.solved).toBe(1);
      expect(returned.bestGuessCount).toBe(2);
    });
  });
});
