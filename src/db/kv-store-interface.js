/**
 * @file KVStore interface — JSDoc typedefs only, no runtime code.
 *
 * This is the contract every storage backend must satisfy. modules receive
 * a prefixed `KVStore` (via {@link module:db/create-store}) and must NEVER
 * touch the underlying binding. to swap Cloudflare KV for a different
 * backend (D1, Upstash Redis, ...) in the future, implement this interface
 * in a new file and change the one import in create-store.js — no module
 * code changes required.
 */

/**
 * @typedef {Object} KVStorePutOptions
 * @property {number} [expirationTtl] seconds — value auto-deletes after this many seconds.
 */

/**
 * @typedef {Object} KVStoreListOptions
 * @property {string} [prefix] additional prefix (appended AFTER the module namespace).
 * @property {number} [limit]
 * @property {string} [cursor] pagination cursor from a previous list() call.
 */

/**
 * @typedef {Object} KVStoreListResult
 * @property {string[]} keys — module namespace already stripped.
 * @property {string} [cursor] — present if more pages available.
 * @property {boolean} done — true when list_complete.
 */

/**
 * @typedef {Object} KVStore
 * @property {(key: string) => Promise<string|null>} get
 * @property {(key: string, value: string, opts?: KVStorePutOptions) => Promise<void>} put
 * @property {(key: string) => Promise<void>} delete
 * @property {(opts?: KVStoreListOptions) => Promise<KVStoreListResult>} list
 * @property {(key: string) => Promise<any|null>} getJSON
 *   returns null on missing key OR malformed JSON (logs a warning — does not throw).
 * @property {(key: string, value: any, opts?: KVStorePutOptions) => Promise<void>} putJSON
 *   throws if value is undefined or contains a cycle.
 */

// JSDoc-only module. No runtime exports.
export {};
