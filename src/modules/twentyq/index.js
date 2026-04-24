/**
 * @file Twentyq module — reverse-Akinator yes/no guessing game.
 *
 * Bot picks a secret keyword from ./seeds.js. Workers AI
 * (@cf/google/gemma-4-26b-a4b-it) generates {category, initialHint} at
 * round start and emits a one-line JSON {is_guess, answer, hint} per turn.
 * Round ends on a correct guess or /twentyq_giveup. Unlimited turns.
 * Per-subject state in KV (user id in DMs, chat id in groups).
 *
 * `init` captures both the prefixed KV store AND the raw env so handlers can
 * reach env.AI per request without changing the dispatcher contract.
 */

import { handleGiveup, handleStats, handleTwentyq } from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;
/** @type {any} */
let aiEnv = null;

/** @type {import("../registry.js").BotModule} */
const twentyqModule = {
  name: "twentyq",
  init: async ({ db: store, env }) => {
    db = store;
    aiEnv = env;
  },
  commands: [
    {
      name: "twentyq",
      visibility: "public",
      description: "20 questions — bot picks an object, you ask yes/no questions",
      handler: (ctx) => handleTwentyq(ctx, { db, env: aiEnv }),
    },
    {
      name: "twentyq_giveup",
      visibility: "public",
      description: "Reveal the current twentyq answer (auto-starts a fresh round)",
      handler: (ctx) => handleGiveup(ctx, { db }),
    },
    {
      name: "twentyq_stats",
      visibility: "public",
      description: "Show your twentyq stats (played, solved, best round)",
      handler: (ctx) => handleStats(ctx, { db }),
    },
  ],
};

export default twentyqModule;
