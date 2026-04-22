/**
 * @file Semantle module — similarity guessing game backed by Cloudflare Workers AI.
 *
 * Targets come from a curated local wordlist (same list doubles as the
 * vocabulary for OOV detection, so no upstream check is needed to pick or
 * validate a word). Similarity scores come from cosine distance between
 * `@cf/baai/bge-m3` multilingual embeddings produced by the `env.AI` binding.
 */

import { createClient } from "./api-client.js";
import { handleGiveup, handleSemantle, handleStats } from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;
/** @type {ReturnType<typeof createClient> | null} */
let client = null;

/** @type {import("../registry.js").BotModule} */
const semantleModule = {
  name: "semantle",
  init: async ({ db: store, env }) => {
    db = store;
    client = createClient(env.AI);
  },
  commands: [
    {
      name: "semantle",
      visibility: "public",
      description: "Semantle — guess the hidden word (unlimited tries)",
      handler: (ctx) => handleSemantle(ctx, { db, client }),
    },
    {
      name: "semantle_giveup",
      visibility: "public",
      description: "Reveal the current semantle answer (auto-starts a fresh round)",
      handler: (ctx) => handleGiveup(ctx, { db, client }),
    },
    {
      name: "semantle_stats",
      visibility: "public",
      description: "Show your semantle stats",
      handler: (ctx) => handleStats(ctx, { db, client }),
    },
  ],
};

export default semantleModule;
