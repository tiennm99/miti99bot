/**
 * @file loldle-splash — guess the champion from splash art (random skin).
 * Skin pool scraped from loldle.net, splash URLs from Data Dragon CDN.
 */

import { handleGiveup, handleSplash, handleStats } from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const loldleSplashModule = {
  name: "loldle-splash",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "loldle_splash",
      visibility: "public",
      description: "Splash loldle — guess the champion from splash art",
      handler: (ctx) => handleSplash(ctx, db),
    },
    {
      name: "loldle_splash_giveup",
      visibility: "public",
      description: "Reveal the current splash loldle answer",
      handler: (ctx) => handleGiveup(ctx, db),
    },
    {
      name: "loldle_splash_stats",
      visibility: "public",
      description: "Show your splash loldle stats (wins, streak)",
      handler: (ctx) => handleStats(ctx, db),
    },
  ],
};

export default loldleSplashModule;
