/**
 * @file mongo-list-cursor — cursor-based list pagination helper for MongoKVStore.
 *
 * Extracted to keep mongo-kv-store.js under 200 LOC.
 * Encodes the last `_id` of a page as a base64 cursor; decodes it on the
 * next call to build a `$gt` filter. Does NOT use skip() — purely sorted-_id
 * pagination to avoid O(n) offset scans.
 *
 * @module db/mongo-list-cursor
 */

/**
 * @typedef {import("./kv-store-interface.js").KVStoreListOptions} KVStoreListOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListResult} KVStoreListResult
 */

/**
 * Escape special RegExp characters so a prefix string is safe inside a
 * MongoDB `$regex` filter.
 *
 * @param {string} str
 * @returns {string}
 */
export function escapeRegex(str) {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Execute a paginated list query on a MongoDB collection.
 *
 * @param {import("mongodb").Collection} col — resolved collection handle.
 * @param {KVStoreListOptions} opts
 * @returns {Promise<KVStoreListResult>}
 */
export async function listWithCursor(col, opts = {}) {
  const { prefix = "", limit = 1000, cursor } = opts;
  const pageSize = limit;

  const filter = {};
  if (prefix) {
    filter._id = { $regex: `^${escapeRegex(prefix)}` };
  }
  if (cursor) {
    const lastId = Buffer.from(cursor, "base64").toString("utf8");
    filter._id = filter._id ? { ...filter._id, $gt: lastId } : { $gt: lastId };
  }

  const docs = await col
    .find(filter)
    .sort({ _id: 1 })
    .limit(pageSize + 1)
    .project({ _id: 1 })
    .toArray();

  const hasMore = docs.length > pageSize;
  const page = hasMore ? docs.slice(0, pageSize) : docs;
  const keys = page.map((d) => d._id);
  const nextCursor = hasMore
    ? Buffer.from(page[page.length - 1]._id, "utf8").toString("base64")
    : undefined;

  return { keys, cursor: nextCursor, done: !hasMore };
}
