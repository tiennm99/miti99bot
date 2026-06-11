---
phase: 3
title: Gold portfolio commands
status: completed
priority: P1
effort: 3h
dependencies:
  - 2
---

# Phase 3: Gold portfolio commands

## Overview

Build the gold module's state, formatting, and user-facing command handlers.

## Requirements

- Functional: `/gold_topup <amount>` credits VND and increments invested amount.
- Functional: `/gold_buy <luong>` deducts VND at current VND/luong price and adds gold holding.
- Functional: `/gold_sell <luong>` deducts gold holding and credits VND.
- Functional: `/gold_stats` renders VND, gold luong, current price, gold value, total value, invested, P&L.
- Functional: full sell after fractional buys must leave exact zero after dust normalization.
- Non-functional: no command accepts a currency, ticker, or unit in v1.
- Non-functional: floating quantities must reject NaN, Inf, overflow, zero, and negative values.

## Architecture

Use module-local state:

```go
type Portfolio struct {
    VND   float64       `json:"vnd"`
    Luong float64       `json:"luong"`
    Meta  PortfolioMeta `json:"meta"`
}
```

This is simpler than trading's `Currency` and `Assets` maps because v1 gold has exactly one cash currency and one asset. Storage key remains `user:<telegramID>` inside the gold module namespace. Arithmetic uses a concrete dust threshold: after each balance mutation, values whose absolute value is `< 1e-9` are set to zero.

## Related Code Files

- Create: `internal/modules/gold/gold.go`
- Create: `internal/modules/gold/handlers.go`
- Create: `internal/modules/gold/portfolio.go`
- Create: `internal/modules/gold/format.go`
- Create: `internal/modules/gold/handlers_test.go`
- Create: `internal/modules/gold/portfolio_test.go`
- Read: `internal/modules/trading/handlers.go`
- Read: `internal/modules/trading/portfolio.go`
- Read: `internal/modules/trading/format.go`

## Implementation Steps

1. Add `state` with `kv`, `prices`, `locks`, and `nowFn`.
2. Copy `senderInfo` and `argsAfterCommand` locally or extract only if another module already has a shared helper. Do not refactor trading unless necessary.
3. Add a shared local parser for positive finite floats. It must reject `NaN`, `Inf`, `+Inf`, `-Inf`, overflow, zero, and negative values.
4. Implement `LoadPortfolio`, `SavePortfolio`, `AddVND`, `DeductVND`, `AddLuong`, and `DeductLuong`.
5. Apply dust cleanup after each mutation using `const goldDustEpsilon = 1e-9`.
6. Implement `FormatLuong`, keeping up to 4 decimals and trimming trailing zeros.
7. Implement `handleTopup`:
   - usage: `Usage: /gold_topup <amount>`
   - parse amount as positive finite float
   - add VND, increment invested
8. Implement `handleBuy`:
   - usage: `Usage: /gold_buy <luong>`
   - fetch VND/luong before lock
   - cost = qty * price
   - deduct VND, add luong
9. Implement `handleSell`:
   - fetch price before lock
   - deduct luong, add VND
10. Implement `handleStats`:
   - fetch price; if unavailable, show holdings with `(no price)` and cash balance
   - include total value and P&L when price exists
11. Keep replies plain text unless future Telegram formatting needs HTML.

## Success Criteria

- [x] Fresh user can top up, buy, sell, and view stats.
- [x] Insufficient VND and insufficient gold return clear messages.
- [x] Price errors do not mutate portfolio.
- [x] Portfolio load repairs zero-value/missing fields safely.
- [x] Fractional buy/sell round trips leave no dust above `1e-9`.
- [x] Special float strings and overflow inputs are rejected before mutation.

## Risk Assessment

Using float for `luong` can produce tiny rounding artifacts. Mitigation: use one explicit epsilon, normalize dust to zero after arithmetic, test full fractional sell scenarios, and keep display precision bounded.
