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
import { getCurrentRegistry } from "./modules/registry.js";

/** @type {Bot | null} */
let botInstance = null;
/** @type {Promise<Bot> | null} */
let botInitPromise = null;

/**
 * Returns the memoized registry, building it (and the bot) if needed.
 * Shares the same instance used by the fetch handler so scheduled() and
 * fetch() operate on identical registry state within a warm instance.
 *
 * @param {any} env
 * @returns {Promise<import("./modules/registry.js").Registry>}
 */
export async function getRegistry(env) {
  // If the bot is already initialised the registry was built as a side effect.
  if (botInstance) return getCurrentRegistry();
  // Otherwise bootstrap via getBot (which calls buildRegistry internally).
  await getBot(env);
  return getCurrentRegistry();
}

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
