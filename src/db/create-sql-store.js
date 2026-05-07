/**
 * @file create-sql-store — factory returning a namespaced SqlStore for a module.
 *
 * Table naming is by convention: `{moduleName}_{table}`. Authors write the
 * full prefixed name directly in SQL (e.g. `trading_trades`). `tablePrefix`
 * is exposed for authors who want to interpolate the prefix dynamically.
 *
 * Returns null when `env.DB` is absent so modules that don't use D1 have
 * zero overhead — the registry passes `sql: null` and modules check for it.
 */

import { CFSqlStore } from "./cf-sql-store.js";

/**
 * @typedef {import("./sql-store-interface.js").SqlStore} SqlStore
 */

const MODULE_NAME_RE = /^[a-z0-9_-]+$/;

/**
 * @param {string} moduleName — must match `[a-z0-9_-]+`.
 * @param {{ DB?: D1Database }} env — worker env (or test double).
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
