---
phase: 1
title: "Symbol Registry + Formatters"
status: Pending
priority: P2
effort: 45m
---

# Phase 1: Symbol Registry + Formatters

## Context

- [Module pattern](../../docs/adding-a-module.md)
- [KV interface](../../src/db/kv-store-interface.js)

## Overview

Two pure-data/pure-function files with zero side effects. Foundation for all other phases.

## File: `src/modules/trading/symbols.js`

### Requirements

- Export `SYMBOLS` — frozen object keyed by uppercase symbol name
- Export `CURRENCIES` — frozen Set of supported fiat: `VND`, `USD`
- Export helper `getSymbol(name)` — case-insensitive lookup, returns entry or `undefined`
- Export helper `listSymbols()` — returns formatted string of all symbols grouped by category

### Data shape

```js
export const SYMBOLS = Object.freeze({
  // crypto
  BTC:  { category: "crypto",  apiId: "bitcoin",  label: "Bitcoin" },
  ETH:  { category: "crypto",  apiId: "ethereum", label: "Ethereum" },
  SOL:  { category: "crypto",  apiId: "solana",   label: "Solana" },
  // stock (Vietnam)
  TCB:  { category: "stock",   apiId: "TCB", label: "Techcombank" },
  VPB:  { category: "stock",   apiId: "VPB", label: "VPBank" },
  FPT:  { category: "stock",   apiId: "FPT", label: "FPT Corp" },
  VNM:  { category: "stock",   apiId: "VNM", label: "Vinamilk" },
  HPG:  { category: "stock",   apiId: "HPG", label: "Hoa Phat" },
  // others
  GOLD: { category: "others",  apiId: "pax-gold", label: "Gold (troy oz)" },
});
```

### Implementation steps

1. Create `src/modules/trading/symbols.js`
2. Define `SYMBOLS` constant with all entries above
3. Define `CURRENCIES = Object.freeze(new Set(["VND", "USD"]))`
4. `getSymbol(name)` — `SYMBOLS[name.toUpperCase()]` with guard for falsy input
5. `listSymbols()` — group by category, format as `SYMBOL — Label` per line
6. Keep under 60 lines

---

## File: `src/modules/trading/format.js`

### Requirements

- `formatVND(n)` — integer, dot thousands separator, suffix ` VND`. Example: `15.000.000 VND`
- `formatUSD(n)` — 2 decimals, comma thousands, prefix `$`. Example: `$1,234.56`
- `formatCrypto(n)` — up to 8 decimals, strip trailing zeros. Example: `0.00125`
- `formatStock(n)` — integer (Math.floor), no decimals. Example: `150`
- `formatAmount(n, symbol)` — dispatcher: looks up symbol category, calls correct formatter
- `formatCurrency(n, currency)` — VND or USD formatter based on currency string

### Implementation steps

1. Create `src/modules/trading/format.js`
2. `formatVND`: `Math.round(n).toLocaleString("vi-VN")` + ` VND` — verify dot separator (or manual impl for CF Workers locale support)
3. `formatUSD`: `n.toFixed(2)` with comma grouping + `$` prefix
4. `formatCrypto`: `parseFloat(n.toFixed(8)).toString()` to strip trailing zeros
5. `formatStock`: `Math.floor(n).toString()`
6. `formatAmount`: switch on `getSymbol(sym).category`
7. `formatCurrency`: switch on currency string
8. Keep under 80 lines

### Edge cases

- `formatVND(0)` -> `0 VND`
- `formatCrypto(1.00000000)` -> `1`
- `formatAmount` with unknown symbol -> return raw number string
- CF Workers may not have full locale support — implement manual dot-separator for VND

### Failure modes

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `toLocaleString` not available in CF Workers runtime | Medium | Medium | Manual formatter fallback: split on groups of 3, join with `.` |

## Success criteria

- [ ] `SYMBOLS` has 9 entries across 3 categories
- [ ] `getSymbol("btc")` returns BTC entry (case-insensitive)
- [ ] `getSymbol("NOPE")` returns `undefined`
- [ ] `formatVND(15000000)` === `"15.000.000 VND"`
- [ ] `formatCrypto(0.001)` === `"0.001"` (no trailing zeros)
- [ ] `formatStock(1.7)` === `"1"`
- [ ] Both files under 200 lines
