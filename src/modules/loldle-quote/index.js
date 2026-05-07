/**
 * @file loldle-quote — guess the champion from a one-sentence lore blurb.
 * Text-only (no audio). Pool seeded from DDragon's champion title + lore
 * (see scripts/fetch-ddragon-data.js).
 */

import { handleGiveup, handleQuote, handleSetMax, handleStats } from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const loldleQuoteModule = {
  name: "loldle-quote",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "loldle_quote",
      visibility: "public",
      description: "Quote loldle — guess the champion from a lore blurb",
      handler: (ctx) => handleQuote(ctx, db),
    },
    {
      name: "loldle_quote_giveup",
      visibility: "public",
      description: "Reveal the current quote loldle answer",
      handler: (ctx) => handleGiveup(ctx, db),
    },
    {
      name: "loldle_quote_stats",
      visibility: "public",
      description: "Show your quote loldle stats (wins, streak)",
      handler: (ctx) => handleStats(ctx, db),
    },
    {
      name: "loldle_quote_setmax",
      visibility: "private",
      description: "Override quote loldle max guesses per round (1-10)",
      handler: (ctx) => handleSetMax(ctx, db),
    },
  ],
};

export default loldleQuoteModule;
