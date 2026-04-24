/**
 * @file loldle-ability — guess the champion from a single ability icon.
 * Pool seeded from Data Dragon (same source loldle.net uses at runtime).
 */

import { handleAbility, handleGiveup, handleStats } from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const loldleAbilityModule = {
  name: "loldle-ability",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "loldle_ability",
      visibility: "public",
      description: "Ability loldle — guess the champion from an ability icon",
      handler: (ctx) => handleAbility(ctx, db),
    },
    {
      name: "loldle_ability_giveup",
      visibility: "public",
      description: "Reveal the current ability loldle answer",
      handler: (ctx) => handleGiveup(ctx, db),
    },
    {
      name: "loldle_ability_stats",
      visibility: "public",
      description: "Show your ability loldle stats (wins, streak)",
      handler: (ctx) => handleStats(ctx, db),
    },
  ],
};

export default loldleAbilityModule;
