---
phase: 3
title: "Portfolio Data Layer"
status: Pending
priority: P2
effort: 45m
---

# Phase 3: Portfolio Data Layer

## Context

- [KV interface](../../src/db/kv-store-interface.js) — `getJSON`/`putJSON`
- [Symbols](phase-01-symbols-and-format.md)

## Overview

CRUD operations on per-user portfolio KV objects. Pure data logic, no Telegram ctx dependency.

## File: `src/modules/trading/portfolio.js`

### Data shape — KV key `user:{telegramId}`

```js
{
  currency: { VND: 0, USD: 0 },
  stock: {},       // e.g. { TCB: 10, FPT: 5 }
  crypto: {},      // e.g. { BTC: 0.5, ETH: 2.1 }
  others: {},      // e.g. { GOLD: 0.1 }
  totalvnd: 0      // cumulative VND topped up (cost basis)
}
```

### Exports

- `getPortfolio(db, userId)` — returns portfolio object; inits empty if first-time user
- `savePortfolio(db, userId, portfolio)` — writes to KV
- `addCurrency(portfolio, currency, amount)` — mutates + returns portfolio
- `deductCurrency(portfolio, currency, amount)` — returns `{ ok, portfolio, balance }`. `ok=false` if insufficient
- `addAsset(portfolio, symbol, qty)` — adds to correct category bucket
- `deductAsset(portfolio, symbol, qty)` — returns `{ ok, portfolio, held }`. `ok=false` if insufficient
- `emptyPortfolio()` — returns fresh empty portfolio object

### Implementation steps

1. Create `src/modules/trading/portfolio.js`
2. `emptyPortfolio()` — returns deep clone of default shape
3. `getPortfolio(db, userId)`:
   - `db.getJSON("user:" + userId)`
   - If null, return `emptyPortfolio()`
   - Validate shape: ensure all category keys exist (migration-safe)
4. `savePortfolio(db, userId, portfolio)`:
   - `db.putJSON("user:" + userId, portfolio)`
5. `addCurrency(portfolio, currency, amount)`:
   - `portfolio.currency[currency] += amount`
   - Return portfolio
6. `deductCurrency(portfolio, currency, amount)`:
   - Check `portfolio.currency[currency] >= amount`
   - If not, return `{ ok: false, portfolio, balance: portfolio.currency[currency] }`
   - Deduct, return `{ ok: true, portfolio }`
7. `addAsset(portfolio, symbol, qty)`:
   - Lookup category from SYMBOLS
   - `portfolio[category][symbol] = (portfolio[category][symbol] || 0) + qty`
8. `deductAsset(portfolio, symbol, qty)`:
   - Lookup category, check held >= qty
   - If not, return `{ ok: false, portfolio, held: portfolio[category][symbol] || 0 }`
   - Deduct (remove key if 0), return `{ ok: true, portfolio }`

### Edge cases

- First-time user -> `getPortfolio` returns empty, no KV write until explicit `savePortfolio`
- Deduct exactly full balance -> ok, set to 0
- Deduct asset to exactly 0 -> delete key from object (keep portfolio clean)
- Portfolio shape migration: if old KV entry missing a category key, fill with `{}`

### Failure modes

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Race condition (two concurrent commands) | Low (paper trading) | Low | Acceptable for paper trading; last write wins |
| KV write fails | Low | Medium | Command handler catches, reports error to user |

## Success criteria

- [ ] `getPortfolio` returns empty portfolio for new user
- [ ] `addCurrency` + `deductCurrency` correctly modify balances
- [ ] `deductCurrency` returns `ok: false` when insufficient
- [ ] `addAsset` / `deductAsset` work across all 3 categories
- [ ] `deductAsset` to zero removes key from category object
- [ ] File under 120 lines
