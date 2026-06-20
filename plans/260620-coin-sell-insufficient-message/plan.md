---
title: Improve coin sell insufficient message
description: >-
  Make /coin_sell insufficient-holdings replies explain available-to-sell USD
  value, with coin quantity and price as context.
status: completed
priority: P2
branch: main
tags:
  - bugfix
  - backend
  - ux
blockedBy: []
blocks: []
created: '2026-06-20T02:12:55.097Z'
createdBy: 'ck:plan'
source: skill
---

# Improve coin sell insufficient message

## Overview

`/coin_sell <COIN> <usd_amount>` sells a USD amount of crypto. When holdings are insufficient, the reply must stay in the user's USD framing while making clear the limiting balance is coin holdings, not USD cash.

Recommended UX:

```text
Not enough BTC to sell $600.00.
Available to sell: $500.00 (0.01 BTC @ $50,000.00).
Try $500.00 or less.
```

Zero holdings should use a clearer special case:

```text
No ETH available to sell.
Try /coin_buy ETH <usd_amount> first.
```

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Update sell message and tests](./phase-01-update-sell-message-and-tests.md) | Completed |

## Dependencies

- No blocking active plan. Existing coin module plan `plans/260612-1005-coin-module-price-fallback/plan.md` is completed and only provides context.

## Scope

- In scope: sell insufficient-holdings message in `internal/modules/coin/handlers.go`; regression tests in `internal/modules/coin/handlers_test.go`.
- Out of scope: changing command arguments, portfolio math, price providers, command registration, storage schema, buy wording, docs.

## Success Criteria

- `/coin_sell 600 BTC` after holding only `$500` worth of BTC says not enough BTC to sell `$600`, shows `$500` available, includes `0.01 BTC @ $50,000.00`, and suggests `$500.00 or less`.
- `/coin_sell 10 ETH` with zero ETH says no ETH available to sell and suggests buying ETH first.
- Failed sell still does not mutate portfolio.
- Existing successful sell response remains unchanged.
- `go test ./internal/modules/coin -count=1` and `go test ./... -count=1` pass.

## Unresolved Questions

None.
