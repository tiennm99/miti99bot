# Stock Dividend Commands Journal

## Context

Implemented the validated stock-dividend plan: clearer manual commands,
ratio-based share entitlement, and historical stats preservation.

## What Changed

- Added `/stock_cash_dividend <vnd_per_share> <TICKER>` for positive whole-VND
  cash credits.
- Added `/stock_share_dividend <owned:new> <TICKER>` with unreduced ratio
  preservation and floor-rounded whole shares.
- Changed `/stock_dividend <vnd_per_share> <owned:new> <TICKER>` to apply cash
  and shares from the same pre-event holding with one portfolio save.
- Share-only zero entitlement rejects with the minimum holding; combined events
  still credit cash when shares round to zero.
- Updated registration, command menu, usage text, README, and contract tests;
  removed `/stock_bonus` from active commands.

## Stats Migration

- Migrates `stock_dividend -> stock_cash_dividend` and
  `stock_bonus -> stock_share_dividend`, including anonymous and per-user rows.
- Merges existing target counts without loss.
- Permanent global and per-row markers retain migration history.
- Each row stores a `prepared` exact target count before mutation. Retries set
  that count after any write-boundary failure, preventing double increments.

## Review Findings

- Guarded `strconv.ParseInt` range errors so its saturated return value cannot be
  accepted as valid input.
- Added exact `int64` share formatting so quantities above `2^53` do not lose
  digits through `float64` conversion.
- Enforced exact float-backed VND balance addition at the `2^53` boundary;
  inexact sums reject without saving.
- Reviewer: 9/10, approve. Adversarial review: PASS; no disproven claims or
  reachable regressions.

## Decisions

- Ratios remain positive integer `owned:new` values and are echoed as entered;
  no normalization.
- Commands are manual adjustments. Repeated calls remain allowed, with no event
  ledger or duplicate-call guard; caller owns correctness.
- Overflow, exactness, ticker, and syntax validation remain mandatory despite
  the manual workflow.

## Verification

- Passed: `go test ./internal/modules/stock`
- Passed: `go test ./internal/modules/stats`
- Passed: `go test ./cmd/server`
- Passed: `go test ./...`
- Passed: `go vet ./...`
- Passed: `telegram-commands.json` PowerShell `ConvertFrom-Json`
- Passed: `git diff --check` (line-ending warnings only)
- Skipped: Mongo-backed tests because `MONGODB_TEST_URL` was unset.
- Skipped: `golangci-lint run` because the binary was not installed.

## Next Considerations

- Run Mongo-backed migration tests when a test database is available.
- Run the lint gate when `golangci-lint` is installed.
- After deployment, verify migrated counts and retained system markers before
  considering migration-runtime cleanup; keep historical stats and markers.
