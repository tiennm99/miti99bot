/**
 * @file Semantle module — word2vec similarity guessing game.
 *
 * Target words come from our own hosted word2sim instance
 * (default: https://word2sim.sg.miti99.com). Override via env var
 * `WORD2SIM_API_URL` for local dev or self-hosting.
 */

import { createClient } from "./api-client.js";
import { handleGiveup, handleSemantle, handleStats } from "./handlers.js";

const DEFAULT_API_URL = "https://word2sim.sg.miti99.com";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;
/** @type {ReturnType<typeof createClient> | null} */
let client = null;

/** @type {import("../registry.js").BotModule} */
const semantleModule = {
  name: "semantle",
  init: async ({ db: store, env }) => {
    db = store;
    const base = env?.WORD2SIM_API_URL || DEFAULT_API_URL;
    client = createClient(base);
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
