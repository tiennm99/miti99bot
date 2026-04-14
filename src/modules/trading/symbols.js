/**
 * @file Symbol registry — hardcoded list of tradable assets + fiat currencies.
 * Adding a new asset = one line here, no logic changes elsewhere.
 */

/** @typedef {{ category: "crypto"|"stock"|"others", apiId: string, label: string }} SymbolEntry */

/** @type {Readonly<Record<string, SymbolEntry>>} */
export const SYMBOLS = Object.freeze({
  // crypto — CoinGecko IDs
  BTC: { category: "crypto", apiId: "bitcoin", label: "Bitcoin" },
  ETH: { category: "crypto", apiId: "ethereum", label: "Ethereum" },
  SOL: { category: "crypto", apiId: "solana", label: "Solana" },
  // Vietnam stocks — TCBS tickers
  TCB: { category: "stock", apiId: "TCB", label: "Techcombank" },
  VPB: { category: "stock", apiId: "VPB", label: "VPBank" },
  FPT: { category: "stock", apiId: "FPT", label: "FPT Corp" },
  VNM: { category: "stock", apiId: "VNM", label: "Vinamilk" },
  HPG: { category: "stock", apiId: "HPG", label: "Hoa Phat" },
  // others
  GOLD: { category: "others", apiId: "pax-gold", label: "Gold (troy oz)" },
});

/** Supported fiat currencies */
export const CURRENCIES = Object.freeze(new Set(["VND", "USD"]));

/**
 * Case-insensitive symbol lookup.
 * @param {string} name
 * @returns {SymbolEntry & { symbol: string } | undefined}
 */
export function getSymbol(name) {
  if (!name) return undefined;
  const key = name.toUpperCase();
  const entry = SYMBOLS[key];
  return entry ? { ...entry, symbol: key } : undefined;
}

/**
 * Formatted list of all supported symbols grouped by category.
 * @returns {string}
 */
export function listSymbols() {
  const groups = { crypto: [], stock: [], others: [] };
  for (const [sym, entry] of Object.entries(SYMBOLS)) {
    groups[entry.category].push(`${sym} — ${entry.label}`);
  }
  const lines = [];
  if (groups.crypto.length) lines.push("Crypto:", ...groups.crypto.map((s) => `  ${s}`));
  if (groups.stock.length) lines.push("Stocks:", ...groups.stock.map((s) => `  ${s}`));
  if (groups.others.length) lines.push("Others:", ...groups.others.map((s) => `  ${s}`));
  return lines.join("\n");
}
