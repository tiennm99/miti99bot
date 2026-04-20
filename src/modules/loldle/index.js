/**
 * @file Loldle module — classic-mode champion guessing game.
 *
 * Ported from tiennm99/loldle (lib/classic-mode.js). Data sourced from
 * tiennm99/loldle-data's champions.json (synced via GH Actions).
 */

import { handleGiveup, handleLoldle, handleStats } from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const loldleModule = {
  name: "loldle",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "loldle",
      visibility: "public",
      description: "Classic loldle — guess today's champion",
      handler: (ctx) => handleLoldle(ctx, db),
    },
    {
      name: "loldle_giveup",
      visibility: "public",
      description: "Reveal today's loldle answer",
      handler: (ctx) => handleGiveup(ctx, db),
    },
    {
      name: "loldle_stats",
      visibility: "public",
      description: "Show your loldle stats (wins, streak)",
      handler: (ctx) => handleStats(ctx, db),
    },
  ],
};

export default loldleModule;
