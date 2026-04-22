import { beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import { Word2SimError } from "../../../src/modules/semantle/api-client.js";
import {
  handleGiveup,
  handleNew,
  handleSemantle,
  handleStats,
} from "../../../src/modules/semantle/handlers.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

function makeCtx(userId = 1, chatType = "private", msgText = "/semantle") {
  const replies = [];
  return {
    chat: { id: userId, type: chatType },
    from: { id: userId },
    message: { text: msgText },
    reply: vi.fn((text, opts) => {
      replies.push({ text, opts });
      return Promise.resolve();
    }),
    replies,
  };
}

function makeClient() {
  return {
    randomWord: vi.fn(),
    similarity: vi.fn(),
  };
}

describe("semantle/handlers", () => {
  let db;
  let client;

  beforeEach(() => {
    db = createStore("semantle", { KV: makeFakeKv() });
    client = makeClient();
  });

  describe("handleSemantle", () => {
    it("starts a new round when no args", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });

      const ctx = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx, { db, client });

      expect(client.randomWord).toHaveBeenCalledOnce();
      expect(ctx.reply).toHaveBeenCalledOnce();
      expect(ctx.replies[0].text).toContain("Round ready");
    });

    it("shows board with 0 guesses after fresh start", async () => {
      client.randomWord.mockResolvedValue({ word: "target", rank: 500 });

      const ctx = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx, { db, client });

      expect(ctx.replies[0].text).toContain("0 guesses");
      expect(ctx.replies[0].opts.parse_mode).toBe("HTML");
    });

    it("reuses existing unsolved game", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });

      const ctx1 = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx1, { db, client });
      expect(client.randomWord).toHaveBeenCalledTimes(1);

      const ctx2 = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx2, { db, client });
      expect(client.randomWord).toHaveBeenCalledTimes(1);
    });

    it("starts fresh game after solving", async () => {
      // First game
      client.randomWord.mockResolvedValueOnce({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValueOnce({
        a: "apple",
        b: "apple",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "apple",
        similarity: 1.0,
      });

      let ctx = makeCtx(1, "private", "/semantle apple");
      await handleSemantle(ctx, { db, client });
      expect(ctx.replies[0].text).toContain("Solved in 1 guess");

      // Second game
      client.randomWord.mockResolvedValueOnce({ word: "orange", rank: 1000 });
      ctx = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx, { db, client });

      expect(client.randomWord).toHaveBeenCalledTimes(2);
      expect(ctx.replies[0].text).toContain("0 guesses");
    });

    it("submits guess and appends to board", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "orange",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "orange",
        similarity: 0.45,
      });

      const ctx = makeCtx(1, "private", "/semantle orange");
      await handleSemantle(ctx, { db, client });

      expect(ctx.reply).toHaveBeenCalledOnce();
      expect(ctx.replies[0].text).toContain("orange");
      expect(ctx.replies[0].text).toContain("+45");
    });

    it("solves when guess equals target (case-insensitive)", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "APPLE",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "apple",
        similarity: 1.0,
      });

      const ctx = makeCtx(1, "private", "/semantle APPLE");
      await handleSemantle(ctx, { db, client });

      expect(ctx.replies[0].text).toContain("✅ Solved in 1 guess");
    });

    it("clears game after solve and records result", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "apple",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "apple",
        similarity: 1.0,
      });

      const ctx = makeCtx(1, "private", "/semantle apple");
      await handleSemantle(ctx, { db, client });

      // Verify game is cleared
      const { loadGame, loadStats } = await import("../../../src/modules/semantle/state.js");
      const game = await loadGame(db, 1);
      expect(game).toBeNull();

      // Verify stats recorded
      const stats = await loadStats(db, 1);
      expect(stats.played).toBe(1);
      expect(stats.solved).toBe(1);
    });

    it("rejects invalid shape guess", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });

      const ctx = makeCtx(1, "private", "/semantle 123abc");
      await handleSemantle(ctx, { db, client });

      expect(ctx.replies[0].text).toContain("letter-only");
      expect(client.similarity).not.toHaveBeenCalled();
    });

    it("rejects OOV guess and does not save to board", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "xyz",
        in_vocab_a: true,
        in_vocab_b: false,
        similarity: null,
      });

      const ctx = makeCtx(1, "private", "/semantle xyz");
      await handleSemantle(ctx, { db, client });

      expect(ctx.replies[0].text).toContain("vocabulary");
      expect(ctx.replies[0].text).toContain("xyz");

      // Verify guess was not saved
      const { loadGame } = await import("../../../src/modules/semantle/state.js");
      const game = await loadGame(db, 1);
      expect(game.guesses.length).toBe(0);
    });

    it("deduplicates re-submitted words", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "orange",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "orange",
        similarity: 0.45,
      });

      const ctx1 = makeCtx(1, "private", "/semantle orange");
      await handleSemantle(ctx1, { db, client });

      const ctx2 = makeCtx(1, "private", "/semantle orange");
      await handleSemantle(ctx2, { db, client });

      const { loadGame } = await import("../../../src/modules/semantle/state.js");
      const game = await loadGame(db, 1);
      expect(game.guesses.length).toBe(1);
      expect(ctx2.replies[0].text).toContain("1 guess");
    });

    it("sets startedAt on first guess", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "orange",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "orange",
        similarity: 0.45,
      });

      const before = Date.now();
      const ctx = makeCtx(1, "private", "/semantle orange");
      await handleSemantle(ctx, { db, client });
      const after = Date.now();

      const { loadGame } = await import("../../../src/modules/semantle/state.js");
      const game = await loadGame(db, 1);
      expect(game.startedAt).toBeGreaterThanOrEqual(before);
      expect(game.startedAt).toBeLessThanOrEqual(after);
    });

    it("includes latest guess marker in render", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity
        .mockResolvedValueOnce({
          a: "apple",
          b: "orange",
          in_vocab_a: true,
          in_vocab_b: true,
          canonical_b: "orange",
          similarity: 0.45,
        })
        .mockResolvedValueOnce({
          a: "apple",
          b: "banana",
          in_vocab_a: true,
          in_vocab_b: true,
          canonical_b: "banana",
          similarity: 0.35,
        });

      const ctx1 = makeCtx(1, "private", "/semantle orange");
      await handleSemantle(ctx1, { db, client });

      const ctx2 = makeCtx(1, "private", "/semantle banana");
      await handleSemantle(ctx2, { db, client });

      expect(ctx2.replies[0].text).toContain("➡️");
    });

    it("replies with UPSTREAM_FAIL on randomWord error", async () => {
      client.randomWord.mockRejectedValue(new Word2SimError("timeout", { status: 504 }));

      const ctx = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx, { db, client });

      expect(ctx.replies[0].text).toContain("⚠️ Upstream hiccup");
    });

    it("replies with UPSTREAM_FAIL on similarity error", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockRejectedValue(new Word2SimError("network error"));

      const ctx = makeCtx(1, "private", "/semantle guess");
      await handleSemantle(ctx, { db, client });

      expect(ctx.replies[0].text).toContain("⚠️ Upstream hiccup");
    });

    it("handles group chat (shared game)", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });

      const ctx = makeCtx(-123456, "group", "/semantle");
      await handleSemantle(ctx, { db, client });

      expect(ctx.reply).toHaveBeenCalledOnce();
      expect(ctx.replies[0].text).toContain("Round ready");
    });

    it("rejects when cannot identify subject", async () => {
      const ctx = makeCtx();
      ctx.chat = null;
      ctx.from = null;

      await handleSemantle(ctx, { db, client });

      expect(ctx.replies[0].text).toContain("Cannot identify chat");
    });

    it("normalizes guess to lowercase", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "ORANGE",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "orange",
        similarity: 0.45,
      });

      const ctx = makeCtx(1, "private", "/semantle   ORANGE   ");
      await handleSemantle(ctx, { db, client });

      expect(client.similarity).toHaveBeenCalledWith("apple", "orange");
    });
  });

  describe("handleNew", () => {
    it("starts fresh game with no prior game", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });

      const ctx = makeCtx(1, "private", "/semantle_new");
      await handleNew(ctx, { db, client });

      expect(ctx.reply).toHaveBeenCalledOnce();
      expect(ctx.replies[0].text).toContain("🆕 New round started");
    });

    it("abandons unsolved game and records non-solve", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "orange",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "orange",
        similarity: 0.45,
      });

      const ctx1 = makeCtx(1, "private", "/semantle orange");
      await handleSemantle(ctx1, { db, client });

      client.randomWord.mockResolvedValueOnce({ word: "banana", rank: 1000 });
      const ctx2 = makeCtx(1, "private", "/semantle_new");
      await handleNew(ctx2, { db, client });

      const { loadStats } = await import("../../../src/modules/semantle/state.js");
      const stats = await loadStats(db, 1);
      expect(stats.played).toBe(1);
      expect(stats.solved).toBe(0);
      expect(stats.totalGuesses).toBe(1);
    });

    it("does not record result if game had zero guesses", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });

      const ctx1 = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx1, { db, client });

      client.randomWord.mockResolvedValueOnce({ word: "banana", rank: 1000 });
      const ctx2 = makeCtx(1, "private", "/semantle_new");
      await handleNew(ctx2, { db, client });

      const { loadStats } = await import("../../../src/modules/semantle/state.js");
      const stats = await loadStats(db, 1);
      expect(stats.played).toBe(0);
    });

    it("replies UPSTREAM_FAIL on randomWord error", async () => {
      client.randomWord.mockRejectedValue(new Word2SimError("timeout"));

      const ctx = makeCtx(1, "private", "/semantle_new");
      await handleNew(ctx, { db, client });

      expect(ctx.replies[0].text).toContain("⚠️ Upstream hiccup");
    });

    it("handles group chat", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });

      const ctx = makeCtx(-123456, "group", "/semantle_new");
      await handleNew(ctx, { db, client });

      expect(ctx.reply).toHaveBeenCalledOnce();
    });
  });

  describe("handleGiveup", () => {
    it("reveals target and clears game", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });

      const ctx1 = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx1, { db, client });

      const ctx2 = makeCtx(1, "private", "/semantle_giveup");
      await handleGiveup(ctx2, { db });

      expect(ctx2.replies[0].text).toContain("<b>apple</b>");
      expect(ctx2.replies[0].text).toContain("🏳️");

      const { loadGame } = await import("../../../src/modules/semantle/state.js");
      const game = await loadGame(db, 1);
      expect(game).toBeNull();
    });

    it("records non-solve result when giveup", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "orange",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "orange",
        similarity: 0.45,
      });

      const ctx1 = makeCtx(1, "private", "/semantle orange");
      await handleSemantle(ctx1, { db, client });

      const ctx2 = makeCtx(1, "private", "/semantle_giveup");
      await handleGiveup(ctx2, { db });

      const { loadStats } = await import("../../../src/modules/semantle/state.js");
      const stats = await loadStats(db, 1);
      expect(stats.played).toBe(1);
      expect(stats.solved).toBe(0);
      expect(stats.totalGuesses).toBe(1);
    });

    it("replies 'no active round' when no game", async () => {
      const ctx = makeCtx(1, "private", "/semantle_giveup");
      await handleGiveup(ctx, { db });

      expect(ctx.replies[0].text).toContain("No active round");
    });

    it("escapes HTML in target reveal", async () => {
      client.randomWord.mockResolvedValue({ word: "<xss>", rank: 1000 });

      const ctx1 = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx1, { db, client });

      const ctx2 = makeCtx(1, "private", "/semantle_giveup");
      await handleGiveup(ctx2, { db });

      expect(ctx2.replies[0].text).toContain("&lt;xss&gt;");
    });
  });

  describe("handleStats", () => {
    it("shows default message for new user", async () => {
      const ctx = makeCtx(1, "private", "/semantle_stats");
      await handleStats(ctx, { db });

      expect(ctx.replies[0].text).toContain("No semantle games played yet");
    });

    it("shows stats after games", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity
        .mockResolvedValueOnce({
          a: "apple",
          b: "apple",
          in_vocab_a: true,
          in_vocab_b: true,
          canonical_b: "apple",
          similarity: 1.0,
        })
        .mockResolvedValueOnce({
          a: "banana",
          b: "orange",
          in_vocab_a: true,
          in_vocab_b: true,
          canonical_b: "orange",
          similarity: 0.45,
        })
        .mockResolvedValueOnce({
          a: "banana",
          b: "grape",
          in_vocab_a: true,
          in_vocab_b: true,
          canonical_b: "grape",
          similarity: 0.35,
        });

      // Game 1: solve in 1 guess
      const ctx1 = makeCtx(1, "private", "/semantle apple");
      await handleSemantle(ctx1, { db, client });

      // Game 2: lose after 2 guesses
      client.randomWord.mockResolvedValueOnce({ word: "banana", rank: 1000 });
      const ctx2a = makeCtx(1, "private", "/semantle");
      await handleSemantle(ctx2a, { db, client });

      const ctx2b = makeCtx(1, "private", "/semantle orange");
      await handleSemantle(ctx2b, { db, client });

      const ctx2c = makeCtx(1, "private", "/semantle grape");
      await handleSemantle(ctx2c, { db, client });

      const ctx2d = makeCtx(1, "private", "/semantle_giveup");
      await handleGiveup(ctx2d, { db });

      // Check stats
      const ctx3 = makeCtx(1, "private", "/semantle_stats");
      await handleStats(ctx3, { db });

      const statsText = ctx3.replies[0].text;
      expect(statsText).toContain("Played: 2");
      expect(statsText).toContain("Solved: 1 (50%)");
      expect(statsText).toContain("Total guesses: 3");
      expect(statsText).toContain("Fewest to solve: 1");
      expect(statsText).toContain("Avg per round: 2");
    });

    it("shows '—' for bestGuessCount when no solves", async () => {
      client.randomWord.mockResolvedValue({ word: "apple", rank: 1000 });
      client.similarity.mockResolvedValue({
        a: "apple",
        b: "orange",
        in_vocab_a: true,
        in_vocab_b: true,
        canonical_b: "orange",
        similarity: 0.45,
      });

      const ctx1 = makeCtx(1, "private", "/semantle orange");
      await handleSemantle(ctx1, { db, client });

      const ctx2 = makeCtx(1, "private", "/semantle_giveup");
      await handleGiveup(ctx2, { db });

      const ctx3 = makeCtx(1, "private", "/semantle_stats");
      await handleStats(ctx3, { db });

      expect(ctx3.replies[0].text).toContain("Fewest to solve: —");
    });

    it("calculates solve percentage correctly", async () => {
      const { recordResult } = await import("../../../src/modules/semantle/state.js");
      await recordResult(db, 1, { solved: true, guessCount: 2 });
      await recordResult(db, 1, { solved: true, guessCount: 3 });
      await recordResult(db, 1, { solved: false, guessCount: 4 });
      await recordResult(db, 1, { solved: false, guessCount: 5 });

      const ctx = makeCtx(1, "private", "/semantle_stats");
      await handleStats(ctx, { db });

      expect(ctx.replies[0].text).toContain("Solved: 2 (50%)");
    });

    it("formats average guesses per round", async () => {
      const { recordResult } = await import("../../../src/modules/semantle/state.js");
      await recordResult(db, 1, { solved: true, guessCount: 3 });
      await recordResult(db, 1, { solved: false, guessCount: 5 });

      const ctx = makeCtx(1, "private", "/semantle_stats");
      await handleStats(ctx, { db });

      expect(ctx.replies[0].text).toContain("Avg per round: 4");
    });

    it("includes HTML formatting", async () => {
      const { recordResult } = await import("../../../src/modules/semantle/state.js");
      await recordResult(db, 1, { solved: true, guessCount: 1 });

      const ctx = makeCtx(1, "private", "/semantle_stats");
      await handleStats(ctx, { db });

      expect(ctx.replies[0].opts.parse_mode).toBe("HTML");
      expect(ctx.replies[0].text).toContain("<b>");
    });
  });
});
