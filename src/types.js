/**
 * @file Central JSDoc typedefs for the miti99bot codebase.
 *
 * No runtime exports — this file exists solely to provide type definitions
 * that are visible to editor tooling and validated by eslint-plugin-jsdoc.
 *
 * Import-reexport typedefs from sub-modules live here so that consumers
 * can `@typedef {import("../types.js").Env} Env` without chasing paths.
 */

/**
 * Cloudflare Worker bindings + environment variables.
 *
 * @typedef {object} Env
 * @property {string} TELEGRAM_BOT_TOKEN — bot token from BotFather.
 * @property {string} TELEGRAM_WEBHOOK_SECRET — secret for X-Telegram-Bot-Api-Secret-Token header.
 * @property {string} MODULES — comma-separated list of module names to load (e.g. "trading,wordle").
 * @property {KVNamespace} KV — Cloudflare KV namespace binding.
 * @property {D1Database|undefined} DB — Cloudflare D1 database binding; absent when not configured.
 */

/**
 * A single slash-command registered by a module.
 *
 * @typedef {object} Command
 * @property {string} name — without leading slash; must match /^[a-z0-9_]{1,32}$/.
 * @property {"public"|"protected"|"private"} visibility
 *   public: shown in Telegram /menu + /help.
 *   protected: shown in /help only.
 *   private: hidden easter egg — functional but undiscoverable.
 * @property {string} description — ≤256 chars; required for all visibilities.
 * @property {(ctx: any) => Promise<void>} handler — grammY context handler.
 */

/**
 * A scheduled cron job registered by a module.
 *
 * @typedef {object} Cron
 * @property {string} schedule — cron expression (e.g. "0 0 * * *").
 * @property {string} name — human-readable identifier for logging.
 * @property {(event: any, ctx: ModuleContext) => Promise<void>} handler — receives the scheduled event and module context.
 */

/**
 * Context object passed to module init() and cron handlers.
 *
 * @typedef {object} ModuleContext
 * @property {KVStore} db — namespaced KV store for this module.
 * @property {SqlStore|null} sql — namespaced D1 store; null when env.DB is not bound.
 * @property {Env} env — raw worker environment bindings.
 */

/**
 * A plug-n-play module definition.
 *
 * @typedef {object} Module
 * @property {string} name — unique module identifier; must match folder name and import-map key.
 * @property {(ctx: ModuleContext) => Promise<void>} [init] — optional async setup called once at startup.
 * @property {Command[]} commands — slash-commands provided by this module.
 * @property {Cron[]} [crons] — optional scheduled jobs provided by this module.
 */

/**
 * A single trade record stored in D1.
 *
 * @typedef {object} Trade
 * @property {number} id — auto-increment primary key.
 * @property {number} userId — Telegram user ID.
 * @property {string} symbol — asset ticker (e.g. "TCB", "BTC").
 * @property {"buy"|"sell"} side — direction of the trade.
 * @property {number} qty — quantity traded (integer for stocks).
 * @property {number} priceVnd — execution price in VND.
 * @property {number} ts — Unix timestamp (ms) of the trade.
 */

// Re-export typedefs from sub-modules so consumers have a single import path.
/**
 * @typedef {import("./db/kv-store-interface.js").KVStore} KVStore
 * @typedef {import("./db/kv-store-interface.js").KVStorePutOptions} KVStorePutOptions
 * @typedef {import("./db/kv-store-interface.js").KVStoreListOptions} KVStoreListOptions
 * @typedef {import("./db/kv-store-interface.js").KVStoreListResult} KVStoreListResult
 */

/**
 * @typedef {import("./db/sql-store-interface.js").SqlStore} SqlStore
 * @typedef {import("./db/sql-store-interface.js").SqlRunResult} SqlRunResult
 */

/**
 * @typedef {import("./modules/trading/portfolio.js").Portfolio} Portfolio
 * @typedef {import("./modules/trading/portfolio.js").PortfolioMeta} PortfolioMeta
 */

// JSDoc-only module. No runtime exports.
export {};
