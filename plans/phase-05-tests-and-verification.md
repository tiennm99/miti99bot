---
phase: 5
title: "Tests and verification"
status: completed
priority: P1
effort: "2h"
dependencies: [3, 4]
---

# Phase 5: Tests and verification

## Overview

Add unit tests for the new client and integration smoke tests for the composite fetcher, then run the full suite.

## Requirements

- Test VNAppMob key refresh, storage, expiry parsing, 403 retry, and SJC price parsing.
- Test composite fetcher fallback behavior.
- Ensure existing gold module tests still pass.

## Architecture

Create `internal/modules/gold/vnappmob_client_test.go`:
- `TestJWTExp` — valid, expired, malformed tokens.
- `TestRefreshKey` — mock refresh endpoint, verify KV storage.
- `TestFetchSJCPrice` — mock SJC endpoint, verify buy/sell.
- `TestFetchSJCPrice_403Refreshes` — first 403, refresh, second 200.
- `TestFetchSJCPrice_FallbackError` — on total failure, return `ErrNoGoldPrice`.

Create/update `internal/modules/gold/prices_test.go` or `composite_prices_test.go`:
- `TestCompositeFetcher_PrefersVNAppMob`
- `TestCompositeFetcher_FallsBack`

## Related Code Files

- Create: `internal/modules/gold/vnappmob_client_test.go`
- Modify: existing test files in `internal/modules/gold/`

## Implementation Steps

1. Write `vnappmob_client_test.go` using `httptest.Server`.
2. Add composite fetcher tests with stub `priceFetcher` implementations.
3. Run `make vet` and `make test`.
4. Run local server with `MODULES=gold` and hit `/gold_price` via Telegram or curl if possible.

## Success Criteria

- [x] All new tests pass.
- [x] `make test` passes.
- [x] `make vet` passes.
- [ ] Manual local smoke test returns SJC price.

## Risk Assessment

- **Risk**: External API tests are flaky. **Mitigation**: use `httptest` for all unit tests; external calls only in manual smoke test.
