/**
 * @file Subscriber list persisted in KV under the module's `subscribers` key.
 *
 * Stored as a plain JSON array of chat ids (numbers). Kept intentionally small
 * and dependency-free — idempotent add/remove, returns a boolean indicating
 * whether the list changed.
 */

const KEY = "subscribers";

/** @param {import("../../db/kv-store-interface.js").KVStore} db */
export async function listSubscribers(db) {
  const raw = await db.getJSON(KEY);
  return Array.isArray(raw) ? raw : [];
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} chatId
 * @returns {Promise<boolean>} true if added, false if already subscribed
 */
export async function addSubscriber(db, chatId) {
  const ids = await listSubscribers(db);
  if (ids.includes(chatId)) return false;
  ids.push(chatId);
  await db.putJSON(KEY, ids);
  return true;
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} chatId
 * @returns {Promise<boolean>} true if removed, false if not subscribed
 */
export async function removeSubscriber(db, chatId) {
  const ids = await listSubscribers(db);
  const next = ids.filter((id) => id !== chatId);
  if (next.length === ids.length) return false;
  await db.putJSON(KEY, next);
  return true;
}
