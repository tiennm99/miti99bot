/**
 * @file loldle-emoji — guess the champion from a short emoji sequence.
 * Emoji pool derived from classic champion metadata (see
 * scripts/fetch-ddragon-data.js).
 */

import { handleEmoji, handleGiveup, handleSetMax, handleStats } from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const loldleEmojiModule = {
  name: "loldle-emoji",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "loldle_emoji",
      visibility: "public",
      description: "Emoji loldle — guess the champion from emojis",
      handler: (ctx) => handleEmoji(ctx, db),
    },
    {
      name: "loldle_emoji_giveup",
      visibility: "public",
      description: "Reveal the current emoji loldle answer",
      handler: (ctx) => handleGiveup(ctx, db),
    },
    {
      name: "loldle_emoji_stats",
      visibility: "public",
      description: "Show your emoji loldle stats (wins, streak)",
      handler: (ctx) => handleStats(ctx, db),
    },
    {
      name: "loldle_emoji_setmax",
      visibility: "private",
      description: "Override emoji loldle max guesses per round (1-10)",
      handler: (ctx) => handleSetMax(ctx, db),
    },
  ],
};

export default loldleEmojiModule;
