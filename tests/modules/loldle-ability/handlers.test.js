import { beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import abilitiesData from "../../../src/modules/loldle-ability/abilities.json" with {
  type: "json",
};
import {
  handleAbility,
  handleGiveup,
  handleStats,
} from "../../../src/modules/loldle-ability/handlers.js";
import { loadStats } from "../../../src/modules/loldle-ability/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

function pinRandom(value) {
  // deterministic: force Math.random() to the floor of the first entry
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

describe("loldle-ability handlers — happy path", () => {
  let db;
  beforeEach(() => {
    db = createStore("loldle-ability", { KV: makeFakeKv() });
    pinRandom(0); // picks abilitiesData[0] + abilities[0]
  });

  it("no-arg sends a photo with DDragon icon URL and caption", async () => {
    const { ctx, photos } = makeCtx();
    await handleAbility(ctx, db);
    expect(photos).toHaveLength(1);
    expect(photos[0].url).toMatch(/^https:\/\/ddragon\.leagueoflegends\.com\//);
    expect(photos[0].opts.caption).toMatch(/0\/5 guesses so far/);
  });

  it("correct guess wins and cites slot + ability name", async () => {
    const target = abilitiesData[0];
    const { ctx, replies } = makeCtx({ text: `/loldle_ability ${target.championName}` });
    await handleAbility(ctx, db);
    expect(replies[0].body).toContain("🎉 Got it!");
    expect(replies[0].body).toContain(target.championName);
    expect(replies[0].body).toMatch(/\([PQWER]\)/);
    const s = await loadStats(db, 1);
    expect(s).toMatchObject({ played: 1, wins: 1, streak: 1 });
  });

  it("wrong guess does not end the round", async () => {
    const wrong = abilitiesData[1];
    const { ctx, replies } = makeCtx({ text: `/loldle_ability ${wrong.championName}` });
    await handleAbility(ctx, db);
    expect(replies[0].body).toContain("❌");
    const s = await loadStats(db, 1);
    expect(s.played).toBe(0);
  });

  it("giveup records loss and names the ability", async () => {
    await handleAbility(makeCtx().ctx, db);
    const { ctx, replies } = makeCtx();
    await handleGiveup(ctx, db);
    expect(replies[0].body).toContain("🏳️");
    expect(replies[0].body).toMatch(/\([PQWER]\)/);
    const s = await loadStats(db, 1);
    expect(s).toMatchObject({ played: 1, wins: 0 });
  });

  it("stats renders zero-state", async () => {
    const { ctx, replies } = makeCtx();
    await handleStats(ctx, db);
    expect(replies[0].body).toContain("Played: 0");
  });
});
