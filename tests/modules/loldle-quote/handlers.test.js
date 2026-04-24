import { beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  handleGiveup,
  handleQuote,
  handleStats,
} from "../../../src/modules/loldle-quote/handlers.js";
import quotesData from "../../../src/modules/loldle-quote/quotes.json" with { type: "json" };
import { loadStats } from "../../../src/modules/loldle-quote/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

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

describe("loldle-quote handlers — happy path", () => {
  let db;
  beforeEach(() => {
    db = createStore("loldle-quote", { KV: makeFakeKv() });
    pinRandom(0, quotesData.length);
  });

  it("no-arg shows italic quote block", async () => {
    const { ctx, replies } = makeCtx();
    await handleQuote(ctx, db);
    expect(replies[0].body).toContain("🎭 <i>");
    expect(replies[0].body).toContain("No guesses yet.");
  });

  it("correct guess wins, stats update", async () => {
    const target = quotesData[0].championName;
    const { ctx, replies } = makeCtx({ text: `/loldle_quote ${target}` });
    await handleQuote(ctx, db);
    expect(replies[0].body).toContain("🎉 Nailed it!");
    const s = await loadStats(db, 1);
    expect(s).toMatchObject({ played: 1, wins: 1, streak: 1 });
  });

  it("giveup records loss", async () => {
    await handleQuote(makeCtx().ctx, db);
    const { ctx, replies } = makeCtx();
    await handleGiveup(ctx, db);
    expect(replies[0].body).toContain("🏳️");
    const s = await loadStats(db, 1);
    expect(s).toMatchObject({ played: 1, wins: 0 });
  });

  it("stats shows zero-state nicely", async () => {
    const { ctx, replies } = makeCtx();
    await handleStats(ctx, db);
    expect(replies[0].body).toContain("Played: 0");
    expect(replies[0].body).toContain("Wins: 0 (0%)");
  });
});
