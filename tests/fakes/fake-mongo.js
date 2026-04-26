/**
 * @file fake-mongo — Map-backed in-memory MongoDB fake for unit tests.
 *
 * **FROZEN SURFACE** (Phase 02–08): This fake implements the minimal subset of MongoDB
 * API required by MongoKVStore and MongoTradesStore tests. New MongoClient features
 * must be added to this fake to remain testable.
 *
 * **Implemented methods:**
 * - `db.collection(name)` → Collection
 * - Collection: `findOne(query)`, `updateOne(filter, update, opts)`, `deleteOne(query)`,
 *   `find(query)`, `insertOne(doc)`, `insertMany(docs)`, `distinct(field, query)`,
 *   `deleteMany(query)`, `countDocuments(query)`, `createIndex(spec)` (no-op)
 *
 * **Cursor methods** (from `find()`): `sort(spec)`, `skip(n)`, `limit(n)`,
 * `project(spec)`, `toArray()`
 *
 * **NOT SIMULATED:**
 * - TTL expirations at server-side. Tests assert `expiresAt` field presence and
 *   control Date.now() via `vi.setSystemTime()` to test read-time expiration filters
 *   in MongoKVStore. Real Atlas TTL is validated during Phase 06 soak.
 * - Transactions, sharding, replica sets, or other cluster features.
 * - Advanced operators beyond those in `matchQuery()` ($gt, $gte, $lt, $lte,
 *   $exists, $in, $regex, $or, $and).
 *
 * @see tests/db/mongo-kv-store.test.js
 * @see tests/modules/trading/ (trades store usage)
 */

/**
 * Apply $set and $unset from an update document to a target object.
 *
 * @param {object} doc
 * @param {object} update
 * @returns {object}
 */
function applyUpdate(doc, update) {
  const result = { ...doc };
  if (update.$set) {
    for (const [k, v] of Object.entries(update.$set)) {
      result[k] = v;
    }
  }
  if (update.$unset) {
    for (const key of Object.keys(update.$unset)) {
      delete result[key];
    }
  }
  return result;
}

/**
 * Minimal regex-query matcher. Supports:
 *   { field: value }          — strict equality
 *   { field: { $gt: v } }     — greater-than
 *   { field: { $exists: b } } — field existence
 *   { $or: [cond, ...] }      — logical OR
 *   { $and: [cond, ...] }     — logical AND
 *
 * @param {object} doc
 * @param {object} query
 * @returns {boolean}
 */
function matchQuery(doc, query) {
  for (const [key, condition] of Object.entries(query)) {
    if (key === "$or") {
      if (!condition.some((sub) => matchQuery(doc, sub))) return false;
      continue;
    }
    if (key === "$and") {
      if (!condition.every((sub) => matchQuery(doc, sub))) return false;
      continue;
    }
    if (condition !== null && typeof condition === "object" && !Array.isArray(condition)) {
      const ops = Object.keys(condition);
      if (ops.some((op) => op.startsWith("$"))) {
        for (const [op, operand] of Object.entries(condition)) {
          if (op === "$gt") {
            if (!(doc[key] > operand)) return false;
          } else if (op === "$gte") {
            if (!(doc[key] >= operand)) return false;
          } else if (op === "$lt") {
            if (!(doc[key] < operand)) return false;
          } else if (op === "$lte") {
            if (!(doc[key] <= operand)) return false;
          } else if (op === "$exists") {
            const has = key in doc && doc[key] !== undefined;
            if (operand !== has) return false;
          } else if (op === "$in") {
            if (!operand.includes(doc[key])) return false;
          } else if (op === "$regex") {
            const flags = condition.$options ?? "";
            if (!new RegExp(operand, flags).test(doc[key])) return false;
          }
        }
        continue;
      }
    }
    if (doc[key] !== condition) return false;
  }
  return true;
}

/**
 * Returns a chainable cursor builder from an array of matched docs.
 *
 * @param {object[]} docs
 * @returns {object} chainable cursor with sort/skip/limit/project/toArray
 */
function makeCursor(docs) {
  const items = [...docs];
  let sortField = null;
  let sortDir = 1;
  let skipN = 0;
  let limitN = Number.POSITIVE_INFINITY;
  let projection = null;

  const cursor = {
    sort(spec) {
      const entries = Object.entries(spec);
      if (entries.length > 0) {
        [sortField, sortDir] = [entries[0][0], entries[0][1]];
      }
      return cursor;
    },
    skip(n) {
      skipN = n;
      return cursor;
    },
    limit(n) {
      limitN = n;
      return cursor;
    },
    project(spec) {
      projection = spec;
      return cursor;
    },
    async toArray() {
      let result = [...items];
      if (sortField !== null) {
        result.sort((a, b) => {
          if (a[sortField] < b[sortField]) return -sortDir;
          if (a[sortField] > b[sortField]) return sortDir;
          return 0;
        });
      }
      result = result.slice(
        skipN,
        limitN === Number.POSITIVE_INFINITY ? undefined : skipN + limitN,
      );
      if (projection) {
        result = result.map((doc) => {
          const out = {};
          for (const [k, include] of Object.entries(projection)) {
            if (include) out[k] = doc[k];
          }
          return out;
        });
      }
      return result;
    },
  };
  return cursor;
}

/**
 * Create a fake MongoDB collection backed by a Map.
 *
 * @param {Map<string, object>} store
 * @returns {object}
 */
function makeCollection(store) {
  return {
    /** @returns {Promise<object|null>} */
    async findOne(query) {
      for (const doc of store.values()) {
        if (matchQuery(doc, query)) return { ...doc };
      }
      return null;
    },

    /**
     * Supports upsert with $set / $unset.
     * @returns {Promise<{matchedCount: number, upsertedCount: number, modifiedCount: number}>}
     */
    async updateOne(filter, update, opts = {}) {
      for (const [id, doc] of store.entries()) {
        if (matchQuery(doc, filter)) {
          store.set(id, applyUpdate(doc, update));
          return { matchedCount: 1, upsertedCount: 0, modifiedCount: 1 };
        }
      }
      if (opts.upsert) {
        // Build new doc from filter equality fields + $set fields
        const newDoc = {};
        for (const [k, v] of Object.entries(filter)) {
          if (typeof v !== "object") newDoc[k] = v;
        }
        const merged = applyUpdate(newDoc, update);
        const id = merged._id ?? String(Date.now() + Math.random());
        merged._id = id;
        store.set(String(id), merged);
        return { matchedCount: 0, upsertedCount: 1, modifiedCount: 0 };
      }
      return { matchedCount: 0, upsertedCount: 0, modifiedCount: 0 };
    },

    /** @returns {Promise<{deletedCount: number}>} */
    async deleteOne(filter) {
      for (const [id, doc] of store.entries()) {
        if (matchQuery(doc, filter)) {
          store.delete(id);
          return { deletedCount: 1 };
        }
      }
      return { deletedCount: 0 };
    },

    /**
     * Returns a chainable cursor.
     * @param {object} [query]
     * @returns {object}
     */
    find(query = {}) {
      const matched = [...store.values()].filter((doc) => matchQuery(doc, query));
      return makeCursor(matched);
    },

    /** @returns {Promise<{insertedId: string}>} */
    async insertOne(doc) {
      const id = doc._id ?? String(Date.now() + Math.random());
      store.set(String(id), { ...doc, _id: id });
      return { insertedId: id };
    },

    /** @returns {Promise<{insertedIds: string[]}>} */
    async insertMany(docs) {
      const insertedIds = [];
      for (const doc of docs) {
        const id = doc._id ?? String(Date.now() + Math.random());
        store.set(String(id), { ...doc, _id: id });
        insertedIds.push(id);
      }
      return { insertedIds };
    },

    /** @returns {Promise<any[]>} */
    async distinct(field, query = {}) {
      const values = new Set();
      for (const doc of store.values()) {
        if (matchQuery(doc, query) && field in doc) {
          values.add(doc[field]);
        }
      }
      return [...values];
    },

    /** @returns {Promise<{deletedCount: number}>} */
    async deleteMany(filter = {}) {
      let count = 0;
      for (const [id, doc] of store.entries()) {
        if (matchQuery(doc, filter)) {
          store.delete(id);
          count++;
        }
      }
      return { deletedCount: count };
    },

    /** @returns {Promise<number>} */
    async countDocuments(query = {}) {
      let count = 0;
      for (const doc of store.values()) {
        if (matchQuery(doc, query)) count++;
      }
      return count;
    },

    /** No-op — index creation is idempotent; tests only verify field presence. */
    async createIndex(_spec, _opts) {
      return "ok";
    },
  };
}

/**
 * Create a fake MongoDB Db object.
 * Each collection is lazily created and backed by its own Map.
 *
 * @returns {{ collection: (name: string) => object, _stores: Map<string, Map<string, object>> }}
 */
export function makeFakeMongo() {
  /** @type {Map<string, Map<string, object>>} */
  const stores = new Map();

  return {
    /** @param {string} name */
    collection(name) {
      if (!stores.has(name)) {
        stores.set(name, new Map());
      }
      return makeCollection(stores.get(name));
    },
    /** Expose raw stores so tests can inspect or seed data directly. */
    _stores: stores,
  };
}
