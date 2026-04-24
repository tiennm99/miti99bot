import { beforeEach, describe, expect, it } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  clearGame,
  loadGame,
  loadStats,
  recordResult,
  saveGame,
} from "../../../src/modules/loldle-ability/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

describe("loldle-ability state", () => {
  let db;

  beforeEach(() => {
    db = createStore("loldle-ability", { KV: makeFakeKv() });
  });

  it("round-trips a game with slot field", async () => {
    const state = { target: "Ahri", slot: "Q", guesses: ["Akali"], startedAt: 1234 };
    await saveGame(db, 42, state);
    expect(await loadGame(db, 42)).toEqual(state);
  });

  it("clearGame removes the record", async () => {
    await saveGame(db, 42, { target: "Ahri", slot: "P", guesses: [], startedAt: null });
    await clearGame(db, 42);
    expect(await loadGame(db, 42)).toBeNull();
  });

  it("recordResult tracks streaks + bestStreak", async () => {
    await recordResult(db, 42, true);
    await recordResult(db, 42, true);
    await recordResult(db, 42, false);
    const s = await loadStats(db, 42);
    expect(s).toMatchObject({ played: 3, wins: 2, streak: 0, bestStreak: 2 });
  });
});
