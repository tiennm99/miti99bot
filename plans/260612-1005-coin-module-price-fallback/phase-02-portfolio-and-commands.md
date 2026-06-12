---
phase: 2
title: Portfolio and commands
status: completed
priority: P1
effort: 4-5h
dependencies:
  - 1
---

# Phase 2: Portfolio and commands

## Context Links

- Price client phase: `phase-01-price-provider-chain.md`
- Gold handler pattern: `internal/modules/gold/handlers.go`, `internal/modules/gold/portfolio.go`
- Trading handler pattern: `internal/modules/trading/handlers.go`, `internal/modules/trading/portfolio.go`

## Overview

Implement the standalone `coin` module state, portfolio model, formatting helpers, and Telegram commands for USD paper trading.

## Key Insights

- Coin balances must be fractional `float64`, like gold holdings.
- Portfolio state must stay module-scoped via `kv.For("coin")`; storage key can remain `user:<telegramID>` inside namespace.
- Fetch price before acquiring per-user lock or CAS update.
- Use `UpdatePortfolio` CAS pattern from `gold` if storage supports `CompareAndSwapStore`.

## Requirements

- Functional: `/coin_price <COIN>` shows USD price and provider source.
- Functional: `/coin_topup <usd_amount>` credits fake USD and increments invested.
- Functional: `/coin_buy <usd_amount> <COIN>` deducts USD and credits `usd_amount / price` units.
- Functional: `/coin_sell <qty> <COIN>` deducts coin units and credits `qty * price` USD.
- Functional: `/coin_stats` shows USD, holdings, price source, market value, total, invested, P&L.
- Non-functional: reject invalid sender, unsupported coin, non-finite amount, insufficient balance, and too-large trade values.

## Architecture

```go
type Portfolio struct {
    USD    float64            `json:"usd"`
    Assets map[string]float64 `json:"assets"`
    Meta   PortfolioMeta      `json:"meta"`
}

type PortfolioMeta struct {
    Invested  float64 `json:"invested"`
    CreatedAt int64   `json:"createdAt"`
}
```

Handlers depend on a `priceFetcher` interface returning `CoinPrice`, not concrete providers. This keeps tests deterministic.

## Related Code Files

- Create: `internal/modules/coin/coin.go`
- Create: `internal/modules/coin/helpers.go`
- Create: `internal/modules/coin/handlers.go`
- Create: `internal/modules/coin/portfolio.go`
- Create: `internal/modules/coin/format.go`
- Create: `internal/modules/coin/handlers_test.go`
- Create: `internal/modules/coin/portfolio_test.go`
- Read: `internal/modules/gold/*`
- Read: `internal/modules/trading/*`

## Implementation Steps

1. Create `coin.go` with public commands:
   - `coin_price`
   - `coin_topup`
   - `coin_buy`
   - `coin_sell`
   - `coin_stats`
2. Create `helpers.go` with local copies/patterns for `senderInfo`, `argsAfterCommand`, finite positive parsing, and safe USD checks.
3. Create `portfolio.go` using gold's `UpdatePortfolio` retry/CAS approach:
   - initialize `Assets` map on load
   - normalize NaN/Inf and dust below `1e-9`
   - delete asset key when holding hits zero
4. Create formatting helpers:
   - `FormatUSD`
   - `FormatCoinQty`
   - `FormatPnLUSD`
5. Implement `/coin_price <COIN>` read-only path.
6. Implement `/coin_topup <usd_amount>` with no provider call.
7. Implement `/coin_buy <usd_amount> <COIN>`:
   - validate amount and symbol
   - fetch price
   - compute qty
   - CAS update deduct USD/add asset
8. Implement `/coin_sell <qty> <COIN>`:
   - validate qty and symbol
   - fetch price
   - CAS update deduct asset/add USD
9. Implement `/coin_stats`:
   - load portfolio
   - fetch prices for held assets through cached client
   - degrade gracefully if some prices fail
10. Keep replies concise and include source, e.g. `Price: $67,321.42 (Binance)`.

## Todo List

- [x] Module factory created.
- [x] Portfolio state and update helpers created.
- [x] Command handlers implemented.
- [x] Formatting helpers created.
- [x] Handler tests use fake price fetcher.

## Success Criteria

- [x] All commands return usage messages for bad argument count.
- [x] Topup changes USD and invested only.
- [x] Buy/sell mutate state only after price success.
- [x] Stats works with empty portfolio and with held assets.
- [x] Source name visible in price-dependent replies.

## Risk Assessment

Main risk: floating-point dust or accidental negative balances. Mitigation: finite validation, dust normalization, safe range checks, and table-driven tests around exact insufficient-balance paths.

## Security Considerations

This is fake money. Still treat state updates as user-owned data: key by Telegram user ID, reject senderless updates, and avoid logging user balances unnecessarily.

## Next Steps

Phase 3 wires the package into startup and deployment docs.
