---
phase: 1
title: "Implement gold_price command"
status: completed
priority: P2
effort: "30min"
dependencies: []
---

# Phase 1: Implement gold_price command

## Overview

Add `/gold_price` — a read-only command that fetches and displays spot XAU in USD/oz and VND/luong. No arguments, no portfolio state, no lock needed.

## Requirements

- Functional: `/gold_price` with no arguments displays current gold price in USD per troy ounce and VND per luong.
- Functional: on price fetch failure, show user-friendly error (same pattern as `replyPriceError`).
- Non-functional: no portfolio mutation, no keylock acquisition.
- Non-functional: reject extra arguments (`len(args) != 0`).

## Architecture

Add a `GoldPrice` struct and `FetchPrice(ctx)` method to `GoldPriceClient` that returns all three components (XAU USD, USD/VND rate, VND/luong) in one call. Reuses existing `fetchXAUUSD` and `fetchUSDVND` internals. The `priceFetcher` interface gains `FetchPrice` so tests can mock it.

Output format:
```
Gold Spot Price
XAU: $3,285.50 USD/oz
Rate: 25,000 VND/USD
VND: 99,450,000 VND/luong
```

## Related Code Files

- Modify: `internal/modules/gold/prices.go` — add `GoldPrice` struct + `FetchPrice` method
- Modify: `internal/modules/gold/helpers.go` — update `priceFetcher` interface
- Modify: `internal/modules/gold/handlers.go` — add `handlePrice` handler
- Modify: `internal/modules/gold/gold.go` — register `gold_price` command
- Modify: `internal/modules/gold/format.go` — add `FormatUSD` helper
- Modify: `internal/modules/gold/handlers_test.go` — add handler test
- Modify: `internal/modules/gold/prices_test.go` — add `FetchPrice` test

## Implementation Steps

1. Add `GoldPrice` struct to `prices.go`:
   ```go
   type GoldPrice struct {
       XAUUSD     float64 // USD per troy ounce
       USDVND     float64 // VND per USD
       VNDPerLuong float64 // VND per luong
   }
   ```
2. Add `FetchPrice(ctx context.Context) (GoldPrice, error)` to `GoldPriceClient` — calls `fetchXAUUSD` + `fetchUSDVND`, computes VNDPerLuong, returns struct.
3. Update `priceFetcher` interface in `helpers.go` to include `FetchPrice`.
4. Update `fakePriceFetcher` in `handlers_test.go` to implement `FetchPrice`.
5. Add `FormatUSD(n float64) string` to `format.go` — e.g. `$3,285.50`.
6. Add `handlePrice` to `handlers.go`:
   - Reject if `len(args) != 0` → usage message.
   - Call `s.prices.FetchPrice(ctx)`.
   - On error → `replyPriceError`.
   - Format and reply with 3-line price summary.
7. Register `gold_price` command in `gold.go` with description `"Show current gold spot price (USD & VND)"`.
8. Add tests:
   - `TestHandlePrice` — verify output contains USD and VND lines.
   - `TestHandlePriceRejectsArgs` — verify extra args rejected.
   - `TestHandlePriceFetchError` — verify error path.
   - `TestGoldPriceClient_FetchPrice` — verify struct fields from httptest server.
9. Run `go test -count=1 ./internal/modules/gold/...` and `go vet ./...`.

## Success Criteria

- [ ] `/gold_price` returns USD/oz and VND/luong prices.
- [ ] Extra arguments rejected with usage message.
- [ ] Price fetch errors handled gracefully.
- [ ] `priceFetcher` interface updated; `fakePriceFetcher` implements both methods.
- [ ] All existing + new tests pass.
- [ ] `go vet` clean.
