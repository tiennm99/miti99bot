/**
 * @file cf-sql-store — thin wrapper around a Cloudflare D1 database binding.
 *
 * Exposes `prepare`, `run`, `all`, `first`, and `batch` using the D1
 * prepared-statement API. This is the production implementation of SqlStore.
 * Tests use `fake-d1.js` instead.
 */

/**
 * @typedef {import("./sql-store-interface.js").SqlStore} SqlStore
 * @typedef {import("./sql-store-interface.js").SqlRunResult} SqlRunResult
 */

export class CFSqlStore {
  /** @param {D1Database} db */
  constructor(db) {
    this._db = db;
  }

  /**
   * Returns a bound D1PreparedStatement for advanced use (e.g. batch()).
   *
   * @param {string} query
   * @param {...any} binds
   * @returns {D1PreparedStatement}
   */
  prepare(query, ...binds) {
    const stmt = this._db.prepare(query);
    return binds.length > 0 ? stmt.bind(...binds) : stmt;
  }

  /**
   * Execute a write statement (INSERT/UPDATE/DELETE/CREATE).
   *
   * @param {string} query
   * @param {...any} binds
   * @returns {Promise<SqlRunResult>}
   */
  async run(query, ...binds) {
    const result = await this.prepare(query, ...binds).run();
    return {
      changes: result.meta?.changes ?? 0,
      last_row_id: result.meta?.last_row_id ?? 0,
    };
  }

  /**
   * Execute a SELECT and return all matching rows.
   *
   * @param {string} query
   * @param {...any} binds
   * @returns {Promise<any[]>}
   */
  async all(query, ...binds) {
    const result = await this.prepare(query, ...binds).all();
    return result.results ?? [];
  }

  /**
   * Execute a SELECT and return the first row, or null.
   *
   * @param {string} query
   * @param {...any} binds
   * @returns {Promise<any|null>}
   */
  async first(query, ...binds) {
    return this.prepare(query, ...binds).first() ?? null;
  }

  /**
   * Execute multiple prepared statements in a single round-trip.
   *
   * @param {D1PreparedStatement[]} statements
   * @returns {Promise<any[]>}
   */
  async batch(statements) {
    const results = await this._db.batch(statements);
    return results.map((r) => r.results ?? []);
  }
}
