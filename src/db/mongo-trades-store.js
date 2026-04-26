/**
 * @file mongo-trades-store — MongoDB implementation of trade persistence.
 *
 * Wraps the `trading_trades` collection. Each document shape:
 *   { _id: ObjectId, legacy_id: number|null, user_id, symbol, side, qty, price_vnd, ts }
 *
 * `legacy_id` is set to null for new runtime trades. During Phase 05 backfill
 * it will be populated with the original D1 autoincrement row id.
 *
 * @module db/mongo-trades-store
 */

import { ObjectId } from "mongodb";
import { getDb } from "./mongo-client.js";

/** @typedef {import("../types.js").Trade} Trade */

const COLLECTION = "trading_trades";

/**
 * Tracks which collections have had their indexes created this isolate lifetime.
 * @type {Set<string>}
 */
const indexedCollections = new Set();

/**
 * @implements {object} Direct trade persistence — no SQL strings.
 */
export class MongoTradesStore {
  /**
   * @param {{ MONGODB_URI: string }} env — Worker env (or test double).
   * @param {import("mongodb").Db} [dbOverride] — injected Db for tests; bypasses real connect.
   */
  constructor(env, dbOverride) {
    this._env = env;
    this._dbOverride = dbOverride ?? null;
  }

  /** @returns {Promise<import("mongodb").Db>} */
  async _db() {
    return this._dbOverride ?? getDb(this._env);
  }

  /** Lazily create the three required indexes once per isolate. */
  async _ensureIndexes() {
    if (indexedCollections.has(COLLECTION)) return;
    const db = await this._db();
    const coll = db.collection(COLLECTION);
    await coll.createIndex({ user_id: 1, ts: -1 });
    await coll.createIndex({ ts: -1 });
    await coll.createIndex({ legacy_id: 1 }, { sparse: true });
    indexedCollections.add(COLLECTION);
  }

  /**
   * Insert a new trade document. Sets `legacy_id: null` (no D1 row id for
   * runtime-created trades). Returns SqlRunResult-compatible shape.
   *
   * @param {{ userId: number, symbol: string, side: "buy"|"sell", qty: number, priceVnd: number, ts?: number }} trade
   * @returns {Promise<{ changes: number, last_row_id: number }>}
   */
  async insert(trade) {
    await this._ensureIndexes();
    const db = await this._db();
    await db.collection(COLLECTION).insertOne({
      _id: new ObjectId(),
      legacy_id: null,
      user_id: trade.userId,
      symbol: trade.symbol,
      side: trade.side,
      qty: trade.qty,
      price_vnd: trade.priceVnd,
      ts: trade.ts ?? Date.now(),
    });
    return { changes: 1, last_row_id: 0 };
  }

  /**
   * Fetch the N most recent trades for a user, newest first.
   *
   * @param {number} userId
   * @param {number} limit
   * @returns {Promise<Trade[]>}
   */
  async byUser(userId, limit) {
    await this._ensureIndexes();
    const db = await this._db();
    const docs = await db
      .collection(COLLECTION)
      .find({ user_id: userId })
      .sort({ ts: -1 })
      .limit(limit)
      .toArray();
    return docs.map((d) => ({
      id: d._id,
      userId: d.user_id,
      symbol: d.symbol,
      side: d.side,
      qty: d.qty,
      priceVnd: d.price_vnd,
      ts: d.ts,
    }));
  }

  /** @returns {Promise<number[]>} all distinct user_id values. */
  async distinctUsers() {
    await this._ensureIndexes();
    const db = await this._db();
    return db.collection(COLLECTION).distinct("user_id");
  }

  /**
   * @param {number} userId
   * @param {number} keepN
   * @returns {Promise<import("mongodb").ObjectId[]>} _id values of rows beyond keepN newest for user.
   */
  async oldRowsForUser(userId, keepN) {
    await this._ensureIndexes();
    const db = await this._db();
    const docs = await db
      .collection(COLLECTION)
      .find({ user_id: userId })
      .sort({ ts: -1 })
      .skip(keepN)
      .project({ _id: 1 })
      .toArray();
    return docs.map((d) => d._id);
  }

  /**
   * @param {number} keepN
   * @returns {Promise<import("mongodb").ObjectId[]>} _id values of rows beyond keepN newest globally.
   */
  async oldRows(keepN) {
    await this._ensureIndexes();
    const db = await this._db();
    const docs = await db
      .collection(COLLECTION)
      .find({})
      .sort({ ts: -1 })
      .skip(keepN)
      .project({ _id: 1 })
      .toArray();
    return docs.map((d) => d._id);
  }

  /**
   * @param {import("mongodb").ObjectId[]} ids
   * @returns {Promise<{ deletedCount: number }>}
   */
  async deleteByIds(ids) {
    if (ids.length === 0) return { deletedCount: 0 };
    await this._ensureIndexes();
    const db = await this._db();
    return db.collection(COLLECTION).deleteMany({ _id: { $in: ids } });
  }
}
