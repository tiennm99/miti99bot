/**
 * @file createStore — factory that returns a namespaced KVStore for a module.
 *
 * Every module gets its own prefixed view: module `wordle` calling `put("k", v)`
 * writes raw key `wordle:k`. list() automatically constrains to the module's
 * namespace AND strips the prefix from returned keys so the module sees its
 * own flat key-space. modules CANNOT escape their namespace without
 * reconstructing prefixes manually — a code-review boundary, not a hard one.
 */

import { CFKVStore } from "./cf-kv-store.js";

/**
 * @typedef {import("./kv-store-interface.js").KVStore} KVStore
 * @typedef {import("./kv-store-interface.js").KVStorePutOptions} KVStorePutOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListOptions} KVStoreListOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListResult} KVStoreListResult
 */

const MODULE_NAME_RE = /^[a-z0-9_-]+$/;

/**
 * @param {string} moduleName — must match `[a-z0-9_-]+`. Used verbatim as the key prefix.
 * @param {{ KV: KVNamespace }} env — worker env (or test double) with a `KV` binding.
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

  const base = new CFKVStore(env.KV);
  const prefix = `${moduleName}:`;

  return {
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
