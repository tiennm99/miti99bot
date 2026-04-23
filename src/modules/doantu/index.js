/**
 * @file Doantu module — Vietnamese semantle.
 *
 * Target words + cosine similarity come from our own hosted phow2sim instance
 * (default: https://phow2sim.sg.miti99.com). Override via env var
 * `PHOW2SIM_API_URL` for local dev or self-hosting.
 */

import { createClient } from "./api-client.js";
import { handleDoantu, handleGiveup, handleStats } from "./handlers.js";

const DEFAULT_API_URL = "https://phow2sim.sg.miti99.com";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;
/** @type {ReturnType<typeof createClient> | null} */
let client = null;

/** @type {import("../registry.js").BotModule} */
const doantuModule = {
  name: "doantu",
  init: async ({ db: store, env }) => {
    db = store;
    const base = env?.PHOW2SIM_API_URL || DEFAULT_API_URL;
    client = createClient(base);
  },
  commands: [
    {
      name: "doantu",
      visibility: "public",
      description: "Đoán từ — Vietnamese semantic word guessing (unlimited tries)",
      handler: (ctx) => handleDoantu(ctx, { db, client }),
    },
    {
      name: "doantu_giveup",
      visibility: "public",
      description: "Reveal the current doantu answer (auto-starts a fresh round)",
      handler: (ctx) => handleGiveup(ctx, { db, client }),
    },
    {
      name: "doantu_stats",
      visibility: "public",
      description: "Show your doantu stats",
      handler: (ctx) => handleStats(ctx, { db, client }),
    },
  ],
};

export default doantuModule;
