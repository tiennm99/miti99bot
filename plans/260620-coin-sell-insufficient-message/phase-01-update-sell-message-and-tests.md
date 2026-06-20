---
phase: 1
title: Update sell message and tests
status: completed
priority: P2
dependencies: []
effort: 30-45m
---

# Phase 1: Update sell message and tests

## Overview

Improve `/coin_sell` insufficient-holdings replies so users see the failed sell amount and available-to-sell value in USD, with coin quantity and current price as supporting context.

## Requirements

- Functional: If user has some holdings but less than requested USD sell amount, reply:
  - `Not enough <COIN> to sell <requested USD>.`
  - `Available to sell: <available USD> (<held qty> <COIN> @ <price USD>).`
  - `Try <available USD> or less.`
- Functional: If user has zero holdings, reply:
  - `No <COIN> available to sell.`
  - `Try /coin_buy <COIN> <usd_amount> first.`
- Functional: Keep successful sell response unchanged.
- Non-functional: Preserve current portfolio mutation semantics and command argument contract.

## Architecture

`handleSell` already resolves the USD amount, fetches price, computes required coin quantity, and receives held quantity from `Portfolio.DeductAsset` on failure. Reuse that held quantity to format a user-facing reply through a small helper or localized branch inside `handleSell`.

No new storage, price, command registration, or module boundary changes.

## Related Code Files

- Modify: `internal/modules/coin/handlers.go` — format improved insufficient sell reply.
- Modify: `internal/modules/coin/handlers_test.go` — assert partial-holding and zero-holding wording.
- Delete: none.
- Create: none.

## Implementation Steps

1. Add or inline a small formatter for insufficient sell replies.
2. In `handleSell`, keep `held` from `DeductAsset`.
3. If `held` normalizes to zero, return the zero-holdings message.
4. Otherwise compute `availableUSD := held * price.USD` and return the three-line available-to-sell message.
5. Update `TestHandleSellInsufficientCoin` to assert the zero-holdings copy.
6. Update `TestHandleSellInsufficientCoinWithHoldings` to assert requested USD, available USD, coin quantity, price, and retry hint; keep mutation assertion.
7. Run focused and full tests.

## Success Criteria

- [x] Partial-holdings insufficient sell message uses requested USD as primary value.
- [x] Partial-holdings insufficient sell message includes available-to-sell USD, coin quantity, and price.
- [x] Zero-holdings insufficient sell message does not show `$0.00 (0 ETH @ price)`.
- [x] Failed sell leaves portfolio unchanged.
- [x] `go test ./internal/modules/coin -count=1` passes.
- [x] `go test ./... -count=1` passes.

## Risk Assessment

- Risk: message includes USD cash-like wording and remains ambiguous. Mitigation: use `Available to sell`, not `have`.
- Risk: floating point precision leaks into text. Mitigation: use existing `FormatUSD` and `FormatCoinQty`.
- Risk: tests become too brittle around full text. Mitigation: assert meaningful substrings, not every newline.

## Unresolved Questions

None.
