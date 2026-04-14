/**
 * @file Portfolio CRUD — per-user KV read/write and balance operations.
 * All mutations are in-memory; caller must savePortfolio() to persist.
 */

import { getSymbol } from "./symbols.js";

/**
 * @typedef {Object} Portfolio
 * @property {{ [currency: string]: number }} currency
 * @property {{ [symbol: string]: number }} stock
 * @property {{ [symbol: string]: number }} crypto
 * @property {{ [symbol: string]: number }} others
 * @property {number} totalvnd
 */

/** @returns {Portfolio} */
export function emptyPortfolio() {
  return { currency: { VND: 0, USD: 0 }, stock: {}, crypto: {}, others: {}, totalvnd: 0 };
}

/**
 * Load user portfolio from KV, or return empty if first-time user.
 * Ensures all category keys exist (migration-safe).
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} userId
 * @returns {Promise<Portfolio>}
 */
export async function getPortfolio(db, userId) {
  const raw = await db.getJSON(`user:${userId}`);
  if (!raw) return emptyPortfolio();
  // ensure all expected keys exist
  const p = emptyPortfolio();
  p.currency = { ...p.currency, ...raw.currency };
  p.stock = { ...raw.stock };
  p.crypto = { ...raw.crypto };
  p.others = { ...raw.others };
  p.totalvnd = raw.totalvnd ?? 0;
  return p;
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
 * Add asset (stock/crypto/others) to portfolio.
 * @param {Portfolio} p
 * @param {string} symbol
 * @param {number} qty
 */
export function addAsset(p, symbol, qty) {
  const info = getSymbol(symbol);
  if (!info) return;
  const cat = info.category;
  p[cat][symbol] = (p[cat][symbol] || 0) + qty;
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
  const info = getSymbol(symbol);
  if (!info) return { ok: false, held: 0 };
  const cat = info.category;
  const held = p[cat][symbol] || 0;
  if (held < qty) return { ok: false, held };
  const remaining = held - qty;
  if (remaining === 0) delete p[cat][symbol];
  else p[cat][symbol] = remaining;
  return { ok: true, held: remaining };
}
