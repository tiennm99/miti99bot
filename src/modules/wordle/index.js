/**
 * @file Wordle module — classic 5-letter word guessing game.
 *
 * Word list sourced from:
 *   https://gist.github.com/dracos/dd0668f281e685bad51479e5acaadb93
 * (Anna Eilering (dracos) — combined Wordle dictionary of allowed guesses.)
 * Synced via scripts/build-wordle-data.js into ./words-data.js.
 */

import { handleGiveup, handleNew, handleStats, handleWordle } from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const wordleModule = {
  name: "wordle",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "wordle",
      visibility: "public",
      description: "Classic wordle — guess the 5-letter word",
      handler: (ctx) => handleWordle(ctx, db),
    },
    {
      name: "wordle_new",
      visibility: "public",
      description: "Start a new round (auto-gives-up any in-progress one)",
      handler: (ctx) => handleNew(ctx, db),
    },
    {
      name: "wordle_giveup",
      visibility: "public",
      description: "Reveal the current wordle answer",
      handler: (ctx) => handleGiveup(ctx, db),
    },
    {
      name: "wordle_stats",
      visibility: "public",
      description: "Show your wordle stats (wins, streak)",
      handler: (ctx) => handleStats(ctx, db),
    },
  ],
};

export default wordleModule;
