/**
 * @file bot — lazy, memoized grammY Bot factory.
 *
 * The Bot instance is constructed on the first request (env is not available
 * at module-load time in CF Workers) and reused on subsequent warm requests.
 * Dispatcher middleware is installed exactly once, as part of that same
 * lazy init, so the registry is built before any update is routed.
 */

import { Bot } from "grammy";
import { installDispatcher } from "./modules/dispatcher.js";

/** @type {Bot | null} */
let botInstance = null;
/** @type {Promise<Bot> | null} */
let botInitPromise = null;

/**
 * Fail fast if any required env var is missing — better a 500 on first webhook
 * than a confusing runtime error inside grammY.
 *
 * @param {any} env
 */
function requireEnv(env) {
  const required = ["TELEGRAM_BOT_TOKEN", "TELEGRAM_WEBHOOK_SECRET", "MODULES"];
  const missing = required.filter((key) => !env?.[key]);
  if (missing.length > 0) {
    throw new Error(`missing required env vars: ${missing.join(", ")}`);
  }
}

/**
 * @param {any} env
 * @returns {Promise<Bot>}
 */
export async function getBot(env) {
  if (botInstance) return botInstance;
  if (botInitPromise) return botInitPromise;

  requireEnv(env);

  botInitPromise = (async () => {
    const bot = new Bot(env.TELEGRAM_BOT_TOKEN);
    await installDispatcher(bot, env);
    botInstance = bot;
    return bot;
  })();

  try {
    return await botInitPromise;
  } catch (err) {
    // Clear the failed promise so the next request retries init.
    botInitPromise = null;
    throw err;
  }
}
