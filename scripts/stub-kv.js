/**
 * @file stub-kv — no-op binding stubs used by scripts/register.js.
 *
 * The register script imports buildRegistry() to derive the public command
 * list at deploy time. Module init hooks in this codebase dereference
 * runtime bindings like `env.KV` (via createStore) and `env.AI` (semantle).
 * These stubs satisfy the shape without doing any real IO. All init hooks
 * are assumed read-only (or tolerant of missing state) at registration time.
 * If a future module writes inside init(), update the matching stub to
 * swallow writes safely.
 *
 * `STUB_SENTINEL` is passed as `env.MONGODB_URI` so the create-store /
 * create-sql-store factories short-circuit BEFORE constructing any MongoClient.
 * This ensures zero Atlas connections during `npm run register:dry`.
 *
 * `stubMongo` is a duck-typed no-op that satisfies the MongoClient surface
 * used by MongoKVStore. It is NOT a real MongoClient instance. It must never
 * be used in production — sentinel check in the factories prevents that.
 */

/** @type {KVNamespace} */
export const stubKv = {
  async get() {
    return null;
  },
  async put() {
    // no-op
  },
  async delete() {
    // no-op
  },
  async list() {
    return {
      keys: [],
      list_complete: true,
      cursor: undefined,
    };
  },
  // getWithMetadata is part of the KVNamespace type but unused by CFKVStore
  // — provide a stub so duck-typing doesn't trip.
  async getWithMetadata() {
    return { value: null, metadata: null };
  },
};

/**
 * Workers AI binding stub. Semantle's init() wires `createClient(env.AI)`
 * which only type-checks `.run` — no inference ever fires during register.
 */
export const stubAi = {
  async run() {
    return { data: [] };
  },
};

/**
 * Sentinel value passed as `env.MONGODB_URI` during deploy-time registry
 * builds. Factories check for this value FIRST and return CF-only stores
 * immediately — no MongoClient is ever constructed.
 *
 * @type {string}
 */
export const STUB_SENTINEL = "__stub_mongo__";

/**
 * Duck-typed no-op MongoClient surface. Satisfies the shape used by
 * MongoKVStore / MongoTradesStore without performing any network IO.
 * Used only when `env.MONGODB_URI === STUB_SENTINEL`.
 */
export const stubMongo = {
  db() {
    return {
      collection() {
        throw new Error("stubMongo: no IO at deploy time");
      },
    };
  },
  async connect() {
    return undefined;
  },
  async close() {
    return undefined;
  },
};
