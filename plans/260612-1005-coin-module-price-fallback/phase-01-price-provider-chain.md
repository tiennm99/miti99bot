---
phase: 1
title: Price provider chain
status: completed
priority: P1
effort: 3-4h
dependencies: []
---

# Phase 1: Price provider chain

## Context Links

- Research: `plans/reports/260612-0948-coin-module-research-report.md`
- Reference price clients: `internal/modules/gold/prices.go`, `internal/modules/trading/prices.go`

## Overview

Build the crypto USD price lookup layer with fixed fallback order: Binance, Coinbase, CoinGecko. This phase should be independent from Telegram handlers and portfolio mutation.

## Key Insights

- Binance `/api/v3/ticker/price` is best first for major `USDT` pairs but pair coverage is not universal.
- Coinbase `/v2/exchange-rates` requires no auth and returns direct USD rates for supported base currencies.
- CoinGecko `/simple/price` has broad coin-ID coverage but public/demo rate limit is around 30 calls/min and variable.
- All provider failures must return typed errors and allow fallback where safe.

## Requirements

- Functional: fetch USD price for whitelisted symbol.
- Functional: try Binance `SYMBOLUSDT`, then optionally `SYMBOLUSD`, then Coinbase, then CoinGecko.
- Functional: return `CoinPrice{Symbol, USD, Source}` from first valid provider.
- Functional: expose env URL overrides: `COIN_BINANCE_API_URL`, `COIN_COINBASE_API_URL`, `COIN_COINGECKO_API_URL`.
- Non-functional: 10s HTTP timeout, injected HTTP client for tests, no network calls in tests.
- Non-functional: 15-30s in-memory cache; cache only valid positive prices.

## Architecture

```text
handlers/stats
  -> PriceClient.FetchUSD(symbol)
      -> cache lookup
      -> BinanceProvider.FetchUSD(symbol)
      -> CoinbaseProvider.FetchUSD(symbol)
      -> CoinGeckoProvider.FetchUSD(symbol)
      -> ErrNoCoinPrice
```

Provider-specific structs stay inside `internal/modules/coin`. Do not create shared price infrastructure until another module needs it.

## Related Code Files

- Create: `internal/modules/coin/prices.go`
- Create: `internal/modules/coin/price_providers.go`
- Create: `internal/modules/coin/symbols.go`
- Create: `internal/modules/coin/prices_test.go`
- Read: `internal/modules/gold/prices.go`
- Read: `internal/modules/gold/price_providers.go`
- Read: `internal/modules/trading/prices.go`

## Implementation Steps

1. Define `CoinPrice`, `PriceProvider`, `PriceClient`, `ErrNoCoinPrice`, and provider error handling.
2. Add supported symbol mapping:
   - `BTC -> bitcoin`
   - `ETH -> ethereum`
   - `SOL -> solana`
   - `BNB -> binancecoin`
   - `XRP -> ripple`
   - `ADA -> cardano`
   - `DOGE -> dogecoin`
   - `TON -> the-open-network`
3. Implement Binance provider:
   - request `?symbol={SYMBOL}USDT` first
   - if no price, request `?symbol={SYMBOL}USD`
   - parse JSON string `price`
   - treat non-2xx, 429, invalid/zero price as fallback-safe errors
4. Implement Coinbase provider:
   - request `?currency={SYMBOL}`
   - parse `data.rates.USD` string
5. Implement CoinGecko provider:
   - request `?ids={coinID}&vs_currencies=usd&include_last_updated_at=true`
   - parse `{coinID}.usd`
6. Implement cache keyed by symbol with source and expiry.
7. Add URL validation if following gold's endpoint validation pattern. At minimum reject blank/malformed override URLs.
8. Wrap errors with `coin:` prefix but keep typed `ErrNoCoinPrice` detectable.

## Todo List

- [x] Price client and provider interfaces added.
- [x] Binance provider implemented.
- [x] Coinbase provider implemented.
- [x] CoinGecko provider implemented.
- [x] Symbol whitelist and CoinGecko ID mapping added.
- [x] Cache implemented and tested.

## Success Criteria

- [x] First valid provider wins and returns source name.
- [x] 429/non-2xx/decode/no-price failures fall through to next provider.
- [x] Unsupported local symbols fail before network calls.
- [x] Unit tests cover each provider and fallback path.

## Risk Assessment

Main risk: accidental provider spam from `/coin_stats`. Mitigation: cache and no retry loops. Binance has IP-ban risk after repeated 429 abuse; on 429, immediately fall through and do not re-call within cache/backoff window.

## Security Considerations

No API keys required. Do not add secrets. Do not accept arbitrary user-provided URLs; env overrides only. Reject untrusted symbols before request building.

## Next Steps

Phase 2 consumes `PriceClient` via a small interface so handlers can use fakes in tests.
