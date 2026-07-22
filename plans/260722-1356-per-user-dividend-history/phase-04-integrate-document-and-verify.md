---
phase: 4
title: Integrate, Document, and Verify
status: completed
priority: P1
dependencies:
  - phase-03-gate-notifications-and-approval
effort: medium
---

# Phase 4: Integrate, Document, and Verify

## Overview

Complete handler integration, align user and operations documentation, and run
the repository's full quality gates.

## Requirements

- `/stock_portfolio` sends the portfolio first and then runs history sync and
  notifications.
- User-facing errors no longer refer to dividend checkpoints.
- Documentation describes the new schema, Record-date gate, refresh policy,
  eligibility approximation, pending lifetime, and retention.
- All focused and repository-wide Go checks pass.

## Related Code Files

- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/handlers.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/handlers_test.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividends_test.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/README.md`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/docs/deploy-coolify-selfhosted.md`

## Implementation Steps

1. Add integration assertions that the portfolio is sent before SSI messages
   and remains available during discovery or refresh errors.
2. Update handler error text and remove obsolete cursor expectations from
   manual dividend and portfolio tests.
3. Document per-user history, exact recent discovery, historical missing-date
   refresh, repeated unprocessed notices, Record-date approval, current-holding
   approximation, and 90-day cleanup.
4. Run `gofmt` on all changed Go files.
5. Run focused tests for `internal/modules/stock` and `cmd/server`.
6. Run `go test ./...`, `go vet ./...`, and `go build ./...`.
7. Run `golangci-lint run` when available and resolve all in-scope findings.

## Success Criteria

- [x] `/stock_portfolio` preserves portfolio-first and best-effort SSI behavior.
- [x] README and deployment docs contain no cursor or hashed-ledger claims.
- [x] Focused stock/server tests pass.
- [x] Full tests, vet, and build pass.
- [x] CI lint gate passes when locally available.
- [x] `git diff --check` reports no whitespace errors.

## Risk Assessment

The main integration risk is documentation or older tests silently preserving
the cursor-era contract. Search the full repository for `dividendCheckedAt`,
`AppliedDividendEvents`, `appliedDividendEvents`, and checkpoint wording before
declaring completion.
