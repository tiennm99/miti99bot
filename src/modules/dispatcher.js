/**
 * @file dispatcher — wires every command (public, protected, AND private)
 * into the grammY Bot via `bot.command()`.
 *
 * Visibility is pure metadata at this layer: the dispatcher does NOT care
 * whether a command is hidden from the menu or from /help. All three paths
 * share a single bot.command() registration. Visibility only affects:
 *   1. What scripts/register.js pushes to Telegram's setMyCommands (public only).
 *   2. What phase-05's /help renderer shows (public + protected).
 */

import { buildRegistry } from "./registry.js";

/**
 * Build the registry (if not already built) and register every command with grammY.
 *
 * @param {import("grammy").Bot} bot
 * @param {any} env
 * @returns {Promise<import("./registry.js").Registry>}
 */
export async function installDispatcher(bot, env) {
  const reg = await buildRegistry(env);

  for (const { cmd } of reg.allCommands.values()) {
    // grammY's bot.command() matches /cmd and /cmd@botname, case-sensitively,
    // which naturally satisfies the "exact, case-sensitive" rule for private commands.
    bot.command(cmd.name, cmd.handler);
  }

  return reg;
}
