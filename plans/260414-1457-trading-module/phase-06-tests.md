---
phase: 6
title: "Tests"
status: Pending
priority: P2
effort: 1.5h
depends_on: [1, 2, 3, 4]
---

# Phase 6: Tests

## Overview

Unit tests for all trading module files. Use existing test infrastructure: vitest, `makeFakeKv`, fake bot helpers.

## Test files

### `tests/modules/trading/symbols.test.js`

| Test | Assertion |
|------|-----------|
| SYMBOLS has 9 entries | `Object.keys(SYMBOLS).length === 9` |
| Every entry has category, apiId, label | Shape check |
| getSymbol case-insensitive | `getSymbol("btc")` === `getSymbol("BTC")` |
| getSymbol unknown returns undefined | `getSymbol("NOPE")` === `undefined` |
| getSymbol falsy input returns undefined | `getSymbol("")`, `getSymbol(null)` |
| listSymbols groups by category | Contains "crypto", "stock", "others" headers |
| CURRENCIES has VND and USD | Set membership |

### `tests/modules/trading/format.test.js`

| Test | Assertion |
|------|-----------|
| formatVND(15000000) | `"15.000.000 VND"` |
| formatVND(0) | `"0 VND"` |
| formatVND(500) | `"500 VND"` |
| formatUSD(1234.5) | `"$1,234.50"` |
| formatUSD(0) | `"$0.00"` |
| formatCrypto(0.001) | `"0.001"` |
| formatCrypto(1.00000000) | `"1"` |
| formatCrypto(0.12345678) | `"0.12345678"` |
| formatStock(1.7) | `"1"` |
| formatStock(100) | `"100"` |
| formatAmount dispatches correctly | BTC->crypto, TCB->stock, GOLD->others |
| formatCurrency dispatches | VND->formatVND, USD->formatUSD |

### `tests/modules/trading/portfolio.test.js`

| Test | Assertion |
|------|-----------|
| emptyPortfolio has correct shape | All keys present, zeroed |
| getPortfolio returns empty for new user | Uses fake KV |
| getPortfolio returns stored data | Pre-seed KV |
| addCurrency increases balance | `addCurrency(p, "VND", 1000)` |
| deductCurrency succeeds | Sufficient balance |
| deductCurrency fails insufficient | Returns `{ ok: false, balance }` |
| deductCurrency exact balance | Returns `{ ok: true }`, balance = 0 |
| addAsset correct category | BTC -> crypto, TCB -> stock |
| deductAsset succeeds | Sufficient holding |
| deductAsset fails insufficient | Returns `{ ok: false, held }` |
| deductAsset to zero removes key | Key deleted from category |
| savePortfolio round-trips | Write then read |

### `tests/modules/trading/prices.test.js`

Strategy: Mock `fetch` globally in vitest to return canned API responses. Do NOT call real APIs.

| Test | Assertion |
|------|-----------|
| fetchPrices merges all 3 sources | Correct shape with all categories |
| getPrices returns cache when fresh | Only 1 fetch call if called twice within 60s |
| getPrices refetches when stale | Simulated stale timestamp |
| getPrice returns correct value | `getPrice(db, "BTC")` returns mocked VND price |
| getForexRate VND returns 1 | No fetch needed |
| getForexRate USD returns rate | From mocked forex response |
| Partial API failure | One API rejects; others still returned |
| All APIs fail, stale cache < 5min | Returns stale cache |
| All APIs fail, no cache | Throws with user-friendly message |

### `tests/modules/trading/commands.test.js`

Strategy: Integration-style tests. Use `makeFakeKv` for real KV behavior. Mock `fetch` for price APIs. Simulate grammY `ctx` with `ctx.match` and `ctx.reply` spy.

Helper: `makeCtx(match, userId?)` — returns `{ match, from: { id: userId }, reply: vi.fn() }`

| Test | Assertion |
|------|-----------|
| trade_topup adds VND | Portfolio balance increases |
| trade_topup adds USD + totalvnd | USD balance + totalvnd updated |
| trade_topup no args | Reply contains "Usage" |
| trade_topup negative amount | Reply contains error |
| trade_buy deducts VND, adds asset | Both modified |
| trade_buy stock fractional | Reply contains "whole numbers" |
| trade_buy insufficient VND | Reply contains balance |
| trade_buy unknown symbol | Reply lists supported symbols |
| trade_sell adds VND, deducts asset | Both modified |
| trade_sell insufficient holding | Reply contains current holding |
| trade_convert VND->USD | Both currencies modified |
| trade_convert same currency | Error message |
| trade_stats empty portfolio | Shows zero values |
| trade_stats with holdings | Shows breakdown + P&L |
| Price API failure during buy | Error message, portfolio unchanged |

## Implementation steps

1. Create test directory: `tests/modules/trading/`
2. Create `symbols.test.js` (~40 lines)
3. Create `format.test.js` (~60 lines)
4. Create `portfolio.test.js` (~80 lines)
5. Create `prices.test.js` (~90 lines) — mock global fetch
6. Create `commands.test.js` (~120 lines) — mock fetch + fake KV
7. Run `npm test` — all pass
8. Run `npm run lint` — clean

### Fetch mocking pattern

```js
import { vi, beforeEach } from "vitest";

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

beforeEach(() => mockFetch.mockReset());

// Per-test setup:
mockFetch.mockImplementation((url) => {
  if (url.includes("coingecko")) return Response.json({ bitcoin: { vnd: 2500000000 } });
  if (url.includes("tcbs")) return Response.json({ data: [{ close: 25 }] });
  if (url.includes("er-api")) return Response.json({ rates: { VND: 25400 } });
});
```

## Success criteria

- [ ] All new tests pass
- [ ] All 56 existing tests still pass
- [ ] Coverage: every public export of symbols, format, portfolio, prices tested
- [ ] Command handler tests cover happy path + all error branches
- [ ] Lint passes
- [ ] No real HTTP calls in tests
