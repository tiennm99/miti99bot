---
title: Coin module with crypto price fallback
description: >-
  Add a standalone coin paper-trading module with USD balances and Binance ->
  Coinbase -> CoinGecko price fallback.
status: completed
priority: P2
effort: 10-14h
branch: main
tags:
  - feature
  - backend
  - api
blockedBy: []
blocks: []
created: '2026-06-12'
createdBy: 'ck:plan'
source: skill
---

# Coin module with crypto price fallback

## Overview

Add `internal/modules/coin` as a standalone crypto paper-trading module. Users top up fake USD, buy/sell supported coins at market price, and view portfolio stats. Price lookup uses a best-effort provider chain: Binance first, Coinbase second, CoinGecko third.

## Scope Challenge

- Existing code: `gold` already has the closest fractional-asset portfolio, CAS update, command, price-client, and docs pattern. `trading` has useful asset-map and command naming patterns.
- Minimum changes: new `coin` package, factory registration, optional env URL config, README/template docs, tests. No shared trading refactor required.
- Complexity: expected 10-12 touched files. New abstractions limited to `PriceProvider` interface and provider structs inside `coin` package.
- Selected mode: HOLD SCOPE. Deliver robust MVP, defer nonessential crypto features.

## Key Decisions

- Paper trading only. No wallets, private keys, deposits, withdrawals, real exchange orders, leverage, charts, or tax logic.
- USD-only cash balance for v1.
- Supported coin whitelist first; no arbitrary symbols.
- Provider order fixed for v1: Binance -> Coinbase -> CoinGecko.
- Include price source in user replies so provider differences are visible.
- Enable `coin` in the deployed `ModulesCSV` default so AWS registration includes the module.

## References

- Research: `plans/reports/260612-0948-coin-module-research-report.md`
- Patterns: `internal/modules/gold`, `internal/modules/trading`
- Composition root: `cmd/server/main.go`
- Deployment config: `template.yaml`

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Price provider chain](./phase-01-price-provider-chain.md) | Completed |
| 2 | [Portfolio and commands](./phase-02-portfolio-and-commands.md) | Completed |
| 3 | [Module registration and docs](./phase-03-module-registration-and-docs.md) | Completed |
| 4 | [Tests and verification](./phase-04-tests-and-verification.md) | Completed |

## Dependencies

- No blocking active plan detected.
- Completed gold plan is reference only: `plans/260611-0735-gold-module-trading-parity/plan.md`.
- Existing unresolved migration/deploy plans touch `trading`/infra, not `coin`; no bidirectional dependency needed.

## Success Criteria

- `/coin_price`, `/coin_topup`, `/coin_buy`, `/coin_sell`, `/coin_stats` work when `coin` is enabled.
- Price client falls back Binance -> Coinbase -> CoinGecko and never mutates portfolio when all providers fail.
- Tests cover portfolio math, handler validation, provider decoding, provider fallback, and module registration.
- `go test ./internal/modules/coin ./cmd/server ./internal/modules/...` passes; run `go test ./...` before push.

## Out Of Scope

- Real exchange trading.
- Wallet/on-chain integration.
- Cross-module transfers between `trading`, `gold`, and `coin`.
- Limit orders, recurring buys, alerts, charts, or leaderboards.
- Dynamic coin discovery from public APIs.

## Unresolved Questions

- Should `/coin_buy` remain USD amount only, or support quantity mode too?
- Initial whitelist: top 8 from research enough, or include more now?
