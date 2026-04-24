import { beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import emojisData from "../../../src/modules/loldle-emoji/emojis.json" with { type: "json" };
import {
  handleEmoji,
  handleGiveup,
  handleStats,
} from "../../../src/modules/loldle-emoji/handlers.js";
import { loadStats } from "../../../src/modules/loldle-emoji/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

// Pin randomness so pickRandom() always returns emojisData[0].
function pinRandom(toIndex, poolSize) {
  vi.spyOn(Math, "random").mockReturnValue(toIndex / poolSize);
}

function makeCtx({ text = "", fromId = 1, chatType = "private", chatId = 1 } = {}) {
  const replies = [];
  return {
    replies,
    ctx: {
      from: { id: fromId },
      chat: { id: chatId, type: chatType },
      message: { text },
      reply: async (body, opts) => {
        replies.push({ body, opts });
      },
    },
  };
}

describe("loldle-emoji handlers — happy path", () => {
  let db;
  beforeEach(() => {
    db = createStore("loldle-emoji", { KV: makeFakeKv() });
    pinRandom(0, emojisData.length);
  });

  it("no-arg call shows the clue and an empty board", async () => {
    const { ctx, replies } = makeCtx();
    await handleEmoji(ctx, db);
    expect(replies).toHaveLength(1);
    expect(replies[0].body).toContain("🎭 ");
    expect(replies[0].body).toContain("No guesses yet.");
    expect(replies[0].opts.parse_mode).toBe("HTML");
  });

  it("correct guess increments stats and ends the round", async () => {
    const target = emojisData[0].championName;
    const { ctx, replies } = makeCtx({ text: `/loldle_emoji ${target}` });
    await handleEmoji(ctx, db);
    expect(replies[0].body).toContain("🎉 Got it!");
    expect(replies[0].body).toContain(target);
    const stats = await loadStats(db, 1);
    expect(stats).toMatchObject({ played: 1, wins: 1, streak: 1 });
  });

  it("wrong guess leaves the round open and no stats", async () => {
    const target = emojisData[0].championName;
    const wrong = emojisData[1].championName;
    const { ctx, replies } = makeCtx({ text: `/loldle_emoji ${wrong}` });
    await handleEmoji(ctx, db);
    expect(replies[0].body).toContain("❌");
    expect(replies[0].body).toContain(target === wrong ? target : wrong);
    const stats = await loadStats(db, 1);
    expect(stats.played).toBe(0);
  });

  it("giveup records a loss and clears the round", async () => {
    // seed a round first
    const { ctx: c1 } = makeCtx();
    await handleEmoji(c1, db);
    const { ctx: c2, replies } = makeCtx();
    await handleGiveup(c2, db);
    expect(replies[0].body).toContain("🏳️");
    const stats = await loadStats(db, 1);
    expect(stats).toMatchObject({ played: 1, wins: 0, streak: 0 });
  });

  it("unknown champion replies with not-found", async () => {
    const { ctx, replies } = makeCtx({ text: "/loldle_emoji zzznotreal" });
    await handleEmoji(ctx, db);
    expect(replies[0].body).toContain("Champion not found");
  });

  it("stats includes win rate", async () => {
    const target = emojisData[0].championName;
    await handleEmoji(makeCtx({ text: `/loldle_emoji ${target}` }).ctx, db);
    const { ctx, replies } = makeCtx();
    await handleStats(ctx, db);
    expect(replies[0].body).toContain("Wins: 1 (100%)");
  });
});
