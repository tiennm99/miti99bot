---
title: Stock Dividend Commands
description: >-
  Replace ambiguous stock adjustments with cash-only, share-only, and combined
  dividend commands while preserving historical stats.
status: completed
priority: P1
branch: main
tags:
  - feature
  - backend
  - database
blockedBy: []
blocks: []
created: '2026-07-20T09:16:13.694Z'
createdBy: 'ck:plan'
source: skill
---

# Stock Dividend Commands

## Overview

Add three explicit public contracts: cash dividend in VND/share, share dividend
in `owned:new` ratio form, and a combined command. Use integer floor rounding,
pre-event holdings, and one portfolio save. Rename historical stats through an
idempotent startup migration before `/stock_dividend` gains its new meaning.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Implement Ratio Dividend Commands](./phase-01-implement-ratio-dividend-commands.md) | Completed |
| 2 | [Migrate Command Statistics](./phase-02-migrate-command-statistics.md) | Completed |
| 3 | [Verify User Contracts](./phase-03-verify-user-contracts.md) | Completed |

## Dependencies

- Approved design: [Stock Dividend Command Brainstorm](../reports/260720-1612-stock-dividend-command-brainstorm.md)
- Evidence: [Dividend Notice Ratio Research](../reports/260720-1608-dividend-notice-ratio-research.md)
- Existing Go module, typed storage, `system` marker store, and stats startup path

## Contracts

- `/stock_cash_dividend <vnd_per_share> <TICKER>` credits a positive whole-VND amount per share.
- `/stock_share_dividend <owned:new> <TICKER>` accepts any positive integer ratio, preserves its entered form in replies, and adds floor-rounded shares; zero result rejects.
- `/stock_dividend <vnd_per_share> <owned:new> <TICKER>` applies both from pre-event holdings; zero shares still credits cash.
- Stats move `stock_dividend -> stock_cash_dividend` and `stock_bonus -> stock_share_dividend`; combined usage starts fresh.
- Commands are deliberate manual adjustments: repeated calls are allowed and
  no issuer-event deduplication or ratio reduction is imposed.

## Validation Gate

- Focused stock, stats migration, startup, and command-menu tests.
- `gofmt` changed Go files; run `go test ./...` and `go vet ./...`.
- Run `golangci-lint run` when installed.

## Completion Criteria

- Commands, replies, menu JSON, and README agree on names and argument order.
- Ratio math cannot overflow and always floors; combined state persists once.
- Stats migration merges anonymous/per-user targets and repeated startup is a no-op.
- No unresolved questions.

## Validation Log

### Session 1 — 2026-07-20

**Trigger:** User requested `/ck:plan validate` before implementation.
**Questions asked:** 4

### Verification Results

- **Tier:** Standard (Fact Checker + Contract Verifier)
- **Claims checked:** 30
- **Verified:** 30 | **Failed:** 0 | **Unverified:** 0
- Verified current handlers/registry/portfolio storage, stats startup caller and
  persisted row shape, shared system marker store, command-menu flow, tests,
  README/JSON surfaces, and repository verification commands.
- Planned create-paths correctly do not exist yet: `dividends.go`,
  `dividends_test.go`, and stats `startup_test.go`.
- **Failures:** None.

#### Questions & Answers

1. **[Assumption]** Should cash dividends require a positive whole VND amount per share and reject decimals?
   - Options: Require positive whole VND (Recommended) | Preserve positive finite decimal input
   - **Answer:** Require positive whole VND.
   - **Rationale:** Matches approved manual-entry contract and avoids fractional VND credits.
2. **[Architecture]** Should share ratios accept any positive integer form and preserve the entered form without requiring reduction?
   - Options: Accept and preserve any positive ratio (Recommended) | Require reduced ratios | Normalize replies
   - **Answer:** Accept and preserve any positive ratio.
   - **Rationale:** Mirrors issuer notices while avoiding unnecessary restrictions.
3. **[Scope]** Should repeated dividend commands be allowed without an issuer-event deduplication ledger?
   - Options: Allow repeated manual adjustments (Recommended) | Add event IDs and deduplication
   - **Answer:** Allow repeated manual adjustments; caller owns correctness.
   - **Custom input:** "this is for user do manually, so not restrict anything, user call it will have reponsibility to make sure call it correct"
   - **Rationale:** The feature is a manual portfolio adjustment, not an issuer-event ledger.
4. **[Risk]** Should migration retries scan prepared row checkpoints and retain all migration markers permanently?
   - Options: Retry prepared rows and retain markers (Recommended) | Use global marker only | Clean row markers after completion
   - **Answer:** Retry prepared rows and retain markers permanently.
   - **Rationale:** Makes partial retries auditable and prevents double-counting.

#### Confirmed Decisions

- Manual responsibility does not bypass storage-safety validation: cash remains
  positive whole VND; ratio parts remain positive integers; ticker and overflow
  checks remain mandatory.
- No ratio canonicalization, notice lookup, event ID, or duplicate-call guard.
- Migration completion requires reconciling both source rows and prepared row
  checkpoints; global and row markers remain as history.

#### Action Items

- [x] Propagate manual-input and repeat-call rules to Phase 1.
- [x] Propagate checkpoint retry/retention rules to Phase 2.
- [x] Propagate user-facing responsibility wording to Phase 3.

#### Impact on Phases

- Phase 1: tighten cash parsing; preserve ratio text; explicitly allow repeats.
- Phase 2: scan prepared checkpoints on retry; retain all markers.
- Phase 3: document manual responsibility and safety-validation boundary.

### Whole-Plan Consistency Sweep

- Files reread: `plan.md` and all three `phase-*.md` files.
- Decision deltas checked: 4.
- Reconciled stale references: 1 (`float acceptance` replaced by integer-input wording).
- Verified command names, input order, whole-VND rule, unreduced-ratio policy,
  repeat-call policy, zero-share behavior, migration retry flow, and marker
  retention agree across overview, requirements, steps, risks, and criteria.
- Unresolved contradictions: 0.
