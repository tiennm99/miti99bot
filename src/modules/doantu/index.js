/**
 * @file Doantu module — Vietnamese semantle.
 *
 * Targets from a curated local wordlist (duyet/vietnamese-wordlist Viet22K);
 * similarity scores from api.conceptnet.io's `/relatedness` endpoint against
 * `/c/vi/<term>` concept URIs. All commands are `protected` — listed in
 * /help but hidden from Telegram's native / autocomplete menu.
 */

import { createClient } from "./api-client.js";
import { handleDoantu, handleGiveup, handleStats } from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;
/** @type {ReturnType<typeof createClient> | null} */
let client = null;

/** @type {import("../registry.js").BotModule} */
const doantuModule = {
  name: "doantu",
  init: async ({ db: store }) => {
    db = store;
    client = createClient();
  },
  commands: [
    {
      name: "doantu",
      visibility: "protected",
      description: "Đoán từ — Vietnamese semantic word guessing (unlimited tries)",
      handler: (ctx) => handleDoantu(ctx, { db, client }),
    },
    {
      name: "doantu_giveup",
      visibility: "protected",
      description: "Reveal the current doantu answer (auto-starts a fresh round)",
      handler: (ctx) => handleGiveup(ctx, { db, client }),
    },
    {
      name: "doantu_stats",
      visibility: "protected",
      description: "Show your doantu stats",
      handler: (ctx) => handleStats(ctx, { db, client }),
    },
  ],
};

export default doantuModule;
