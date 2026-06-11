---
phase: 1
title: Research and existing trade pattern
status: completed
priority: P2
effort: 1h
dependencies: []
---

# Phase 1: Research and existing trade pattern

## Overview

Lock the exact behavior to mirror from the existing trading module and verify the external price path with a small manual smoke test before writing code.

## Requirements

- Functional: identify which trading behaviors apply directly to gold: topup, buy, sell, stats, per-user lock, per-user KV state, reply style.
- Functional: verify default command syntax: `/gold_topup <amount>`, `/gold_buy <luong>`, `/gold_sell <luong>`, `/gold_stats`.
- Non-functional: avoid coupling gold state to trading state; no shared mutable state between modules.
- Non-functional: keep new code files under 200 lines where practical by splitting price, portfolio, handlers, format, and factory files.

## Architecture

Gold should copy the trading module's workflow, not import trading handlers. The shared pattern is conceptual:

1. Parse command args.
2. Fetch current price if needed.
3. Acquire `keylock.Map` by Telegram user ID.
4. Load module-local portfolio from KV.
5. Mutate portfolio.
6. Save portfolio.
7. Reply via `chathelper`.

## Related Code Files

- Read: `internal/modules/trading/trading.go`
- Read: `internal/modules/trading/handlers.go`
- Read: `internal/modules/trading/portfolio.go`
- Read: `internal/modules/trading/prices.go`
- Read: `internal/modules/trading/format.go`
- Read: `cmd/server/main.go`
- Read: `template.yaml`
- Modify later: none in this phase

## Implementation Steps

1. Re-read trading command tests to copy expected style for parser and recording bot assertions.
2. Smoke-test GoldPrice.org JSON shape with `curl https://data-asg.goldprice.org/dbXRates/USD`.
3. Smoke-test USD/VND conversion source with `curl https://open.er-api.com/v6/latest/USD` and confirm `rates.VND` exists.
4. Decide provider fallback behavior:
   - If GoldPrice.org fails: return user-facing "Could not fetch gold price. Try again later."
   - If FX conversion fails: same error; do not trade on stale unknown conversion.
5. Record actual response fields used by code: `items[0].xauPrice`, `items[0].curr`, `ts`.
6. Confirm all command names pass existing command validation regex.

## Success Criteria

- [x] Existing trading workflow documented enough to implement without changing trading files.
- [x] Price source response fields verified against live endpoint or a captured fixture.
- [x] Decision recorded that v1 is spot gold converted to VND, not SJC retail price.

## Risk Assessment

GoldPrice.org endpoint is not formal API docs. Mitigation: isolate behind `GoldPriceClient`, keep tests fixture-based, and make endpoint overrideable by env/test injection so provider can be swapped without rewriting handlers.
