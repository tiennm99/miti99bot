---
phase: 5
title: Tests and verification
status: completed
priority: P1
effort: 1.5h
dependencies:
  - 4
---

# Phase 5: Tests and verification

## Overview

Add focused coverage and run compile/test commands required for a safe module addition.

## Requirements

- Functional: test all user-visible command paths.
- Functional: test real module catalog wiring and module namespace isolation.
- Non-functional: no live network dependency in unit tests.
- Non-functional: no syntax errors; Go tests pass.

## Architecture

Tests should mirror `internal/modules/trading/*_test.go` and use:

- `httptest.Server` for price and FX clients.
- `internal/storage.NewMemoryKV()` or equivalent existing memory KV.
- `internal/testutil/recording_bot.go` for Telegram replies.
- Injected `nowFn` for deterministic metadata.

## Related Code Files

- Create/modify: `internal/modules/gold/*_test.go`
- Modify: `cmd/server/*_test.go` if needed for real `factories()` coverage
- Maybe modify: `internal/modules/validate_test.go` only if command validation expectations list commands explicitly

## Implementation Steps

1. Test price conversion and invalid upstream responses.
2. Test FX cache behavior, 429 handling, HTTPS override validation, and localhost override exception.
3. Test parser rejection for `NaN`, `Inf`, `+Inf`, `-Inf`, overflow inputs like `1e9999`, zero, and negatives.
4. Test portfolio first-load defaults, add/deduct, insufficient balance, dust cleanup, and save/load round trip.
5. Test full fractional sell after fractional buys leaves zero after dust normalization.
6. Test handlers:
   - topup usage and success
   - buy usage, success, insufficient VND, price failure
   - sell usage, success, insufficient luong, price failure
   - stats with holdings and stats with no price
7. Test module factory registers exact commands:
   - `gold_topup`
   - `gold_buy`
   - `gold_sell`
   - `gold_stats`
8. Test real composition-root wiring:
   - `factories()["gold"]` exists
   - `modules.Build([]string{"gold"}, factories(), ...)` succeeds
9. Test namespace isolation by enabling both `trading` and `gold` and verifying their `user:<id>` portfolio keys do not collide under module-prefixed KV storage.
10. Run:
   - `gofmt` on new/modified Go files
   - `go test ./internal/modules/gold`
   - `go test ./internal/modules ./cmd/server`
   - `go test ./...` before push
11. Do a self-review for file size; split handlers if any new code file exceeds 200 lines and logical extraction is clean.

## Success Criteria

- [x] All gold unit tests pass without network.
- [x] Existing module registry and server catalog tests pass.
- [x] Parser tests reject special float and overflow inputs.
- [x] Namespace isolation test proves trading and gold portfolios do not collide.
- [x] `go test ./...` passes locally.
- [x] Manual smoke syntax documented: `/gold_topup 10000000`, `/gold_buy 1`, `/gold_stats`, `/gold_sell 0.5`.

## Risk Assessment

Stats depends on external price source at runtime. Tests must verify graceful degradation so users still see cash/holding state if upstream is temporarily unavailable.
