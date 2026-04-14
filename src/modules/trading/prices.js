/**
 * @file Price fetching — CoinGecko (crypto+gold), TCBS (VN stocks), ER-API (forex).
 * Caches merged result in KV for 60s to avoid API spam.
 */

import { SYMBOLS } from "./symbols.js";

const CACHE_KEY = "prices:latest";
const CACHE_TTL_MS = 60_000;
const STALE_LIMIT_MS = 300_000; // 5 min — max age for fallback

/** Crypto + gold via CoinGecko free API */
async function fetchCrypto() {
  const ids = Object.values(SYMBOLS)
    .filter((s) => s.category === "crypto" || s.category === "others")
    .map((s) => s.apiId)
    .join(",");
  const res = await fetch(
    `https://api.coingecko.com/api/v3/simple/price?ids=${ids}&vs_currencies=vnd`,
  );
  if (!res.ok) throw new Error(`CoinGecko ${res.status}`);
  const data = await res.json();
  const crypto = {};
  const others = {};
  for (const [sym, entry] of Object.entries(SYMBOLS)) {
    if (entry.category !== "crypto" && entry.category !== "others") continue;
    const price = data[entry.apiId]?.vnd;
    if (price == null) continue;
    if (entry.category === "crypto") crypto[sym] = price;
    else others[sym] = price;
  }
  return { crypto, others };
}

/** Vietnam stock prices via TCBS public API (price in VND * 1000) */
async function fetchStocks() {
  const tickers = Object.entries(SYMBOLS).filter(([, e]) => e.category === "stock");
  const to = Math.floor(Date.now() / 1000);
  const results = await Promise.allSettled(
    tickers.map(async ([sym, entry]) => {
      const url = `https://apipubaws.tcbs.com.vn/stock-insight/v1/stock/bars-long-term?ticker=${entry.apiId}&type=stock&resolution=D&countBack=1&to=${to}`;
      const res = await fetch(url);
      if (!res.ok) throw new Error(`TCBS ${entry.apiId} ${res.status}`);
      const json = await res.json();
      const close = json?.data?.[0]?.close;
      if (close == null) throw new Error(`TCBS ${entry.apiId} no close`);
      return { sym, price: close * 1000 };
    }),
  );
  const stock = {};
  for (const r of results) {
    if (r.status === "fulfilled") stock[r.value.sym] = r.value.price;
  }
  return stock;
}

/** Forex rates via BIDV public API — returns real buy/sell rates */
async function fetchForex() {
  const res = await fetch("https://www.bidv.com.vn/ServicesBIDV/ExchangeDetailServlet");
  if (!res.ok) throw new Error(`BIDV API ${res.status}`);
  const json = await res.json();
  const usd = json?.data?.find((r) => r.currency === "USD");
  if (!usd) throw new Error("BIDV missing USD rate");
  // muaCk = bank buy (transfer), ban = bank sell — parse "26,141" → 26141
  const parse = (s) => Number.parseFloat(String(s).replace(/,/g, ""));
  const buy = parse(usd.muaCk);
  const sell = parse(usd.ban);
  if (!Number.isFinite(buy) || !Number.isFinite(sell)) {
    throw new Error("BIDV invalid USD rate");
  }
  return { USD: { mid: (buy + sell) / 2, buy, sell } };
}

/**
 * Fetch all prices in parallel, merge, cache in KV.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function fetchPrices(db) {
  const [cryptoRes, stockRes, forexRes] = await Promise.allSettled([
    fetchCrypto(),
    fetchStocks(),
    fetchForex(),
  ]);
  const merged = {
    ts: Date.now(),
    crypto: cryptoRes.status === "fulfilled" ? cryptoRes.value.crypto : {},
    stock: stockRes.status === "fulfilled" ? stockRes.value : {},
    forex: forexRes.status === "fulfilled" ? forexRes.value : {},
    others: cryptoRes.status === "fulfilled" ? cryptoRes.value.others : {},
  };
  try {
    await db.putJSON(CACHE_KEY, merged);
  } catch {
    /* best effort cache write */
  }
  return merged;
}

/**
 * Cache-first price retrieval. Returns cached if < 60s old, else fetches fresh.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function getPrices(db) {
  const cached = await db.getJSON(CACHE_KEY);
  if (cached?.ts && Date.now() - cached.ts < CACHE_TTL_MS) return cached;
  try {
    return await fetchPrices(db);
  } catch {
    // fallback to stale cache if < 5 min
    if (cached?.ts && Date.now() - cached.ts < STALE_LIMIT_MS) return cached;
    throw new Error("Could not fetch prices. Try again later.");
  }
}

/**
 * Get VND price for a single symbol.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {string} symbol — uppercase, e.g. "BTC"
 * @returns {Promise<number|null>}
 */
export async function getPrice(db, symbol) {
  const info = SYMBOLS[symbol];
  if (!info) return null;
  const prices = await getPrices(db);
  return prices[info.category]?.[symbol] ?? null;
}

/**
 * Mid-rate VND equivalent of 1 unit of currency (for stats display).
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {string} currency — "VND" or "USD"
 * @returns {Promise<number|null>}
 */
export async function getForexRate(db, currency) {
  if (currency === "VND") return 1;
  const prices = await getPrices(db);
  return prices.forex?.[currency]?.mid ?? null;
}

/**
 * Buy/sell forex rates for a currency.
 * buy = bank buys USD from you (you sell USD → get VND), sell = bank sells USD to you (you buy USD → pay VND).
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {string} currency
 * @returns {Promise<{ buy: number, sell: number }|null>}
 */
export async function getForexBidAsk(db, currency) {
  if (currency === "VND") return null;
  const prices = await getPrices(db);
  const rate = prices.forex?.[currency];
  if (!rate?.buy || !rate?.sell) return null;
  return { buy: rate.buy, sell: rate.sell };
}
