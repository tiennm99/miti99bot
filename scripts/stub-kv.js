/**
 * @file stub-kv — minimal no-op KVNamespace stub used by scripts/register.js.
 *
 * The register script imports buildRegistry() to derive the public command
 * list at deploy time. buildRegistry calls createStore() → CFKVStore → needs
 * a KV binding. This stub satisfies the shape without doing any real IO,
 * since module init hooks in this codebase read-only (or tolerate missing
 * state). If a future module writes inside init(), update the stub to
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
