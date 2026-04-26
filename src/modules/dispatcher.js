/**
 * @file dispatcher — wires every command (public, protected, AND private)
 * into the grammY Bot via `bot.command()`.
 *
 * Visibility is pure metadata at this layer: the dispatcher does NOT care
 * whether a command is hidden from the menu or from /help. All three paths
 * share a single bot.command() registration. Visibility only affects:
 *   1. What scripts/register.js pushes to Telegram's setMyCommands (public only).
 *   2. What phase-05's /help renderer shows (public + protected).
 *
 * Telemetry: a bot.use() middleware installed before command registration
 * emits a structured `cmd_timing` JSON line via console.log for every
 * command request. This preserves the original handler references so that
 * the dispatcher test's strict-equality assertion (`handler === cmd.handler`)
 * continues to pass.
 */

import { getLastCold } from "../util/request-context.js";
import { startTiming } from "../util/timing.js";
import { buildRegistry } from "./registry.js";

/**
 * Build the registry (if not already built) and register every command with grammY.
 * Also installs a timing middleware (bot.use) that emits cmd_timing logs.
 *
 * @param {import("grammy").Bot} bot
 * @param {any} env
 * @returns {Promise<import("./registry.js").Registry>}
 */
export async function installDispatcher(bot, env) {
  const reg = await buildRegistry(env);

  // Install timing middleware BEFORE command handlers so it wraps every command.
  // Uses bot.use() to avoid wrapping individual handlers — keeping handler
  // references identical to cmd.handler (required by dispatcher.test.js).
  if (typeof bot.use === "function") {
    bot.use(async (ctx, next) => {
      // Only time actual bot_command messages; pass other updates through untimed.
      const entities = ctx.message?.entities ?? ctx.channelPost?.entities ?? [];
      const isBotCommand = entities.some((e) => e.type === "bot_command" && e.offset === 0);

      if (!isBotCommand) {
        return next();
      }

      // Extract command name without the leading slash for the timing label.
      const rawText = ctx.message?.text ?? ctx.channelPost?.text ?? "";
      // Command is the first word (e.g. "/wordle" or "/wordle@botname").
      const cmdToken = rawText.split(" ")[0].split("@")[0];

      const t = startTiming(cmdToken);
      const { cold, isolateAgeMs } = getLastCold();

      try {
        await next();
        t.end({ cold, isolateAgeMs });
      } catch (err) {
        t.end({ cold, isolateAgeMs, error: err instanceof Error ? err.message : String(err) });
        throw err;
      }
    });
  }

  for (const { cmd } of reg.allCommands.values()) {
    // grammY's bot.command() matches /cmd and /cmd@botname, case-sensitively,
    // which naturally satisfies the "exact, case-sensitive" rule for private commands.
    bot.command(cmd.name, cmd.handler);
  }

  return reg;
}
