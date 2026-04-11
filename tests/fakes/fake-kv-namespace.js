/**
 * @file fake-kv-namespace — in-memory `Map`-backed KVNamespace for tests.
 *
 * Matches the real CF KVNamespace shape that `CFKVStore` depends on:
 *   get(key, {type: 'text'}) → string | null
 *   put(key, value, opts?)   → stores, records opts for assertion
 *   delete(key)              → removes
 *   list({prefix, limit, cursor}) → { keys: [{name}], list_complete, cursor }
 *
 * `listLimit` is applied BEFORE `list_complete` is computed so tests can
 * exercise pagination.
 */

export function makeFakeKv() {
  /** @type {Map<string, string>} */
  const store = new Map();
  /** @type {Array<{key: string, value: string, opts?: any}>} */
  const putLog = [];

  return {
    store,
    putLog,
    async get(key, _opts) {
      return store.has(key) ? store.get(key) : null;
    },
    async put(key, value, opts) {
      store.set(key, value);
      putLog.push({ key, value, opts });
    },
    async delete(key) {
      store.delete(key);
    },
    async list({ prefix = "", limit = 1000, cursor } = {}) {
      const allKeys = [...store.keys()].filter((k) => k.startsWith(prefix)).sort();
      const start = cursor ? Number.parseInt(cursor, 10) : 0;
      const slice = allKeys.slice(start, start + limit);
      const nextStart = start + slice.length;
      const complete = nextStart >= allKeys.length;
      return {
        keys: slice.map((name) => ({ name })),
        list_complete: complete,
        cursor: complete ? undefined : String(nextStart),
      };
    },
  };
}
