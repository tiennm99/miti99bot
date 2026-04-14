/**
 * @file Symbol resolution — dynamically resolves stock tickers via TCBS API.
 * Resolved symbols are cached in KV permanently to avoid repeated lookups.
 * Currently only supports VN stocks. Crypto, gold, forex coming later.
 */

const COMING_SOON = "Crypto, gold & currency exchange coming soon!";

/**
 * @typedef {Object} ResolvedSymbol
 * @property {string} symbol — uppercase ticker
 * @property {string} category — "stock" (only supported category for now)
 * @property {string} label — company name
 */

/**
 * Resolve a ticker to a symbol entry. Checks KV cache first, then queries TCBS.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {string} ticker — user input, case-insensitive
 * @returns {Promise<ResolvedSymbol|null>} null if not found on TCBS
 */
export async function resolveSymbol(db, ticker) {
  if (!ticker) return null;
  const symbol = ticker.toUpperCase();
  const cacheKey = `sym:${symbol}`;

  // check KV cache
  const cached = await db.getJSON(cacheKey);
  if (cached) return cached;

  // query TCBS to verify this is a real VN stock
  const to = Math.floor(Date.now() / 1000);
  const url = `https://apipubaws.tcbs.com.vn/stock-insight/v1/stock/bars-long-term?ticker=${symbol}&type=stock&resolution=D&countBack=1&to=${to}`;
  const res = await fetch(url);
  if (!res.ok) return null;
  const json = await res.json();
  const close = json?.data?.[0]?.close;
  if (close == null) return null;

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
