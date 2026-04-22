/**
 * @file Semantle module — similarity guessing game backed by ConceptNet.
 *
 * Targets come from a curated local wordlist (ConceptNet has no /random).
 * Similarity scores come from `api.conceptnet.io/relatedness`. The ConceptNet
 * base URL is hardcoded in the client; tests can still override via
 * `createClient(url)` if needed.
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
  init: async ({ db: store }) => {
    db = store;
    client = createClient();
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
