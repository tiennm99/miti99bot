---
phase: 3
title: "Wire client into gold module and handlers"
status: completed
priority: P1
effort: "2h"
dependencies: [2]
---

# Phase 3: Wire client into gold module and handlers

## Overview

Integrate the VNAppMob SJC client as the primary gold price provider while keeping the existing XAU/USD chain as fallback. Update the `priceFetcher` interface and handlers so `/gold_price` shows SJC data and trading commands use SJC-derived prices.

## Requirements

- VNAppMob SJC is the first price source.
- On VNAppMob failure, fall back to existing `GoldPriceClient.FetchLuongPrice`.
- `/gold_price` output shows SJC buy/sell in VND/lượng.
- Portfolio commands (`gold_buy`, `gold_sell`, `gold_stats`) use a representative price (e.g. mid of buy/sell or sell price).

## Architecture

Introduce a composite fetcher in `internal/modules/gold/prices.go`:

```go
type compositePriceFetcher struct {
    vnappmob *VNAppMobClient
    fallback *GoldPriceClient
}

func (f *compositePriceFetcher) FetchLuongPrice(ctx context.Context) (float64, error) {
    buy, sell, err := f.vnappmob.FetchSJCPrice(ctx)
    if err == nil {
        return (buy + sell) / 2, nil
    }
    log.Warn("vnappmob_sjc_failed", "err", err)
    return f.fallback.FetchLuongPrice(ctx)
}

func (f *compositePriceFetcher) FetchPrice(ctx context.Context) (GoldPrice, error) {
    buy, sell, err := f.vnappmob.FetchSJCPrice(ctx)
    if err == nil {
        mid := (buy + sell) / 2
        return GoldPrice{XAUUSD: 0, USDVND: 0, VNDPerLuong: mid}, nil
    }
    return f.fallback.FetchPrice(ctx)
}
```

Update `newState(kv)` to build the composite fetcher. `helpers.go` already defines the `priceFetcher` interface.

Update `handlePrice` in `handlers.go`:
- If `FetchPrice` returned from SJC (detect via new flag or by checking `XAUUSD == 0`), show SJC-specific output.
- Otherwise keep existing spot-price output for fallback.

Add a field to `GoldPrice` to indicate the source, e.g.:
```go
type GoldPrice struct {
    XAUUSD      float64
    USDVND      float64
    VNDPerLuong float64
    Source      string // "vnappmob-sjc" or "xau-fallback"
    SJC         *SJCPrice // optional
}

type SJCPrice struct {
    Buy  float64
    Sell float64
}
```

## Related Code Files

- Modify: `internal/modules/gold/prices.go`
- Modify: `internal/modules/gold/helpers.go`
- Modify: `internal/modules/gold/handlers.go`
- Reference: `internal/modules/gold/gold.go`

## Implementation Steps

1. Extend `GoldPrice` struct with `Source string` and `SJC *SJCPrice`.
2. Implement `compositePriceFetcher` in `prices.go` (or new `composite_prices.go`).
3. Change `newState(kv)` to use composite fetcher.
4. Update `handlePrice` to render SJC-specific lines when `Source == "vnappmob-sjc"`.
5. Ensure `handleBuy`, `handleSell`, `handleStats` continue to work via `FetchLuongPrice`.

## Success Criteria

- [x] `/gold_price` shows SJC buy/sell when VNAppMob succeeds.
- [x] `/gold_price` falls back to old output when VNAppMob fails.
- [x] `gold_buy` and `gold_sell` use SJC mid price for cost/revenue.
- [x] `go vet` passes.

## Risk Assessment

- **Risk**: SJC response has only one of `buy_1l`/`sell_1l`. **Mitigation**: if one is missing, use the other; if both missing, return error so fallback kicks in.
- **Risk**: Existing tests assume `GoldPrice` shape. **Mitigation**: add fields without removing old ones.
