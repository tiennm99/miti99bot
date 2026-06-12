---
phase: 4
title: Tests and verification
status: completed
priority: P1
effort: 2-3h
dependencies:
  - 1
  - 2
  - 3
---

# Phase 4: Tests and verification

## Context Links

- Existing tests: `internal/modules/gold/*_test.go`, `internal/modules/trading/*_test.go`
- Module registry tests: `internal/modules/registry_test.go`, `cmd/server` tests if present
- Commands: `make test`, `make vet`

## Overview

Add focused unit tests and run compile/test gates. Tests must not call real Binance, Coinbase, or CoinGecko.

## Key Insights

- Provider tests should use `httptest.Server` or fake `RoundTripper` like existing modules.
- Handler tests should inject fake price fetchers, not rely on provider chain.
- Fallback behavior is the highest-risk logic; test it explicitly.

## Requirements

- Functional: unit tests cover provider decoding, fallback, cache, portfolio mutation, handlers, and registration.
- Non-functional: no network in tests, deterministic time where needed, no flaky provider timing.

## Architecture

```text
coin tests
  provider fixtures -> prices.go / price_providers.go
  portfolio tests  -> portfolio.go
  handler tests    -> fake priceFetcher + memory KV
  registration     -> factories/build if practical
```

## Related Code Files

- Create: `internal/modules/coin/prices_test.go`
- Create: `internal/modules/coin/portfolio_test.go`
- Create: `internal/modules/coin/handlers_test.go`
- Modify/create: `cmd/server/main_test.go` if factory coverage is missing
- Modify: existing docs only if verification changes behavior

## Implementation Steps

1. Add provider tests:
   - Binance success parses `price` string.
   - Binance non-2xx/429 falls back.
   - Coinbase success parses `data.rates.USD`.
   - CoinGecko success parses `{id}.usd`.
   - all providers fail -> `ErrNoCoinPrice`.
2. Add cache tests:
   - repeated fetch within TTL avoids second provider call.
   - expired cache refetches.
3. Add portfolio tests:
   - new user initializes USD 0 and empty assets.
   - topup increments USD and invested.
   - buy/sell math and dust cleanup.
   - insufficient USD/asset returns current balance.
4. Add handler tests:
   - usage errors.
   - unknown coin.
   - topup success.
   - buy success and insufficient USD.
   - sell success and insufficient coin.
   - stats empty and stats with price failure partial display.
5. Add registration test if current test structure supports it.
6. Run verification commands:
   - `go test ./internal/modules/coin`
   - `go test ./internal/modules/... ./cmd/server`
   - `go test ./...`
   - `go vet ./...` or `make vet`

## Todo List

- [x] Provider tests added.
- [x] Cache tests added.
- [x] Portfolio tests added.
- [x] Handler tests added.
- [x] Registration/docs verification added.
- [x] Compile/test commands run and results recorded.

## Success Criteria

- [x] All coin tests pass without network.
- [x] Provider fallback test proves order Binance -> Coinbase -> CoinGecko.
- [x] Handler tests prove portfolio not mutated on price failure.
- [x] Broader module/server tests pass.
- [x] README/template docs match actual default module behavior.

## Risk Assessment

Main risk: passing unit tests with fake providers while real endpoints have shape drift. Mitigation: keep provider decoders strict but small, include env URL overrides for quick hotfix, and make runtime error messages graceful.

## Security Considerations

Tests must not require real API keys or real network. Do not add dotenv or credentials. Avoid fixtures with sensitive data.

## Next Steps

After this phase, implementation is ready for code review and optional release decision on default enablement.

## Verification Results

- `go test ./internal/modules/coin` passed.
- `go test ./...` passed.
- `go vet ./...` passed.
- `sam validate` not run: SAM CLI is not installed in this environment.
