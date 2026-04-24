import { beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  handleGiveup,
  handleSplash,
  handleStats,
} from "../../../src/modules/loldle-splash/handlers.js";
import splashesData from "../../../src/modules/loldle-splash/splashes.json" with { type: "json" };
import { loadStats } from "../../../src/modules/loldle-splash/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

function pinRandom(value) {
  vi.spyOn(Math, "random").mockReturnValue(value);
}

function makeCtx({ text = "", fromId = 1, chatType = "private", chatId = 1 } = {}) {
  const replies = [];
  const photos = [];
  return {
    replies,
    photos,
    ctx: {
      from: { id: fromId },
      chat: { id: chatId, type: chatType },
      message: { text },
      reply: async (body, opts) => {
        replies.push({ body, opts });
      },
      replyWithPhoto: async (url, opts) => {
        photos.push({ url, opts });
      },
    },
  };
}

describe("loldle-splash handlers — happy path", () => {
  let db;
  beforeEach(() => {
    db = createStore("loldle-splash", { KV: makeFakeKv() });
    pinRandom(0); // picks first champion + first skin
  });

  it("no-arg sends a splash photo with 0/4 caption", async () => {
    const { ctx, photos } = makeCtx();
    await handleSplash(ctx, db);
    expect(photos).toHaveLength(1);
    expect(photos[0].url).toMatch(
      /^https:\/\/ddragon\.leagueoflegends\.com\/cdn\/img\/champion\/splash\//,
    );
    expect(photos[0].opts.caption).toMatch(/0\/4 guesses so far/);
  });

  it("correct guess wins and names the skin", async () => {
    const target = splashesData[0];
    const { ctx, replies } = makeCtx({ text: `/loldle_splash ${target.championName}` });
    await handleSplash(ctx, db);
    expect(replies[0].body).toContain("🎉 Got it!");
    expect(replies[0].body).toContain("skin");
    const s = await loadStats(db, 1);
    expect(s).toMatchObject({ played: 1, wins: 1 });
  });

  it("giveup records loss and names skin", async () => {
    await handleSplash(makeCtx().ctx, db);
    const { ctx, replies } = makeCtx();
    await handleGiveup(ctx, db);
    expect(replies[0].body).toContain("🏳️");
    expect(replies[0].body).toContain("skin");
  });

  it("stats renders zero-state", async () => {
    const { ctx, replies } = makeCtx();
    await handleStats(ctx, db);
    expect(replies[0].body).toContain("Played: 0");
  });
});
