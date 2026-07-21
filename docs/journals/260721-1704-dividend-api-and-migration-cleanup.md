---
type: technical-journal
topic: dividend-api-and-migration-cleanup
conducted_at: 2026-07-21T17:04:00+07:00
status: complete
---

# Dividend API and Migration Cleanup Journal

## Context

Portfolio schema migrations were verified complete in production, so their
compatibility code had become permanent startup and maintenance overhead. Coin
positions also inherited a stock-only dividend cursor with no valid use.
Separately, research was needed before designing automatic discovery of
Vietnamese stock dividend events.

## What Happened

- Removed `dividendCheckedAt` from coin positions, validation, buy behavior,
  and tests. Stock retains the cursor because dividend events apply there.
- Retired completed stock and coin portfolio migrations plus the completed
  stats command-renaming migration and their migration-only tests.
- Preserved recurring startup maintenance: stats query indexes and the LoL
  match-cache TTL index remain idempotently initialized and tested.
- Preserved historical migration records in MongoDB's `system` collection and
  kept the reusable `internal/systemstate` helper for future migrations.
- Removed legacy numeric-position JSON/BSON decoders after production schema
  completion was confirmed. Current models now describe only the supported
  nested asset schema.
- Accepted lazy cleanup of stale coin cursor fields. BSON decoding ignores the
  old unknown field, and the next portfolio mutation uses versioned
  `ReplaceOne`, rewriting that portfolio in the current shape without a new
  one-time migration.

## Dividend API Research

SSI iBoard's corporate-actions endpoint is the recommended initial source. It
currently returns anonymous JSON with ticker/date filters, pagination, stable
`CorId` values, cash amounts, ratios, and event dates. It covered verified cash
and share dividend examples.

The endpoint is undocumented and has no published SLA, rate limit, or stability
contract. Any implementation should therefore isolate it behind a replaceable
provider, validate and locally classify events, deduplicate by `CorId`, and ask
the user to confirm before changing a portfolio. VSDC remains the authoritative
notice source; a licensed FiinGroup feed is the stronger future option if this
becomes production-critical.

## Decisions

- Keep dividend state stock-only; coin assets persist only `quantity` and
  total remaining `base`.
- Remove completed one-time runtime code rather than continuing to scan already
  migrated portfolios on every boot.
- Do not delete system history or general migration infrastructure.
- Do not introduce a cleanup migration solely for stale coin BSON fields;
  normal writes remove them safely over time.
- Treat SSI iBoard as a replaceable prototype provider, not a guaranteed public
  API contract.

## Verification

- Passed focused tests:
  `go test -count=1 ./internal/modules/coin ./internal/modules/stock ./internal/modules/stats ./cmd/server`
- Passed full suite: `go test -count=1 ./...`
- MongoDB 8 tests executed and passed for stats indexes and the LoL TTL index.
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Passed with zero issues: `golangci-lint run`
- Passed: `git diff --check` (expected LF/CRLF working-copy warnings only).
- Independent tester, debugger, and reviewer found no defects.

## Reflection

Migration code is operationally valuable only while incompatible data can
still exist. Removing it after verification narrows startup failure modes and
makes the active persistence contract explicit. Lazy removal is appropriate
for an ignored field because it does not affect reads or correctness; structural
schema changes still require guarded migrations.

## Next

Design the Telegram interaction for listing SSI dividend events, including date
windows, pagination, exact ratio conversion, ambiguity handling, and explicit
user confirmation. Preserve manual dividend commands and keep automatic event
application out of scope until that interaction is approved.
