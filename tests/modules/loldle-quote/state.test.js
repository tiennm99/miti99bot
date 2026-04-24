import { beforeEach, describe, expect, it } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  clearGame,
  loadGame,
  loadStats,
  recordResult,
  saveGame,
} from "../../../src/modules/loldle-quote/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

describe("loldle-quote state", () => {
  let db;

  beforeEach(() => {
    db = createStore("loldle-quote", { KV: makeFakeKv() });
  });

  it("round-trips a game", async () => {
    const state = { target: "Garen", guesses: [], startedAt: null };
    await saveGame(db, 99, state);
    expect(await loadGame(db, 99)).toEqual(state);
  });

  it("clearGame deletes the record", async () => {
    await saveGame(db, 99, { target: "Garen", guesses: [], startedAt: null });
    await clearGame(db, 99);
    expect(await loadGame(db, 99)).toBeNull();
  });

  it("recordResult tracks streaks and bestStreak", async () => {
    await recordResult(db, 99, true);
    await recordResult(db, 99, true);
    await recordResult(db, 99, true);
    await recordResult(db, 99, false);
    const s = await loadStats(db, 99);
    expect(s).toMatchObject({ played: 4, wins: 3, streak: 0, bestStreak: 3 });
  });
});
