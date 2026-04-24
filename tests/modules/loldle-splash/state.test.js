import { beforeEach, describe, expect, it } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  clearGame,
  loadGame,
  loadStats,
  recordResult,
  saveGame,
} from "../../../src/modules/loldle-splash/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

describe("loldle-splash state", () => {
  let db;

  beforeEach(() => {
    db = createStore("loldle-splash", { KV: makeFakeKv() });
  });

  it("round-trips a game with skinId field", async () => {
    const state = { target: "Ahri", skinId: 9, guesses: [], startedAt: null };
    await saveGame(db, 99, state);
    expect(await loadGame(db, 99)).toEqual(state);
  });

  it("clearGame removes the record", async () => {
    await saveGame(db, 99, { target: "Ahri", skinId: 0, guesses: [], startedAt: null });
    await clearGame(db, 99);
    expect(await loadGame(db, 99)).toBeNull();
  });

  it("recordResult tracks streaks + bestStreak", async () => {
    await recordResult(db, 99, true);
    await recordResult(db, 99, true);
    const s = await loadStats(db, 99);
    expect(s).toMatchObject({ played: 2, wins: 2, streak: 2, bestStreak: 2 });
  });
});
