/**
 * @file create-sql-store — factory returning a namespaced SqlStore for a module.
 *
 * Table naming is by convention: `{moduleName}_{table}`. Authors write the
 * full prefixed name directly in SQL (e.g. `trading_trades`). `tablePrefix`
 * is exposed for authors who want to interpolate the prefix dynamically.
 *
 * Returns null when `env.DB` is absent so modules that don't use D1 have
 * zero overhead — the registry passes `sql: null` and modules check for it.
 *
 * ## Flag matrix (same as create-store.js, SQL edition)
 *
 * | STORAGE_PRIMARY | DUAL_WRITE | MONGODB_URI       | Result                                       |
 * |-----------------|------------|-------------------|----------------------------------------------|
 * | (unset) or kv   | 1 (default)| set, real         | DualSqlStore(CFSqlStore primary, Mongo sec.) |
 * | kv              | 0          | any               | CFSqlStore only (legacy / rollback)          |
 * | mongo           | 1          | set, real         | DualSqlStore(Mongo primary, CFSqlStore sec.) |
 * | mongo           | 0          | set, real         | MongoSqlStore only (post-cutover)            |
 * | any             | any        | unset             | CFSqlStore only (or null if env.DB absent)   |
 * | any             | any        | === STUB_SENTINEL | CFSqlStore only (deploy-time register path)  |
 *
 * Note: trading module's `tradesStore` (MongoTradesStore) is wired separately
 * via registry.js — this factory produces the generic SqlStore shim only.
 *
 * post-Phase-07: this entire function returns MongoSqlStore-only;
 * CF/D1 branches removed. The flag matrix above collapses to a single path.
 */

import { CFSqlStore } from "./cf-sql-store.js";
import { DualSqlStore } from "./dual-sql-store.js";
import { MongoSqlStore } from "./mongo-sql-store.js";

/**
 * @typedef {import("./sql-store-interface.js").SqlStore} SqlStore
 */

/** Sentinel value used by scripts/register.js to signal deploy-time context. */
const STUB_SENTINEL = "__stub_mongo__";

const MODULE_NAME_RE = /^[a-z0-9_-]+$/;

/**
 * @param {string} moduleName — must match `[a-z0-9_-]+`.
 * @param {object} env — worker env (or test double).
 * @param {any} [env.DB] — CF D1Database binding (optional).
 * @param {string} [env.MONGODB_URI] — Atlas connection string (or STUB_SENTINEL).
 * @param {string} [env.STORAGE_PRIMARY] — "kv" (default) | "mongo".
 * @param {string} [env.DUAL_WRITE] — "1" (default) | "0".
 * @returns {SqlStore | null} null when env.DB is not bound.
 */
export function createSqlStore(moduleName, env) {
  if (!moduleName || typeof moduleName !== "string") {
    throw new Error("createSqlStore: moduleName is required");
  }
  if (!MODULE_NAME_RE.test(moduleName)) {
    throw new Error(
      `createSqlStore: invalid moduleName "${moduleName}" — must match ${MODULE_NAME_RE}`,
    );
  }

  // D1 is optional — workers without a DB binding still work fine.
  if (!env?.DB) return null;

  // --- Sentinel / fallback: always CF-only ---
  const mongoUri = env.MONGODB_URI;
  if (!mongoUri || mongoUri === STUB_SENTINEL) {
    return _wrapCf(moduleName, env);
  }

  const primary = (env.STORAGE_PRIMARY ?? "kv").toLowerCase();
  const dualWrite = (env.DUAL_WRITE ?? "1") !== "0";

  if (primary === "mongo" && !dualWrite) {
    // MongoSqlStore only — post-cutover path.
    return new MongoSqlStore(env, moduleName);
  }

  const cfStore = _wrapCf(moduleName, env);
  const mongoStore = new MongoSqlStore(env, moduleName);

  if (primary === "mongo") {
    // DualSqlStore: read Mongo, write both — cutover phase.
    return new DualSqlStore(mongoStore, cfStore, env.KV);
  }

  // Default: STORAGE_PRIMARY=kv
  if (!dualWrite) {
    // Rollback path: CF only.
    return cfStore;
  }

  // DualSqlStore: read CF/D1, write both — dual-write window.
  return new DualSqlStore(cfStore, mongoStore, env.KV);
}

/**
 * Build a namespaced CFSqlStore wrapper with the same shape as before Phase 04.
 *
 * @param {string} moduleName
 * @param {{ DB: any }} env
 * @returns {SqlStore}
 */
function _wrapCf(moduleName, env) {
  const base = new CFSqlStore(env.DB);
  const tablePrefix = `${moduleName}_`;

  return {
    tablePrefix,

    prepare(query, ...binds) {
      return base.prepare(query, ...binds);
    },

    async run(query, ...binds) {
      return base.run(query, ...binds);
    },

    async all(query, ...binds) {
      return base.all(query, ...binds);
    },

    async first(query, ...binds) {
      return base.first(query, ...binds);
    },

    async batch(statements) {
      return base.batch(statements);
    },
  };
}
