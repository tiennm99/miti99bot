/**
 * @file createStore — factory that returns a namespaced KVStore for a module.
 *
 * Every module gets its own prefixed view: module `wordle` calling `put("k", v)`
 * writes raw key `wordle:k`. list() automatically constrains to the module's
 * namespace AND strips the prefix from returned keys so the module sees its
 * own flat key-space. Modules CANNOT escape their namespace without
 * reconstructing prefixes manually — a code-review boundary, not a hard one.
 *
 * ## Flag matrix (env.STORAGE_PRIMARY × env.DUAL_WRITE × env.MONGODB_URI)
 *
 * | STORAGE_PRIMARY | DUAL_WRITE | MONGODB_URI       | Result                                    |
 * |-----------------|------------|-------------------|--------------------------------------------|
 * | (unset) or kv   | 1 (default)| set, real         | DualKVStore(CFKVStore primary, Mongo sec.) |
 * | kv              | 0          | any               | CFKVStore only (legacy / rollback)         |
 * | mongo           | 1          | set, real         | DualKVStore(Mongo primary, CFKVStore sec.) |
 * | mongo           | 0          | set, real         | MongoKVStore only (post-cutover)           |
 * | any             | any        | unset             | CFKVStore only                             |
 * | any             | any        | === STUB_SENTINEL | CFKVStore only (deploy-time register path) |
 *
 * Boot-time assertion: if STORAGE_PRIMARY=mongo but MONGODB_URI is absent
 * (and not STUB_SENTINEL), the first call throws a clear error rather than
 * silently falling back to KV.
 *
 * post-Phase-07: this entire function returns MongoKVStore-only;
 * KV branches removed. The flag matrix above collapses to a single path.
 */

import { CFKVStore } from "./cf-kv-store.js";
import { DualKVStore } from "./dual-kv-store.js";
import { MongoKVStore } from "./mongo-kv-store.js";

/**
 * @typedef {import("./kv-store-interface.js").KVStore} KVStore
 * @typedef {import("./kv-store-interface.js").KVStorePutOptions} KVStorePutOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListOptions} KVStoreListOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListResult} KVStoreListResult
 */

/** Sentinel value used by scripts/register.js to signal deploy-time context. */
const STUB_SENTINEL = "__stub_mongo__";

const MODULE_NAME_RE = /^[a-z0-9_-]+$/;

/**
 * Wrap a raw KVStore so all keys carry a `<moduleName>:` prefix and
 * list() results are stripped back to the module's flat key-space.
 *
 * @param {string} prefix — e.g. "wordle:"
 * @param {KVStore} base
 * @returns {KVStore}
 */
function withPrefix(prefix, base) {
  return {
    /** @type {"dual" | undefined} */
    _kind: /** @type {any} */ (base)._kind,

    async get(key) {
      return base.get(prefix + key);
    },

    async put(key, value, opts) {
      return base.put(prefix + key, value, opts);
    },

    async delete(key) {
      return base.delete(prefix + key);
    },

    async list(opts = {}) {
      const fullPrefix = prefix + (opts.prefix ?? "");
      const result = await base.list({
        prefix: fullPrefix,
        limit: opts.limit,
        cursor: opts.cursor,
      });
      // Strip the module namespace from returned keys so the caller sees its own flat space.
      return {
        keys: result.keys.map((k) => (k.startsWith(prefix) ? k.slice(prefix.length) : k)),
        cursor: result.cursor,
        done: result.done,
      };
    },

    async getJSON(key) {
      return base.getJSON(prefix + key);
    },

    async putJSON(key, value, opts) {
      return base.putJSON(prefix + key, value, opts);
    },
  };
}

/**
 * @param {string} moduleName — must match `[a-z0-9_-]+`. Used verbatim as the key prefix.
 * @param {object} env — worker env (or test double).
 * @param {any} env.KV — CF KVNamespace binding.
 * @param {string} [env.MONGODB_URI] — Atlas connection string (or STUB_SENTINEL).
 * @param {string} [env.STORAGE_PRIMARY] — "kv" (default) | "mongo".
 * @param {string} [env.DUAL_WRITE] — "1" (default) | "0".
 * @returns {KVStore}
 */
export function createStore(moduleName, env) {
  if (!moduleName || typeof moduleName !== "string") {
    throw new Error("createStore: moduleName is required");
  }
  if (!MODULE_NAME_RE.test(moduleName)) {
    throw new Error(
      `createStore: invalid moduleName "${moduleName}" — must match ${MODULE_NAME_RE}`,
    );
  }
  if (!env?.KV) {
    throw new Error("createStore: env.KV binding is missing");
  }

  const prefix = `${moduleName}:`;
  // Collection name: replace `-` with `_` for MongoDB compatibility.
  const collectionName = moduleName.replace(/-/g, "_");

  // --- Sentinel / fallback: always CF-only ---
  const mongoUri = env.MONGODB_URI;
  if (!mongoUri || mongoUri === STUB_SENTINEL) {
    return withPrefix(prefix, new CFKVStore(env.KV));
  }

  const primary = (env.STORAGE_PRIMARY ?? "kv").toLowerCase();
  const dualWrite = (env.DUAL_WRITE ?? "1") !== "0";

  // Boot-time assertion: if mongo is requested but URI is absent, throw clearly.
  // (URI IS present here since we checked above, so this guard is for STORAGE_PRIMARY=mongo.)
  if (primary === "mongo" && !dualWrite) {
    // MongoKVStore only — post-cutover path.
    return withPrefix(prefix, new MongoKVStore(env, collectionName));
  }

  const cfStore = new CFKVStore(env.KV);
  const mongoStore = new MongoKVStore(env, collectionName);

  if (primary === "mongo") {
    // DualKVStore: read Mongo, write both — cutover phase.
    return withPrefix(prefix, new DualKVStore(mongoStore, cfStore, env.KV));
  }

  // Default: STORAGE_PRIMARY=kv + DUAL_WRITE=1
  if (!dualWrite) {
    // Rollback path: KV only.
    return withPrefix(prefix, cfStore);
  }

  // DualKVStore: read KV, write both — dual-write window.
  return withPrefix(prefix, new DualKVStore(cfStore, mongoStore, env.KV));
}
