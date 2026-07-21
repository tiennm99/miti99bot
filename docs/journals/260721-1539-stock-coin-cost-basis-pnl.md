# Stock and Coin Cost Basis P&L Journal

## Context

Stock and coin portfolios tracked account funding and holdings but not the
remaining acquisition cost of each position, so they could not distinguish
realized sale results from unrealized open-position performance.

## What Changed

- Added persisted per-symbol total remaining `costBasis` to stock and coin
  portfolios, with invariant checks on load, update, and save paths.
- Buys add actual spend; partial sells remove proportional weighted-average
  basis and report realized P&L; full sells remove the position and its basis.
- Portfolio views now show average entry price and per-position unrealized P&L.
  Account P&L remains total account value minus top-ups and is withheld when
  missing quotes make valuation partial.
- Stock share dividends preserve total basis while increasing quantity; cash
  dividends and top-ups remain outside position basis.
- Sorted, bounded portfolio replies retain summaries while omitting excess
  position lines before Telegram's message limit.

## Migration Safety

- Enabled modules scan every portfolio before handlers are installed on every
  boot, even when a completion marker already exists.
- Missing legacy basis is initialized from a complete current quote set, giving
  each migrated position zero initial unrealized P&L without repricing rows that
  already have basis.
- Versioned writes retry conflicts; the shared `system` marker is written only
  after all rows succeed and remains an audit record rather than a scan bypass.
- A two-minute overall deadline bounds startup. Invalid data, noncanonical
  symbols, missing quotes, exhausted conflicts, or storage failures abort
  startup instead of allowing trades with unknown basis.

## Decisions

- Persist total remaining cost, deriving average entry price as basis divided by
  held quantity; do not persist trade lots or cumulative realized P&L.
- Migrate only loaded modules, so disabled-module data waits until re-enabled.
- Preserve all command names, parameter contracts, and existing portfolio data.

## Verification

- Passed: focused stock and coin accounting, migration, output, and reply-budget
  tests.
- Passed: MongoDB 8 Testcontainers migration and idempotency coverage.
- Passed: `go test ./...`
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Passed: `golangci-lint run`

## Operational Impact

- Deployments with legacy holdings may perform market-price lookups and writes
  during startup; a required provider or database failure intentionally keeps
  the bot offline until initialization can complete safely.
- Subsequent healthy boots still verify invariants but do not reprice completed
  positions.
