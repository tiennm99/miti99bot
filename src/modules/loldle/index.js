/**
 * @file Loldle module — classic-mode champion guessing game.
 * Champion data is scraped weekly from loldle.net.
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
      description: "Classic loldle — guess the current champion",
      handler: (ctx) => handleLoldle(ctx, db),
    },
    {
      name: "loldle_giveup",
      visibility: "public",
      description: "Reveal the current loldle answer (auto-starts a fresh round)",
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
