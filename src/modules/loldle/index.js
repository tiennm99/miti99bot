/**
 * @file loldle module stub — proves the plugin system end-to-end.
 *
 * One public, one protected, one private slash command.
 */

/** @type {import("../registry.js").BotModule} */
const loldleModule = {
  name: "loldle",
  commands: [
    {
      name: "loldle",
      visibility: "public",
      description: "Play loldle (stub)",
      handler: async (ctx) => {
        await ctx.reply("Loldle stub.");
      },
    },
    {
      name: "lstats",
      visibility: "protected",
      description: "Loldle stats",
      handler: async (ctx) => {
        await ctx.reply("loldle stats stub");
      },
    },
    {
      name: "ggwp",
      visibility: "private",
      description: "Easter egg — post-match courtesy",
      handler: async (ctx) => {
        await ctx.reply("gg well played (stub)");
      },
    },
  ],
};

export default loldleModule;
