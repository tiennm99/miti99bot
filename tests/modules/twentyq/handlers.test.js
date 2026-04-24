import { beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import { handleGiveup, handleStats, handleTwentyq } from "../../../src/modules/twentyq/handlers.js";
import { loadGame, loadStats, saveGame } from "../../../src/modules/twentyq/state.js";
import { makeFakeAi, mockFailure, mockJudgement, mockRoundStart } from "../../fakes/fake-ai.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

function makeCtx(userId = 1, chatType = "private", msgText = "/twentyq") {
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

const sampleGame = (overrides = {}) => ({
  category: "instrument",
  target: "organ",
  initialHint: "uses wind through pipes",
  startedAt: 1,
  solved: false,
  turns: [],
  ...overrides,
});

describe("twentyq/handlers", () => {
  /** @type {import("../../../src/db/kv-store-interface.js").KVStore} */
  let db;
  let ai;
  let env;

  beforeEach(() => {
    db = createStore("twentyq", { KV: makeFakeKv() });
    ai = makeFakeAi();
    env = { AI: ai };
  });

  describe("handleTwentyq", () => {
    it("starts a fresh round and shows intro when no game + no arg", async () => {
      mockRoundStart(ai, { category: "instrument", initialHint: "cryptic opener" });
      const ctx = makeCtx(1, "private", "/twentyq");
      await handleTwentyq(ctx, { db, env });
      expect(ctx.reply).toHaveBeenCalledOnce();
      expect(ctx.replies[0].text).toMatch(/I'm thinking/);
      const game = await loadGame(db, 1);
      expect(game).not.toBeNull();
      expect(game.turns).toEqual([]);
      expect(game.category).toBe("instrument");
      expect(game.initialHint).toBe("cryptic opener");
    });

    it("shows board when game exists and no arg", async () => {
      await saveGame(db, 1, sampleGame());
      const ctx = makeCtx(1, "private", "/twentyq");
      await handleTwentyq(ctx, { db, env });
      expect(ctx.replies[0].text).toMatch(/Category|Initial hint/);
    });

    it("processes a yes-answer turn", async () => {
      await saveGame(db, 1, sampleGame());
      mockJudgement(ai, { is_guess: false, answer: "yes", hint: "very large" });
      const ctx = makeCtx(1, "private", "/twentyq is it big?");
      await handleTwentyq(ctx, { db, env });
      expect(ai.run).toHaveBeenCalledOnce();
      expect(ctx.replies[0].text).toMatch(/Yes/);
      expect(ctx.replies[0].text).toContain("very large");
      const game = await loadGame(db, 1);
      expect(game.turns).toHaveLength(1);
    });

    it("ends round on correct guess (is_guess && yes)", async () => {
      await saveGame(db, 1, sampleGame());
      mockJudgement(ai, { is_guess: true, answer: "yes", hint: "—" });
      const ctx = makeCtx(1, "private", "/twentyq is it an organ?");
      await handleTwentyq(ctx, { db, env });
      expect(ctx.replies[0].text).toContain("Correct");
      expect(ctx.replies[0].text).toContain("organ");
      // Game cleared
      expect(await loadGame(db, 1)).toBeNull();
      // Stats updated
      const stats = await loadStats(db, 1);
      expect(stats.solved).toBe(1);
      expect(stats.played).toBe(1);
      expect(stats.bestTurnCount).toBe(1);
    });

    it("treats wrong guess (is_guess && no) as a normal hint turn", async () => {
      await saveGame(db, 1, sampleGame());
      mockJudgement(ai, { is_guess: true, answer: "no", hint: "metal pipes" });
      const ctx = makeCtx(1, "private", "/twentyq is it a piano?");
      await handleTwentyq(ctx, { db, env });
      expect(ctx.replies[0].text).toMatch(/Not quite|No/);
      expect(ctx.replies[0].text).toContain("metal pipes");
      expect(await loadGame(db, 1)).not.toBeNull();
    });

    it("rejects open-ended question without hitting AI", async () => {
      await saveGame(db, 1, sampleGame());
      const ctx = makeCtx(1, "private", "/twentyq what is it?");
      await handleTwentyq(ctx, { db, env });
      expect(ai.run).not.toHaveBeenCalled();
      expect(ctx.replies[0].text).toMatch(/yes\/no/i);
      const game = await loadGame(db, 1);
      expect(game.turns).toHaveLength(0);
    });

    it("dedups exact repeat questions without hitting AI", async () => {
      await saveGame(
        db,
        1,
        sampleGame({
          turns: [{ text: "is it big?", isGuess: false, answer: "yes", hint: "x", ts: 1 }],
        }),
      );
      const ctx = makeCtx(1, "private", "/twentyq Is It Big?");
      await handleTwentyq(ctx, { db, env });
      expect(ai.run).not.toHaveBeenCalled();
      expect(ctx.replies[0].text).toMatch(/already asked/i);
    });

    it("starts a fresh round transparently after a solved game", async () => {
      await saveGame(db, 1, sampleGame({ solved: true }));
      mockRoundStart(ai);
      const ctx = makeCtx(1, "private", "/twentyq");
      await handleTwentyq(ctx, { db, env });
      expect(ctx.replies[0].text).toMatch(/I'm thinking/);
    });

    it("starts round and processes turn when /twentyq <text> with no prior game", async () => {
      mockRoundStart(ai);
      mockJudgement(ai, { is_guess: false, answer: "yes", hint: "yes hint" });
      const ctx = makeCtx(1, "private", "/twentyq is it big?");
      await handleTwentyq(ctx, { db, env });
      expect(ai.run).toHaveBeenCalledTimes(2); // roundstart + judge
      expect(ctx.reply).toHaveBeenCalledTimes(2); // intro + turn
      expect(ctx.replies[0].text).toMatch(/I'm thinking/);
      expect(ctx.replies[1].text).toMatch(/Yes/);
    });

    it("surfaces roundstart UpstreamError as friendly message", async () => {
      mockFailure(ai, new Error("roundstart down"));
      const ctx = makeCtx(1, "private", "/twentyq");
      await handleTwentyq(ctx, { db, env });
      expect(ctx.replies[0].text).toMatch(/hiccup|try again/i);
      expect(await loadGame(db, 1)).toBeNull();
    });

    it("surfaces UpstreamError as friendly message", async () => {
      await saveGame(db, 1, sampleGame());
      mockFailure(ai, new Error("boom"));
      const ctx = makeCtx(1, "private", "/twentyq is it big?");
      await handleTwentyq(ctx, { db, env });
      expect(ctx.replies[0].text).toMatch(/hiccup|try again/i);
    });

    it("uses chat id as subject in groups", async () => {
      mockRoundStart(ai);
      mockJudgement(ai, { is_guess: false, answer: "no", hint: "h" });
      const ctx = makeCtx(99, "group", "/twentyq is it big?");
      ctx.chat.id = 12345;
      await handleTwentyq(ctx, { db, env });
      expect(ai.run).toHaveBeenCalledTimes(2); // roundstart + judge
      // Game saved under chat id (12345), not user id (99)
      expect(await loadGame(db, 12345)).not.toBeNull();
      expect(await loadGame(db, 99)).toBeNull();
    });
  });

  describe("handleGiveup", () => {
    it("replies 'no active round' when none", async () => {
      const ctx = makeCtx(1);
      await handleGiveup(ctx, { db });
      expect(ctx.replies[0].text).toMatch(/no active round/i);
    });

    it("reveals target + records loss + clears game", async () => {
      await saveGame(db, 1, sampleGame());
      const ctx = makeCtx(1);
      await handleGiveup(ctx, { db });
      expect(ctx.replies[0].text).toContain("organ");
      expect(await loadGame(db, 1)).toBeNull();
      const stats = await loadStats(db, 1);
      expect(stats.played).toBe(1);
      expect(stats.solved).toBe(0);
    });
  });

  describe("handleStats", () => {
    it("renders empty-stats message when no rounds finished", async () => {
      const ctx = makeCtx(1);
      await handleStats(ctx, { db });
      expect(ctx.replies[0].text).toMatch(/no.*games/i);
    });
  });
});
