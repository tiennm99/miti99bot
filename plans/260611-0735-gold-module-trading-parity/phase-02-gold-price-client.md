---
phase: 2
title: Gold price client
status: completed
priority: P1
effort: 2h
dependencies:
  - 1
---

# Phase 2: Gold price client

## Overview

Implement a small HTTP client that returns VND price per `luong`, with injectable endpoints/HTTP client for tests and runtime endpoint overrides for operational fallback.

## Requirements

- Functional: fetch spot XAU price in USD per troy ounce.
- Functional: fetch USD to VND exchange rate.
- Functional: convert to VND per `luong`.
- Functional: expose one method, `FetchLuongPrice(ctx) (float64, error)`.
- Functional: support runtime endpoint overrides for gold and FX URLs.
- Functional: cache FX response until `time_next_update_unix` when available; otherwise use a bounded fallback TTL.
- Non-functional: no API key required in default path; timeout bounded for Lambda.
- Non-functional: no global mutable client per request; reuse `http.Client` like trading does.
- Non-functional: reject non-HTTPS override URLs except localhost/127.0.0.1 test servers.

## Architecture

Create `internal/modules/gold/prices.go`:

```go
const gramsPerLuong = 37.5
const gramsPerTroyOunce = 31.1034768

priceVNDPerLuong := xauUSDPerTroyOunce * usdToVND * (gramsPerLuong / gramsPerTroyOunce)
```

Use two HTTP calls in v1. Keep each response struct minimal and defensive. FX can cache because ExchangeRate-API updates once daily; GoldPrice remains uncached in v1 unless latency proves painful.

## Related Code Files

- Create: `internal/modules/gold/prices.go`
- Create: `internal/modules/gold/prices_test.go`
- Modify: `cmd/server/main.go` if endpoint env vars are wired through config
- Modify: `template.yaml` only for optional env var pass-through, not default module enablement
- Read: `internal/modules/trading/prices.go`
- Read: `internal/modules/trading/income_events.go` for HTTPS URL validation pattern

## Implementation Steps

1. Add `GoldPriceClient` with `HTTP`, `GoldURL`, `FXURL`, `defaultOnce`, and `defaultClient`.
2. Add default URLs:
   - `https://data-asg.goldprice.org/dbXRates/USD`
   - `https://open.er-api.com/v6/latest/USD`
3. Add optional env/config plumbing for override URLs:
   - `GOLD_PRICE_API_URL`
   - `GOLD_FX_API_URL`
4. Validate override URLs:
   - remote URLs must be `https`
   - `http://localhost`, `http://127.0.0.1`, and `http://[::1]` allowed for tests/local dev only
5. Add bounded timeout, likely 10s to match trading.
6. Decode GoldPrice.org response:
   - require non-empty `items`
   - require `curr == "USD"` if present
   - require `xauPrice > 0`
7. Decode FX response:
   - require success result when field exists
   - require `rates.VND > 0`
   - read `time_next_update_unix` when present and cache until then
   - treat HTTP 429 as retryable upstream failure, not no-price
8. Return a domain error `ErrNoGoldPrice` for empty/invalid upstream data.
9. Wrap network/decode errors with `gold:` prefix.
10. Unit-test success conversion, cache behavior, 429 handling, HTTPS validation, localhost exception, and invalid response paths with `httptest.Server`.

## Success Criteria

- [x] `FetchLuongPrice` returns expected VND/luong value from fixtures.
- [x] Non-2xx, 429, malformed JSON, missing XAU, missing VND, and zero prices are covered.
- [x] FX cache uses `time_next_update_unix` when present and avoids repeated FX calls inside that window.
- [x] Runtime override URLs are validated and test-local HTTP URLs still work.
- [x] No API key or secret is required for default client construction.

## Risk Assessment

Two upstream calls increase latency and failure rate. Mitigation: cache FX by provider metadata, keep GoldPrice isolated behind a small client, and make provider URLs overrideable without source changes.
