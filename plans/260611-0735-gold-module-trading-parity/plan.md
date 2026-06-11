---
title: Gold module matching trading workflow
description: >-
  Add a standalone gold paper-trading module that mirrors trading UX, defaults
  topups to VND, defaults buys/sells to Vietnamese luong, and uses a free/no-key
  spot price source for v1.
status: completed
priority: P2
branch: main
tags:
  - gold
  - trading
  - telegram
  - price-api
  - free-tier
blockedBy: []
blocks: []
created: '2026-06-11T07:35:05.803Z'
createdBy: 'ck:plan'
source: skill
---

# Gold module matching trading workflow

## Overview

Add `internal/modules/gold` as a separate module, not an extension of `trading`. Keep user behavior parallel to `/trade_*` commands, but gold-only:

- `/gold_topup <amount>` credits VND only. No currency argument.
- `/gold_buy <luong>` buys gold in `luong` by default. No symbol or unit argument.
- `/gold_sell <luong>` sells gold in `luong` by default. No symbol or unit argument.
- `/gold_stats` shows VND balance, gold holding, current price, total value, invested amount, and P&L.

V1 pricing is explicitly **world spot XAU converted to VND per `luong`**, not Vietnamese SJC retail buy/sell price. Default price path: no-key GoldPrice.org spot XAU USD JSON plus no-key ExchangeRate-API USD to VND conversion, converted to VND per `luong` (`1 luong = 37.5g = 37.5 / 31.1034768 troy oz`). This is free-tier friendly but must be isolated behind a provider interface because GoldPrice.org JSON is undocumented. Exact Vietnamese SJC retail pricing is out of v1 unless a separate, higher-maintenance source is approved.

## Current Code Context

- `internal/modules/trading/trading.go` registers `trade_topup`, `trade_buy`, `trade_sell`, `trade_stats`, plus income helpers.
- `internal/modules/trading/handlers.go` already has the target workflow: parse command args, fetch price outside the per-user lock, mutate KV portfolio under `keylock.Map`, reply through `chathelper`.
- `internal/modules/trading/portfolio.go` stores per-user `Currency`, `Assets`, and `Meta.Invested` under `user:<id>`.
- `cmd/server/main.go` owns the module catalog; adding a module requires import + `"gold": gold.New`.
- `template.yaml` default `MODULES` currently includes `trading` but not `gold`.

## Price API Research

| Candidate | Free shape | Fit | Decision |
|---|---|---|---|
| GoldPrice.org `https://data-asg.goldprice.org/dbXRates/USD` | No key; current JSON has `items[0].xauPrice`, `curr`, and timestamp fields. Undocumented endpoint, no stability/SLA claim. | Best zero-secret v1 source for spot XAU if treated as best-effort. | Use as default provider for v1 behind an isolated client and runtime URL override. |
| ExchangeRate-API open endpoint `https://open.er-api.com/v6/latest/USD` | No key; docs require attribution, allow caching, note rate limiting, update once daily, and include `rates.VND`. | Good USD to VND conversion companion. | Use for USD/VND conversion; cache until `time_next_update_unix` when available and handle 429 explicitly. |
| Frankfurter | No-key FX API. | Possible FX fallback if VND support is verified during implementation. | Fallback only. |
| SJC official site | HTML price table, no public JSON API found. | Exact Vietnam local retail price would require scraping or another higher-maintenance source. | Out of v1. Do not plan default SJC JSON integration. |
| API Ninjas `/v1/goldprice` | Requires `X-Api-Key`; free users receive delayed data, and current product pages gate some endpoints. | Less aligned with no-secret free-tier. | Do not default. Keep as optional future provider. |
| Metals-API | Requires API key; current product is key-based and not a strict no-secret default. | Not free-tier enough for this bot. | Do not default. |

## Key Decisions

- Standalone module/package named `gold`, commands prefixed `gold_`.
- Separate KV namespace from `trading`; no cross-portfolio mixing.
- Keep holdings as `float64` luong, with a concrete dust rule: balances whose absolute value is `< 1e-9` are normalized to zero after arithmetic.
- Use VND as only cash currency. Do not accept `USD`, `VND`, symbols, or units in v1 commands.
- Fetch price before locking user state, same as trading, to keep lock scope short.
- Keep `gold` opt-in for first deploy. Do not add it to default `template.yaml` `MODULES` until the operator explicitly promotes the spot-priced module after opt-in smoke.
- No real order execution, no SJC spread, no fees, no cron refresh in v1.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Research and existing trade pattern](./phase-01-research-and-existing-trade-pattern.md) | Completed |
| 2 | [Gold price client](./phase-02-gold-price-client.md) | Completed |
| 3 | [Gold portfolio commands](./phase-03-gold-portfolio-commands.md) | Completed |
| 4 | [Module registration and docs](./phase-04-module-registration-and-docs.md) | Completed |
| 5 | [Tests and verification](./phase-05-tests-and-verification.md) | Completed |

## Dependencies

No blocking unfinished plan. Related prior plans are complete or broader deploy work:

- `plans/260510-0234-pre-deploy-wrapup/` completed the trading module pattern this plan mirrors.
- `plans/260605-0256-trade-income-events-command/` completed recent trading command additions; useful for test style only.

## Review Notes

Three read-only ClaudeKit agents reviewed this plan on 2026-06-11. Accepted changes:

- Resolved v1 pricing as world spot XAU converted to VND; SJC retail is out of scope.
- Changed rollout stance from default-enabled to opt-in first deploy.
- Added FX caching/rate-limit handling and GoldPrice best-effort caveat.
- Added concrete fractional `luong` dust behavior and special-float tests.
- Added real composition-root and cross-module namespace test requirements.

## Success Criteria

- Gold module compiles and registers when enabled in `MODULES`.
- `/gold_topup`, `/gold_buy`, `/gold_sell`, `/gold_stats` match trading behavior where applicable.
- Buy/sell quantities are interpreted as `luong` by default without a unit argument.
- Topup always credits VND without a currency argument.
- Unit tests cover parsing, insufficient funds/holdings, price failures, stats rendering, module registration, and namespace isolation.
- `go test ./internal/modules/gold ./internal/modules ./cmd/server` passes; run broader `go test ./...` before push.

## Out of Scope

- Physical gold dealer/SJC buy/sell spread.
- Multiple gold units in commands (`gram`, `chi`, `oz`).
- Cross-module transfers between trading and gold.
- Historical price charts, alerts, leaderboards, or cron refresh.
- Default production enablement in `template.yaml` before spot-price semantics are accepted.

## Unresolved Questions

None for implementation. Product caveat: v1 uses world spot converted to VND, not Vietnam SJC retail price.
