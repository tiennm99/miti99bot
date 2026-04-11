/**
 * @file misc module stub — proves the plugin system end-to-end AND
 * demonstrates DB usage via getJSON/putJSON in /ping.
 */

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const miscModule = {
  name: "misc",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "ping",
      visibility: "public",
      description: "Health check — replies pong and records last ping",
      handler: async (ctx) => {
        // Best-effort write — if KV is unavailable, still reply.
        try {
          await db?.putJSON("last_ping", { at: Date.now() });
        } catch (err) {
          console.warn("misc /ping: putJSON failed", String(err));
        }
        await ctx.reply("pong");
      },
    },
    {
      name: "mstats",
      visibility: "protected",
      description: "Show the timestamp of the last /ping",
      handler: async (ctx) => {
        const last = await db?.getJSON("last_ping");
        if (last?.at) {
          await ctx.reply(`last ping: ${new Date(last.at).toISOString()}`);
        } else {
          await ctx.reply("last ping: never");
        }
      },
    },
    {
      name: "fortytwo",
      visibility: "private",
      description: "Easter egg — the answer",
      handler: async (ctx) => {
        await ctx.reply("The answer.");
      },
    },
  ],
};

export default miscModule;
