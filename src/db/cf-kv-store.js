/**
 * @file CFKVStore — Cloudflare Workers KV implementation of the KVStore interface.
 *
 * Wraps a `KVNamespace` binding. normalizes the list() response shape so the
 * rest of the codebase never sees CF-specific fields like `list_complete`.
 *
 * @see ./kv-store-interface.js for the interface contract.
 */

/**
 * @typedef {import("./kv-store-interface.js").KVStore} KVStore
 * @typedef {import("./kv-store-interface.js").KVStorePutOptions} KVStorePutOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListOptions} KVStoreListOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListResult} KVStoreListResult
 */

/**
 * @implements {KVStore}
 */
export class CFKVStore {
  /**
   * @param {KVNamespace} kvNamespace — bound via wrangler.toml [[kv_namespaces]].
   */
  constructor(kvNamespace) {
    if (!kvNamespace) throw new Error("CFKVStore: kvNamespace is required");
    this.kv = kvNamespace;
  }

  /**
   * @param {string} key
   * @returns {Promise<string|null>}
   */
  async get(key) {
    return this.kv.get(key, { type: "text" });
  }

  /**
   * @param {string} key
   * @param {string} value
   * @param {KVStorePutOptions} [opts]
   * @returns {Promise<void>}
   */
  async put(key, value, opts) {
    // CF KV rejects `{ expirationTtl: undefined }` on some wrangler versions,
    // so only pass the options object when actually needed.
    if (opts?.expirationTtl) {
      await this.kv.put(key, value, { expirationTtl: opts.expirationTtl });
      return;
    }
    await this.kv.put(key, value);
  }

  /**
   * @param {string} key
   * @returns {Promise<void>}
   */
  async delete(key) {
    await this.kv.delete(key);
  }

  /**
   * @param {KVStoreListOptions} [opts]
   * @returns {Promise<KVStoreListResult>}
   */
  async list(opts = {}) {
    const result = await this.kv.list({
      prefix: opts.prefix,
      limit: opts.limit,
      cursor: opts.cursor,
    });
    return {
      keys: result.keys.map((k) => k.name),
      cursor: result.cursor,
      done: result.list_complete === true,
    };
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
      // Corrupt record — do not crash a handler. Log, return null, move on.
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
