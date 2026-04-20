/**
 * @file Price fetching — TCBS (VN stocks) + BIDV (forex).
 * Single-stock price fetch on demand. Forex rates cached for 60s.
 */

const FOREX_CACHE_KEY = "forex:latest";
const CACHE_TTL_MS = 60_000;
const STALE_LIMIT_MS = 300_000;

/**
 * Fetch current VND price for a VN stock ticker via TCBS.
 * Returns close price * 1000 (TCBS convention).
 * @param {string} ticker — uppercase, e.g. "TCB"
 * @returns {Promise<number|null>}
 */
export async function fetchStockPrice(ticker) {
  const to = Math.floor(Date.now() / 1000);
  const url = `https://apipubaws.tcbs.com.vn/stock-insight/v1/stock/bars-long-term?ticker=${encodeURIComponent(ticker)}&type=stock&resolution=D&countBack=1&to=${to}`;
  const res = await fetch(url);
  if (!res.ok) return null;
  const json = await res.json();
  const close = json?.data?.[0]?.close;
  if (close == null) return null;
  return close * 1000;
}

/** Forex rates via BIDV public API — returns real buy/sell rates */
async function fetchForex() {
  const res = await fetch("https://www.bidv.com.vn/ServicesBIDV/ExchangeDetailServlet");
  if (!res.ok) throw new Error(`BIDV API ${res.status}`);
  const json = await res.json();
  const usd = json?.data?.find((r) => r.currency === "USD");
  if (!usd) throw new Error("BIDV missing USD rate");
  const parse = (s) => Number.parseFloat(String(s).replace(/,/g, ""));
  const buy = parse(usd.muaCk);
  const sell = parse(usd.ban);
  if (!Number.isFinite(buy) || !Number.isFinite(sell)) {
    throw new Error("BIDV invalid USD rate");
  }
  return { USD: { mid: (buy + sell) / 2, buy, sell } };
}

/**
 * Get VND price for a stock symbol.
 * @param {string} symbol — uppercase ticker
 * @returns {Promise<number|null>}
 */
export async function getStockPrice(symbol) {
  return fetchStockPrice(symbol);
}

/**
 * Cache-first forex rate retrieval.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
async function getForexRates(db) {
  const cached = await db.getJSON(FOREX_CACHE_KEY);
  if (cached?.ts && Date.now() - cached.ts < CACHE_TTL_MS) return cached;
  try {
    const forex = await fetchForex();
    const data = { ts: Date.now(), ...forex };
    try {
      await db.putJSON(FOREX_CACHE_KEY, data);
    } catch {
      /* best effort */
    }
    return data;
  } catch {
    if (cached?.ts && Date.now() - cached.ts < STALE_LIMIT_MS) return cached;
    throw new Error("Could not fetch forex rates. Try again later.");
  }
}

/**
 * Mid-rate VND equivalent of 1 unit of currency (for stats display).
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {string} currency
 * @returns {Promise<number|null>}
 */
export async function getForexRate(db, currency) {
  if (currency === "VND") return 1;
  const rates = await getForexRates(db);
  return rates[currency]?.mid ?? null;
}

/**
 * Buy/sell forex rates for a currency.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {string} currency
 * @returns {Promise<{ buy: number, sell: number }|null>}
 */
export async function getForexBidAsk(db, currency) {
  if (currency === "VND") return null;
  const rates = await getForexRates(db);
  const rate = rates[currency];
  if (!rate?.buy || !rate?.sell) return null;
  return { buy: rate.buy, sell: rate.sell };
}
