import { beforeEach, describe, expect, it } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  clearGame,
  loadGame,
  loadStats,
  recordResult,
  saveGame,
} from "../../../src/modules/loldle-emoji/state.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

describe("loldle-emoji state", () => {
  let db;

  beforeEach(() => {
    db = createStore("loldle-emoji", { KV: makeFakeKv() });
  });

  it("round-trips a game", async () => {
    const state = { target: "Ahri", guesses: ["Akali"], startedAt: 1234 };
    await saveGame(db, 42, state);
    expect(await loadGame(db, 42)).toEqual(state);
  });

  it("clearGame removes the round", async () => {
    await saveGame(db, 42, { target: "Ahri", guesses: [], startedAt: null });
    await clearGame(db, 42);
    expect(await loadGame(db, 42)).toBeNull();
  });

  it("loadStats returns zeros when absent", async () => {
    expect(await loadStats(db, 42)).toEqual({
      played: 0,
      wins: 0,
      streak: 0,
      bestStreak: 0,
    });
  });

  it("recordResult(true) increments wins+streak and updates bestStreak", async () => {
    let s = await recordResult(db, 42, true);
    expect(s).toMatchObject({ played: 1, wins: 1, streak: 1, bestStreak: 1 });
    s = await recordResult(db, 42, true);
    expect(s).toMatchObject({ played: 2, wins: 2, streak: 2, bestStreak: 2 });
  });

  it("recordResult(false) resets streak", async () => {
    await recordResult(db, 42, true);
    await recordResult(db, 42, true);
    const s = await recordResult(db, 42, false);
    expect(s).toMatchObject({ played: 3, wins: 2, streak: 0, bestStreak: 2 });
  });

  it("isolates stats per subject", async () => {
    await recordResult(db, 1, true);
    await recordResult(db, 2, false);
    expect((await loadStats(db, 1)).wins).toBe(1);
    expect((await loadStats(db, 2)).wins).toBe(0);
  });
});
