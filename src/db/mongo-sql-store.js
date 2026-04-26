/**
 * @file mongo-sql-store — thin SqlStore shim wrapping MongoTradesStore.
 *
 * PURPOSE (intentionally limited):
 *   This shim exists ONLY for `create-sql-store.js` factory branching and
 *   `tests/db/create-sql-store.test.js` contract compliance. The trading
 *   module is being refactored to call MongoTradesStore directly (Phase 03).
 *   The shim's `run`/`all`/`first` translate the 6 known trading SQL patterns
 *   for any caller that still routes through the SqlStore interface; everything
 *   else throws immediately — no silent fallthrough.
 *
 * `prepare` and `batch` are unsupported: trading code never calls them, and
 * the dual-write layer (Phase 04) uses MongoTradesStore directly.
 *
 * @module db/mongo-sql-store
 */

import { MongoTradesStore } from "./mongo-trades-store.js";

/**
 * @typedef {import("./sql-store-interface.js").SqlStore} SqlStore
 * @typedef {import("./sql-store-interface.js").SqlRunResult} SqlRunResult
 */

/**
 * @implements {SqlStore}
 */
export class MongoSqlStore {
  /**
   * @param {{ MONGODB_URI: string }} env — Worker env (or test double).
   * @param {string} moduleName — used to derive `tablePrefix`.
   * @param {MongoTradesStore} [tradesStoreOverride] — injected store for tests.
   */
  constructor(env, moduleName, tradesStoreOverride) {
    /** @type {string} */
    this.tablePrefix = `${moduleName}_`;
    this._store = tradesStoreOverride ?? new MongoTradesStore(env);
  }

  /**
   * Execute a write statement by delegating to MongoTradesStore.
   * Recognises INSERT INTO trading_trades only.
   * Returns `{ changes: 1, last_row_id: 0 }` — last_row_id is a NUMBER (not hex).
   *
   * @param {string} query
   * @param {...any} binds  [userId, symbol, side, qty, priceVnd, ts]
   * @returns {Promise<SqlRunResult>}
   */
  async run(query, ...binds) {
    const q = query.replace(/\s+/g, " ").trim();
    if (/INSERT\s+INTO\s+trading_trades\b/i.test(q)) {
      // Bind order mirrors history.js: (user_id, symbol, side, qty, price_vnd, ts)
      const [userId, symbol, side, qty, priceVnd, ts] = binds;
      return this._store.insert({ userId, symbol, side, qty, priceVnd, ts });
    }
    // DELETE WHERE id IN (...) — from retention.js buildDeleteByIds
    if (/DELETE\s+FROM\s+trading_trades\s+WHERE\s+id\s+IN\b/i.test(q)) {
      const ids = binds;
      await this._store.deleteByIds(ids);
      return { changes: ids.length, last_row_id: 0 };
    }
    throw new Error(
      "MongoSqlStore: unsupported query — refactor caller to use MongoTradesStore directly",
    );
  }

  /**
   * Execute a read query by delegating to MongoTradesStore.
   * Recognises:
   *   - SELECT … FROM trading_trades WHERE user_id = ? ORDER BY ts DESC LIMIT ?
   *   - SELECT DISTINCT user_id FROM trading_trades
   *   - SELECT id FROM trading_trades WHERE user_id = ? ORDER BY ts DESC … OFFSET ?
   *   - SELECT id FROM trading_trades ORDER BY ts DESC … OFFSET ?
   *
   * @param {string} query
   * @param {...any} binds
   * @returns {Promise<any[]>}
   */
  async all(query, ...binds) {
    const q = query.replace(/\s+/g, " ").trim();

    // SELECT DISTINCT user_id
    if (/SELECT\s+DISTINCT\s+user_id\b/i.test(q)) {
      const ids = await this._store.distinctUsers();
      return ids.map((id) => ({ user_id: id }));
    }

    // SELECT id FROM trading_trades WHERE user_id = ? ORDER BY ts DESC … OFFSET ?
    if (
      /SELECT\s+id\s+FROM\s+trading_trades\s+WHERE\s+user_id\s*=\s*\?/i.test(q) &&
      /OFFSET\s+\?/i.test(q)
    ) {
      const [userId, keepN] = binds;
      return this._store.oldRowsForUser(userId, keepN);
    }

    // SELECT id FROM trading_trades ORDER BY ts DESC … OFFSET ?
    if (/SELECT\s+id\s+FROM\s+trading_trades\b/i.test(q) && /OFFSET\s+\?/i.test(q)) {
      const [keepN] = binds;
      return this._store.oldRows(keepN);
    }

    // SELECT … FROM trading_trades WHERE user_id = ? … LIMIT ? (history query)
    if (
      /SELECT\b.*\bFROM\s+trading_trades\s+WHERE\s+user_id\s*=\s*\?/i.test(q) &&
      /LIMIT\s+\?/i.test(q)
    ) {
      const [userId, limit] = binds;
      return this._store.byUser(userId, limit);
    }

    throw new Error(
      "MongoSqlStore: unsupported query — refactor caller to use MongoTradesStore directly",
    );
  }

  /**
   * Execute a read query and return the first row, or null.
   *
   * @param {string} query
   * @param {...any} binds
   * @returns {Promise<any|null>}
   */
  async first(query, ...binds) {
    const rows = await this.all(query, ...binds);
    return rows[0] ?? null;
  }

  /**
   * Not supported — trading code does not use prepared statements via MongoSqlStore.
   * @throws {Error}
   */
  prepare() {
    throw new Error("unsupported in MongoSqlStore");
  }

  /**
   * Not supported — trading code does not use batch via MongoSqlStore.
   * @throws {Error}
   */
  batch() {
    throw new Error("unsupported in MongoSqlStore");
  }
}
