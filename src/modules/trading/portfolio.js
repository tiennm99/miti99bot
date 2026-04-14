/**
 * @file Portfolio CRUD — per-user KV read/write and balance operations.
 * All mutations are in-memory; caller must savePortfolio() to persist.
 *
 * Schema: { currency: { VND, USD }, assets: { SYMBOL: qty }, totalvnd }
 * Assets are stored in a flat map — category is derived from symbol resolution.
 */

/**
 * @typedef {Object} Portfolio
 * @property {{ [currency: string]: number }} currency
 * @property {{ [symbol: string]: number }} assets
 * @property {number} totalvnd
 */

/** @returns {Portfolio} */
export function emptyPortfolio() {
  return { currency: { VND: 0 }, assets: {}, totalvnd: 0 };
}

/**
 * Load user portfolio from KV, or return empty if first-time user.
 * Migrates old 4-category format to flat assets map.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} userId
 * @returns {Promise<Portfolio>}
 */
export async function getPortfolio(db, userId) {
  const raw = await db.getJSON(`user:${userId}`);
  if (!raw) return emptyPortfolio();

  // migrate old format: merge stock/crypto/others into flat assets
  if (raw.stock || raw.crypto || raw.others) {
    const assets = { ...raw.stock, ...raw.crypto, ...raw.others, ...raw.assets };
    return {
      currency: { VND: 0, ...raw.currency },
      assets,
      totalvnd: raw.totalvnd ?? 0,
    };
  }

  return {
    currency: { VND: 0, ...raw.currency },
    assets: raw.assets ?? {},
    totalvnd: raw.totalvnd ?? 0,
  };
}

/**
 * Persist portfolio to KV.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} userId
 * @param {Portfolio} portfolio
 */
export async function savePortfolio(db, userId, portfolio) {
  await db.putJSON(`user:${userId}`, portfolio);
}

/**
 * Add fiat to portfolio. Mutates in place.
 * @param {Portfolio} p
 * @param {string} currency
 * @param {number} amount
 */
export function addCurrency(p, currency, amount) {
  p.currency[currency] = (p.currency[currency] || 0) + amount;
}

/**
 * Deduct fiat. Returns { ok, balance } — ok=false if insufficient.
 * @param {Portfolio} p
 * @param {string} currency
 * @param {number} amount
 * @returns {{ ok: boolean, balance: number }}
 */
export function deductCurrency(p, currency, amount) {
  const balance = p.currency[currency] || 0;
  if (balance < amount) return { ok: false, balance };
  p.currency[currency] = balance - amount;
  return { ok: true, balance: balance - amount };
}

/**
 * Add asset to flat assets map.
 * @param {Portfolio} p
 * @param {string} symbol
 * @param {number} qty
 */
export function addAsset(p, symbol, qty) {
  p.assets[symbol] = (p.assets[symbol] || 0) + qty;
}

/**
 * Deduct asset. Returns { ok, held } — ok=false if insufficient.
 * Removes key if balance reaches 0.
 * @param {Portfolio} p
 * @param {string} symbol
 * @param {number} qty
 * @returns {{ ok: boolean, held: number }}
 */
export function deductAsset(p, symbol, qty) {
  const held = p.assets[symbol] || 0;
  if (held < qty) return { ok: false, held };
  const remaining = held - qty;
  if (remaining === 0) delete p.assets[symbol];
  else p.assets[symbol] = remaining;
  return { ok: true, held: remaining };
}
