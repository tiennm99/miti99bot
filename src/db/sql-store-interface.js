/**
 * @file SqlStore interface — JSDoc typedefs only, no runtime code.
 *
 * This is the contract every SQL storage backend must satisfy. Modules
 * receive a prefixed `SqlStore` (via {@link module:db/create-sql-store}) and
 * must NEVER touch the underlying `env.DB` binding directly.
 *
 * Table naming convention: `{moduleName}_{table}` (e.g. `trading_trades`).
 * Enforced by convention — `tablePrefix` is exposed so authors can interpolate
 * it when building dynamic table names, but most authors hard-code the full
 * prefixed table name directly in their SQL.
 */

/**
 * Raw D1 run result.
 *
 * @typedef {object} SqlRunResult
 * @property {number} changes — rows affected by INSERT/UPDATE/DELETE.
 * @property {number} last_row_id — rowid of the last inserted row (0 if none).
 */

/**
 * @typedef {object} SqlStore
 * @property {string} tablePrefix
 *   Convenience prefix `"${moduleName}_"`. Authors may interpolate this when
 *   constructing dynamic table names.
 * @property {(query: string, ...binds: any[]) => Promise<SqlRunResult>} run
 *   Execute a write statement (INSERT/UPDATE/DELETE/CREATE). Returns metadata.
 * @property {(query: string, ...binds: any[]) => Promise<any[]>} all
 *   Execute a SELECT and return all matching rows as plain objects.
 * @property {(query: string, ...binds: any[]) => Promise<any|null>} first
 *   Execute a SELECT and return the first row, or null if no rows match.
 * @property {(query: string, ...binds: any[]) => D1PreparedStatement} prepare
 *   Expose the underlying prepared statement for advanced use (e.g. batch()).
 * @property {(statements: D1PreparedStatement[]) => Promise<any[]>} batch
 *   Execute multiple prepared statements in a single round-trip.
 */

// JSDoc-only module. No runtime exports.
export {};
