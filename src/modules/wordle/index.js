/**
 * @file wordle module stub — proves the plugin system end-to-end.
 *
 * One public, one protected, one private (hidden) slash command. Real game
 * logic is out of scope for v1; this exercises the loader, visibility levels,
 * registry, dispatcher, help renderer, and namespaced DB.
 */

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const wordleModule = {
  name: "wordle",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "wordle",
      visibility: "public",
      description: "Play wordle (stub)",
      handler: async (ctx) => {
        await ctx.reply("Wordle stub — real game TBD.");
      },
    },
    {
      name: "wstats",
      visibility: "protected",
      description: "Wordle stats",
      handler: async (ctx) => {
        const stats = (await db?.getJSON("stats")) ?? null;
        const played = stats?.gamesPlayed ?? 0;
        await ctx.reply(`games played: ${played}`);
      },
    },
    {
      name: "konami",
      visibility: "private",
      description: "Easter egg — retro code",
      handler: async (ctx) => {
        await ctx.reply("⬆⬆⬇⬇⬅➡⬅➡BA — secret wordle mode unlocked (stub)");
      },
    },
  ],
};

export default wordleModule;
