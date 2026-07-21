---
title: Stock and Coin Cost Basis P&L
description: >-
  Persist weighted-average cost basis, migrate legacy holdings at startup
  prices, and expose realized/unrealized P&L.
status: completed
priority: P1
branch: main
tags:
  - stock
  - coin
  - mongodb
  - migration
  - pnl
blockedBy: []
blocks: []
created: '2026-07-21T08:08:21.155Z'
createdBy: 'ck:plan'
source: skill
---

# Stock and Coin Cost Basis P&L

## Overview

Add a per-symbol remaining cost basis to stock and coin portfolios. New buys
increase basis, partial sells remove proportional weighted-average basis, and
full sells remove it. Portfolio views show average entry price and unrealized
P&L per position; sell replies show realized P&L for that sale while preserving
the existing account-level P&L.

Legacy holdings migrate before handlers are installed. Each missing basis is
initialized from the current quote, so its unrealized P&L starts at zero.
Migration is idempotent, marker-backed, and fail-fast: the bot does not accept
trades if required quotes or storage writes fail.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Design persistence and migration](./phase-01-design-persistence-and-migration.md) | Completed |
| 2 | [Implement weighted-average accounting](./phase-02-implement-weighted-average-accounting.md) | Completed |
| 3 | [Verify P&L behavior and compatibility](./phase-03-verify-p-l-behavior-and-compatibility.md) | Completed |

## Dependencies

- No active plan dependencies. The completed stock-dividend implementation
  defines the share-dividend behavior this plan must preserve.
- MongoDB 8 Testcontainers are required for persisted migration verification;
  existing Docker-unavailable warning/skip behavior remains unchanged.

## Decisions

- Persist total remaining cost per symbol, not a mutable average-price field.
- Derive average price as `basis / held quantity`.
- Use proportional weighted-average basis for partial sells.
- Keep total stock basis unchanged when share dividends add quantity.
- Keep cash dividends and top-ups outside position basis.
- Do not persist cumulative realized P&L or trade lots.
- Run a module migration only when that module is loaded; disabled-module data
  migrates the next time the module is enabled.
- Abort startup on migration failure to avoid operating with unknown basis.

## Acceptance Criteria

- Stock and coin portfolios persist remaining cost basis without breaking
  legacy BSON/JSON decoding.
- Repeated buys produce the correct weighted average; partial/full sells update
  basis exactly and report realized gain or loss.
- Startup maintenance scans every boot, initializes only missing legacy basis
  from one complete current quote per symbol, never reprices completed work,
  and writes its system marker only after success.
- Share dividends add quantity without adding cost; cash dividends do not
  change position basis.
- Portfolio output shows average price and per-position unrealized P&L plus the
  existing account-level P&L.
- No command names or parameter contracts change.
- Focused tests, MongoDB migration tests, full tests, vet, build, and lint pass.

## Red Team Review

### Session — 2026-07-21

**Findings:** 12 deduplicated (9 accepted, 3 rejected)

| Finding | Severity | Disposition | Applied To |
|---|---|---|---|
| Completion marker cannot enforce future row invariants | Critical | Accept | Phases 1–2 |
| Missing quotes fabricate aggregate account P&L | Critical | Accept | Phase 3 |
| Migration has no overall deadline | High | Accept | Phase 1 |
| Partial stock quote maps can look successful | High | Accept | Phase 1 |
| Corrupt/noncanonical legacy symbols are ambiguous | High | Accept | Phase 1 |
| Migration conflicts lack retry semantics | Medium | Accept | Phase 1 |
| Coin quantity dust can orphan monetary basis | Medium | Accept | Phase 2 |
| Invalid basis can be silently normalized | Medium | Accept | Phases 1–2 |
| Stock output order and reply size are unsafe | Medium | Accept | Phase 3 |
| Storage-wide pagination and migration lease | High | Reject | Deadline and fail-closed runtime validation fit the one-replica deployment |
| Persist quote manifests and correction tooling | High | Reject | Outside paper-trading scope; validate complete finite quotes and log context |
| Restrict portfolio commands to private chats | Medium | Reject | Pre-existing visibility contract outside this accounting change |

### Whole-Plan Consistency Sweep

- Files reread: `plan.md` and all three phase files.
- Decision deltas checked: every-boot invariant scan, fail-closed runtime checks,
  bounded startup, quote completeness, corrupt-symbol rejection, CAS retry,
  dust cleanup, missing-price suppression, sorting, and reply bounds.
- Reconciled stale references: marker short-circuiting and partial aggregate P&L.
- Unresolved contradictions: 0.
