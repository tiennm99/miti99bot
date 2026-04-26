/**
 * @file mongo-kv-store — MongoDB Atlas implementation of the KVStore interface.
 *
 * Behavioral parity with `CFKVStore`, with one documented divergence:
 *   - CFKVStore TTL is enforced server-side (eventual, ~1s granularity).
 *   - MongoKVStore also enforces TTL at read-time via an `expiresAt` filter,
 *     eliminating the up-to-60s Atlas TTL-sweeper stale-read window.
 *
 * Per-module collections: module name with `-` replaced by `_`
 * (e.g. `loldle-emoji` → `loldle_emoji`).
 *
 * `list()` returns keys WITH the module prefix preserved — the wrapper in
 * `create-store.js:65` strips it. MongoKVStore never strips prefixes.
 *
 * @see ./kv-store-interface.js for the full interface contract.
 * @module db/mongo-kv-store
 */

import { getDb } from "./mongo-client.js";
import { listWithCursor } from "./mongo-list-cursor.js";

/**
 * @typedef {import("./kv-store-interface.js").KVStore} KVStore
 * @typedef {import("./kv-store-interface.js").KVStorePutOptions} KVStorePutOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListOptions} KVStoreListOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListResult} KVStoreListResult
 */

/**
 * Tracks which collections have already had the TTL index created this
 * isolate lifetime, to avoid redundant `createIndex` round-trips.
 *
 * @type {Set<string>}
 */
const indexedCollections = new Set();

/**
 * @implements {KVStore}
 */
export class MongoKVStore {
  /**
   * @param {{ MONGODB_URI: string }} env — Worker env (or test double).
   * @param {string} collectionName — module name (e.g. "wordle", "loldle-emoji").
   * @param {import("mongodb").Db} [dbOverride] — injected Db for tests; bypasses real connect.
   */
  constructor(env, collectionName, dbOverride) {
    if (!collectionName) throw new Error("MongoKVStore: collectionName is required");
    this._env = env;
    // Normalize collection name: replace `-` with `_` for MongoDB compatibility.
    this._collName = collectionName.replace(/-/g, "_");
    this._dbOverride = dbOverride ?? null;
  }

  /**
   * Resolve the Db instance (override for tests, real for prod).
   *
   * @returns {Promise<import("mongodb").Db>}
   */
  async _db() {
    return this._dbOverride ?? getDb(this._env);
  }

  /**
   * Lazily create the TTL index once per collection per isolate.
   * Idempotent on the MongoDB side; tracked locally to avoid extra round-trips.
   *
   * @returns {Promise<void>}
   */
  async _ensureIndex() {
    if (indexedCollections.has(this._collName)) return;
    const db = await this._db();
    await db
      .collection(this._collName)
      .createIndex({ expiresAt: 1 }, { expireAfterSeconds: 0, sparse: true });
    indexedCollections.add(this._collName);
  }

  /**
   * Build the read-time TTL filter: accept docs with no `expiresAt` field,
   * OR docs whose `expiresAt` is in the future.
   *
   * @param {string} key
   * @returns {object} MongoDB filter
   */
  _liveFilter(key) {
    return {
      _id: key,
      $or: [{ expiresAt: { $exists: false } }, { expiresAt: { $gt: new Date() } }],
    };
  }

  /**
   * @param {string} key
   * @returns {Promise<string|null>}
   */
  async get(key) {
    await this._ensureIndex();
    const db = await this._db();
    const doc = await db.collection(this._collName).findOne(this._liveFilter(key));
    return doc ? doc.value : null;
  }

  /**
   * @param {string} key
   * @param {string} value
   * @param {KVStorePutOptions} [opts]
   * @returns {Promise<void>}
   */
  async put(key, value, opts) {
    await this._ensureIndex();
    const db = await this._db();
    const $set = { value };
    const $unset = {};

    if (opts?.expirationTtl) {
      $set.expiresAt = new Date(Date.now() + opts.expirationTtl * 1000);
    } else {
      // Clear any existing TTL so the document becomes permanent.
      $unset.expiresAt = "";
    }

    await db.collection(this._collName).updateOne({ _id: key }, { $set, $unset }, { upsert: true });
  }

  /**
   * @param {string} key
   * @returns {Promise<void>}
   */
  async delete(key) {
    await this._ensureIndex();
    const db = await this._db();
    await db.collection(this._collName).deleteOne({ _id: key });
  }

  /**
   * List keys matching an optional prefix, with cursor-based pagination.
   * Keys are returned WITH the full module prefix preserved — the wrapper in
   * `create-store.js` strips it for callers.
   *
   * @param {KVStoreListOptions} [opts]
   * @returns {Promise<KVStoreListResult>}
   */
  async list(opts = {}) {
    await this._ensureIndex();
    const db = await this._db();
    return listWithCursor(db.collection(this._collName), opts);
  }

  /**
   * @param {string} key
   * @returns {Promise<any|null>}
   */
  async getJSON(key) {
    const raw = await this.get(key);
    if (raw == null) return null;
    try {
      return JSON.parse(raw);
    } catch (err) {
      console.warn("getJSON: parse failed", { key, err: String(err) });
      return null;
    }
  }

  /**
   * @param {string} key
   * @param {any} value
   * @param {KVStorePutOptions} [opts]
   * @returns {Promise<void>}
   */
  async putJSON(key, value, opts) {
    if (value === undefined) {
      throw new Error(`putJSON: value for key "${key}" is undefined`);
    }
    // JSON.stringify throws on cycles — let it propagate.
    const serialized = JSON.stringify(value);
    await this.put(key, serialized, opts);
  }
}
