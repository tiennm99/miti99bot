---
phase: 2
title: "Price Fetching + Caching"
status: Pending
priority: P2
effort: 1h
---

# Phase 2: Price Fetching + Caching

## Context

- [KV store interface](../../src/db/kv-store-interface.js) — `putJSON` supports `expirationTtl`
- [Symbol registry](phase-01-symbols-and-format.md)

## Overview

Fetches live prices from three free APIs, merges into a single cache object in KV with 60s TTL. All prices normalized to VND.

## File: `src/modules/trading/prices.js`

### Data shape — KV key `prices:latest`

```js
{
  ts: 1713100000000,          // Date.now() at fetch time
  crypto: { BTC: 2500000000, ETH: 75000000, SOL: 3500000 },
  stock:  { TCB: 25000, VPB: 18000, FPT: 120000, VNM: 70000, HPG: 28000 },
  forex:  { USD: 25400 },     // 1 USD = 25400 VND
  others: { GOLD: 75000000 }  // per troy oz in VND
}
```

### Exports

- `fetchPrices(db)` — fetch all APIs in parallel, merge, cache in KV, return merged object
- `getPrices(db)` — cache-first: read KV, if exists and < 60s old return it, else call `fetchPrices`
- `getPrice(db, symbol)` — convenience: calls `getPrices`, looks up by symbol + category
- `getForexRate(db, currency)` — returns VND equivalent of 1 unit of currency

### API calls

1. **Crypto + Gold (CoinGecko)**
   ```
   GET https://api.coingecko.com/api/v3/simple/price
     ?ids=bitcoin,ethereum,solana,pax-gold
     &vs_currencies=vnd
   ```
   Response: `{ bitcoin: { vnd: N }, ... }`
   Map `apiId -> VND price` using SYMBOLS registry.

2. **Vietnam stocks (TCBS)**
   For each stock symbol, fetch:
   ```
   GET https://apipubaws.tcbs.com.vn/stock-insight/v1/stock/bars-long-term
     ?ticker={SYMBOL}&type=stock&resolution=D&countBack=1&to={unix_seconds}
   ```
   Response: `{ data: [{ close: N }] }` — price in VND (already VND, multiply by 1000 for actual price per TCBS convention).
   Fetch all 5 stocks in parallel via `Promise.allSettled`.

3. **Forex (Exchange Rate API)**
   ```
   GET https://open.er-api.com/v6/latest/USD
   ```
   Response: `{ rates: { VND: N } }`
   Store as `forex.USD = rates.VND`.

### Implementation steps

1. Create `src/modules/trading/prices.js`
2. Implement `fetchCrypto()` — single CoinGecko call, map apiId->VND
3. Implement `fetchStocks()` — `Promise.allSettled` for all stock symbols, extract `close * 1000`
4. Implement `fetchForex()` — single call, extract VND rate
5. Implement `fetchPrices(db)`:
   - `Promise.allSettled([fetchCrypto(), fetchStocks(), fetchForex()])`
   - Merge results, set `ts: Date.now()`
   - `db.putJSON("prices:latest", merged)` — no expirationTtl (we manage staleness manually)
   - Return merged
6. Implement `getPrices(db)`:
   - `db.getJSON("prices:latest")`
   - If exists and `Date.now() - ts < 60_000`, return cached
   - Else call `fetchPrices(db)`
7. Implement `getPrice(db, symbol)`:
   - Get symbol info from registry
   - Get prices via `getPrices(db)`
   - Return `prices[category][symbol]`
8. Implement `getForexRate(db, currency)`:
   - If `currency === "VND"` return 1
   - If `currency === "USD"` return `prices.forex.USD`

### Edge cases

- Any single API fails -> `Promise.allSettled` catches it, use partial results + stale cache for missing category
- All APIs fail -> if cache < 5 min old, use it; else throw with user-friendly message
- CoinGecko rate-limited (30 calls/min free tier) -> 60s cache makes this safe for normal use
- TCBS returns empty data array -> skip that stock, log warning

### Failure modes

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| CoinGecko rate limit | Low (60s cache) | Medium | Cache prevents rapid re-fetch; degrade gracefully |
| TCBS API changes response shape | Medium | Medium | Defensive access `data?.[0]?.close`; skip stock on parse failure |
| Forex API down | Low | Low | USD conversion unavailable; VND operations still work |
| All APIs down simultaneously | Very Low | High | Fall back to cache if < 5min; clear error message if no cache |

### Security

- No API keys needed (all free public endpoints)
- No user data sent to external APIs

## Success criteria

- [ ] `fetchPrices` calls 3 APIs in parallel, returns merged object
- [ ] `getPrices` returns cached data within 60s window
- [ ] `getPrices` refetches when cache is stale
- [ ] Partial API failure doesn't crash — missing data logged, rest returned
- [ ] `getPrice(db, "BTC")` returns a number (VND)
- [ ] `getForexRate(db, "VND")` returns 1
- [ ] File under 200 lines
