---
phase: 4
title: "Command Handlers + Module Entry"
status: Pending
priority: P2
effort: 1.5h
depends_on: [1, 2, 3]
---

# Phase 4: Command Handlers + Module Entry

## Context

- [Module pattern](../../src/modules/misc/index.js) — reference for `init`, command shape
- [Registry types](../../src/modules/registry.js) — `BotModule` typedef
- Phases 1-3 provide: symbols, prices, portfolio, format

## Overview

Thin `index.js` wiring five `trade_*` commands. Each handler: parse args -> validate -> call data layer -> format reply.

## File: `src/modules/trading/index.js`

### Module shape

```js
const tradingModule = {
  name: "trading",
  init: async ({ db: store }) => { db = store; },
  commands: [
    { name: "trade_topup",   visibility: "public", description: "...", handler: handleTopup },
    { name: "trade_buy",     visibility: "public", description: "...", handler: handleBuy },
    { name: "trade_sell",    visibility: "public", description: "...", handler: handleSell },
    { name: "trade_convert", visibility: "public", description: "...", handler: handleConvert },
    { name: "trade_stats",   visibility: "public", description: "...", handler: handleStats },
  ],
};
export default tradingModule;
```

### Command implementations

#### `trade_topup <amount> [currency=VND]`

1. Parse: `ctx.match.trim().split(/\s+/)` -> `[amountStr, currencyStr?]`
2. Validate: amount > 0, numeric; currency in CURRENCIES (default VND)
3. Get portfolio, add currency
4. If currency !== VND: fetch forex rate, add `amount * rate` to `totalvnd`
5. If currency === VND: add amount to `totalvnd`
6. Save portfolio
7. Reply: `Topped up {formatCurrency(amount, currency)}. Balance: {formatCurrency(balance, currency)}`

#### `trade_buy <amount> <symbol>`

1. Parse args: amount + symbol
2. Validate: amount > 0; symbol exists in SYMBOLS
3. If stock: amount must be integer (`Number.isInteger(parseFloat(amount))`)
4. Fetch price via `getPrice(db, symbol)`
5. Cost = amount * price (in VND)
6. Deduct VND from portfolio; if insufficient -> error with current balance
7. Add asset to portfolio
8. Save, reply with purchase summary

#### `trade_sell <amount> <symbol>`

1. Parse + validate (same as buy)
2. Deduct asset; if insufficient -> error with current holding
3. Fetch price, revenue = amount * price
4. Add VND to portfolio
5. Save, reply with sale summary

#### `trade_convert <amount> <from> <to>`

1. Parse: amount, from-currency, to-currency
2. Validate: both in CURRENCIES, from !== to, amount > 0
3. Deduct `from` currency; if insufficient -> error
4. Fetch forex rates, compute converted amount
5. Add `to` currency
6. Save, reply with conversion summary

#### `trade_stats`

1. Get portfolio
2. Fetch all prices
3. For each category, compute current VND value
4. Sum all = total current value
5. P&L = total current value + currency.VND - totalvnd
6. Reply with formatted breakdown table

### Arg parsing helper

Extract into a local `parseArgs(ctx, specs)` at top of file:
- `specs` = array of `{ name, required, type: "number"|"string", default? }`
- Returns parsed object or null (replies usage hint on failure)
- Keeps handlers DRY

### Implementation steps

1. Create `src/modules/trading/index.js`
2. Module-level `let db = null;` set in `init`
3. Implement `parseArgs` helper (inline, ~20 lines)
4. Implement each handler function (~25-35 lines each)
5. Wire into `commands` array
6. Ensure file stays under 200 lines. If approaching limit, extract `parseArgs` to a `helpers.js` file

### Edge cases

| Input | Response |
|-------|----------|
| `/trade_buy` (no args) | Usage: `/trade_buy <amount> <symbol>` |
| `/trade_buy -5 BTC` | Amount must be positive |
| `/trade_buy 0.5 NOPE` | Unknown symbol. Supported: BTC, ETH, ... |
| `/trade_buy 1.5 TCB` | Stock quantities must be whole numbers |
| `/trade_buy 1 BTC` (no VND) | Insufficient VND. Balance: 0 VND |
| `/trade_sell 10 BTC` (only have 5) | Insufficient BTC. You have: 5 |
| `/trade_convert 100 VND VND` | Cannot convert to same currency |
| API failure during buy | Could not fetch price. Try again later. |

### Failure modes

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| File exceeds 200 lines | Medium | Low | Extract parseArgs to helpers.js |
| Price fetch fails mid-trade | Low | Medium | Catch, reply error, don't modify portfolio |
| User sends concurrent commands | Low | Low | Last write wins; acceptable for paper trading |

## Success criteria

- [ ] All 5 commands registered as public
- [ ] Each command validates input and replies helpful errors
- [ ] Buy/sell correctly modify both VND and asset balances
- [ ] Convert works between VND and USD
- [ ] Stats shows breakdown with P&L
- [ ] File under 200 lines (or split cleanly)
