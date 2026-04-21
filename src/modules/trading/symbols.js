/**
 * @file Symbol resolution — dynamically resolves stock tickers via KBS.
 * Resolved symbols are cached in KV permanently to avoid repeated lookups.
 * Currently only supports VN stocks. Crypto, gold, forex coming later.
 */

import { fetchStockPrice } from "./prices.js";

const COMING_SOON = "Crypto, gold & currency exchange coming soon!";

/**
 * @typedef {object} ResolvedSymbol
 * @property {string} symbol — uppercase ticker
 * @property {string} category — "stock" (only supported category for now)
 * @property {string} label — company name
 */

/**
 * Resolve a ticker to a symbol entry. Checks KV cache first, then queries KBS.
 * Validation reuses the KBS price endpoint — if it returns a bar, the ticker is real.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {string} ticker — user input, case-insensitive
 * @returns {Promise<ResolvedSymbol|null>} null if KBS has no data for this ticker
 */
export async function resolveSymbol(db, ticker) {
  if (!ticker) return null;
  const symbol = ticker.toUpperCase();
  const cacheKey = `sym:${symbol}`;

  const cached = await db.getJSON(cacheKey);
  if (cached) return cached;

  const price = await fetchStockPrice(symbol);
  if (price == null) return null;

  const entry = { symbol, category: "stock", label: symbol };
  // cache permanently — stock tickers don't change
  await db.putJSON(cacheKey, entry);
  return entry;
}

/**
 * Error message for unsupported asset types.
 * @returns {string}
 */
export function comingSoonMessage() {
  return COMING_SOON;
}
