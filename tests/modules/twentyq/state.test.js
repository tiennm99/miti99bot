import { beforeEach, describe, expect, it } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  clearGame,
  loadGame,
  loadStats,
  recordResult,
  saveGame,
} from "../../../src/modules/twentyq/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

const sampleGame = () => ({
  category: "instrument",
  target: "organ",
  initialHint: "it uses wind through pipes",
  startedAt: Date.now(),
  solved: false,
  turns: [
    { text: "is it big?", isGuess: false, answer: "yes", hint: "very big", ts: 1 },
    { text: "is it loud?", isGuess: false, answer: "yes", hint: "very loud", ts: 2 },
  ],
});

describe("twentyq/state", () => {
  /** @type {import("../../../src/db/kv-store-interface.js").KVStore} */
  let db;

  beforeEach(() => {
    db = createStore("twentyq", { KV: makeFakeKv() });
  });

  describe("saveGame / loadGame / clearGame", () => {
    it("round-trips game state", async () => {
      const game = sampleGame();
      await saveGame(db, 42, game);
      const loaded = await loadGame(db, 42);
      expect(loaded).toEqual(game);
    });

    it("returns null for missing subject", async () => {
      expect(await loadGame(db, 999)).toBeNull();
    });

    it("clearGame removes only that subject's game", async () => {
      await saveGame(db, 1, sampleGame());
      await saveGame(db, 2, sampleGame());
      await clearGame(db, 1);
      expect(await loadGame(db, 1)).toBeNull();
      expect(await loadGame(db, 2)).not.toBeNull();
    });

    it("overwrites prior game on second save", async () => {
      await saveGame(db, 1, { ...sampleGame(), target: "guitar" });
      await saveGame(db, 1, { ...sampleGame(), target: "drum" });
      const loaded = await loadGame(db, 1);
      expect(loaded.target).toBe("drum");
    });
  });

  describe("loadStats", () => {
    it("returns zeroed defaults when missing", async () => {
      const s = await loadStats(db, 999);
      expect(s).toEqual({
        played: 0,
        solved: 0,
        totalTurns: 0,
        bestTurnCount: null,
        lastResultAt: null,
      });
    });
  });

  describe("recordResult", () => {
    it("loss only increments played + totalTurns", async () => {
      await recordResult(db, 1, { solved: false, turnCount: 4 });
      const s = await loadStats(db, 1);
      expect(s.played).toBe(1);
      expect(s.solved).toBe(0);
      expect(s.totalTurns).toBe(4);
      expect(s.bestTurnCount).toBeNull();
    });

    it("solve sets bestTurnCount on first win", async () => {
      await recordResult(db, 1, { solved: true, turnCount: 6 });
      const s = await loadStats(db, 1);
      expect(s.solved).toBe(1);
      expect(s.bestTurnCount).toBe(6);
    });

    it("only lowers bestTurnCount on a faster solve", async () => {
      await recordResult(db, 1, { solved: true, turnCount: 8 });
      await recordResult(db, 1, { solved: true, turnCount: 5 });
      await recordResult(db, 1, { solved: true, turnCount: 9 });
      const s = await loadStats(db, 1);
      expect(s.bestTurnCount).toBe(5);
    });

    it("loss after a solve does not affect bestTurnCount", async () => {
      await recordResult(db, 1, { solved: true, turnCount: 3 });
      await recordResult(db, 1, { solved: false, turnCount: 10 });
      const s = await loadStats(db, 1);
      expect(s.bestTurnCount).toBe(3);
    });

    it("records lastResultAt", async () => {
      const before = Date.now();
      await recordResult(db, 1, { solved: true, turnCount: 1 });
      const after = Date.now();
      const s = await loadStats(db, 1);
      expect(s.lastResultAt).toBeGreaterThanOrEqual(before);
      expect(s.lastResultAt).toBeLessThanOrEqual(after);
    });
  });
});
