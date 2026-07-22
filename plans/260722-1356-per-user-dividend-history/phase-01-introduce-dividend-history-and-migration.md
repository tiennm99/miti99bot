---
phase: 1
title: Introduce Dividend History and Migration
status: completed
priority: P1
dependencies: []
effort: large
---

# Phase 1: Introduce Dividend History and Migration

## Overview

Replace cursor and hashed-ledger persistence with a validated per-user dividend
history, simplify asset lifecycle state, and physically remove obsolete MongoDB
fields through an idempotent startup migration.

## Requirements

- Add a persisted dividend record containing normalized kind, provider dates,
  amount or share ratio, display details, and `processed`.
- Store records as `Portfolio.Dividends[ticker][rawSSIEventID]`.
- Remove `AssetPosition.DividendCheckedAt` and
  `Portfolio.AppliedDividendEvents` from the runtime schema.
- Preserve `openedAt` when buying more and reset it only for a newly opened
  position.
- Make manual dividend application independent of discovery state.
- Rewrite legacy user documents once and mark migration completion through
  `internal/systemstate`.

## Related Code Files

- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/portfolio.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/portfolio_test.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/handlers.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/handlers_test.go`
- Create: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/startup.go`
- Create: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/startup_test.go`
- Create: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/startup_mongo_test.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/cmd/server/main.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/cmd/server/main_test.go`

## Implementation Steps

1. Add failing table-driven tests for dividend-map initialization, validation,
   storage round trips, buy/partial-sell/full-sell independence, and manual
   dividend behavior without a cursor.
2. Define `DividendRecord` and nested history maps using existing JSON/BSON
   naming conventions and raw provider-ID validation.
3. Update portfolio load, creation, validation, and financial mutation helpers;
   remove obsolete cursor error language from manual handlers.
4. Add migration fixtures containing both obsolete fields and pending-action
   documents; ensure only `user:` portfolios are rewritten.
5. Implement conflict-aware migration of stock user documents, dropping
   `dividendCheckedAt` and `appliedDividendEvents`, initializing the new map, and
   recording an idempotent system-state marker with migrated count.
6. Call stock startup maintenance from `cmd/server/main.go` before module build
   and cover invocation/error propagation at the server boundary.

## Success Criteria

- [x] New and legacy portfolios load with initialized dividend history.
- [x] Assets no longer persist or validate `dividendCheckedAt`.
- [x] Manual dividends change only shares/cash and basis-related portfolio data.
- [x] The legacy hashed ledger is deliberately discarded as approved.
- [x] Startup migration is idempotent, retries version conflicts safely, skips
      pending-action keys, and records completion in `system`.
- [x] A MongoDB fixture proves obsolete BSON fields are physically absent after
      migration.

## Risk Assessment

The migration touches every stock user document and legacy duplicate history is
intentionally removed. Limit the scan to `user:`, validate rewritten documents,
use version-aware writes, and set the completion marker only after the entire
scan succeeds. Never log document contents.
