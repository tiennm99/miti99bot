/**
 * @file stub-kv — no-op binding stubs used by scripts/register.js.
 *
 * The register script imports buildRegistry() to derive the public command
 * list at deploy time. Module init hooks in this codebase dereference
 * runtime bindings like `env.KV` (via createStore) and `env.AI` (semantle).
 * These stubs satisfy the shape without doing any real IO. All init hooks
 * are assumed read-only (or tolerant of missing state) at registration time.
 * If a future module writes inside init(), update the matching stub to
 * swallow writes or gate the write on a `process.env.REGISTER_DRYRUN` flag.
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
