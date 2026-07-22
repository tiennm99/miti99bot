---
title: Per-User Stock Dividend History
description: >-
  Replace dividend cursors and the hashed applied ledger with per-user SSI
  dividend history, Record-date approval gating, refresh, and retention.
status: completed
priority: P1
branch: main
tags:
  - feature
  - stock
  - dividends
  - migration
  - telegram
blockedBy: []
blocks: []
created: '2026-07-22T13:56:06+07:00'
createdBy: 'ck:plan'
source: skill
---

# Per-User Stock Dividend History

## Overview

Persist normalized SSI events under
`portfolio.dividends.<ticker>.<ssi-event-id>`, independent of active positions.
Discover events from the recent 30-day publication window, refresh incomplete
historical records, repeat unprocessed notices on every portfolio request, and
require approval from Record date onward. Remove records after 90 days and migrate obsolete cursor and
hashed-ledger fields out of MongoDB.

Source: [approved brainstorm report](../reports/260722-1356-per-user-dividend-history-brainstorm.md).

## Delivery Mode

Test-first. Each phase adds or revises focused tests before changing financial
state or notification behavior.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Introduce Dividend History and Migration](./phase-01-introduce-dividend-history-and-migration.md) | Completed |
| 2 | [Synchronize, Refresh, and Retain SSI Events](./phase-02-synchronize-refresh-and-retain-events.md) | Completed |
| 3 | [Gate Notifications and Approval by Record Date](./phase-03-gate-notifications-and-approval.md) | Completed |
| 4 | [Integrate, Document, and Verify](./phase-04-integrate-document-and-verify.md) | Completed |

## Dependencies

- Phases are sequential because notification and callback behavior depend on
  the new persisted model and synchronization helpers.
- No cross-plan dependencies.
- SSI remains the existing provider; no new external dependency is required.
- MongoDB integration tests require Docker or `MONGODB_TEST_URL` and may skip
  explicitly when neither is available.

## Data Contract

- `assets.<ticker>` retains only quantity, basis, and `openedAt`.
- `dividends.<ticker>.<raw SSI ID>` contains normalized provider details plus
  local `processed` state.
- Pending actions reference ticker and SSI ID; they do not duplicate dividend
  financial fields.
- Entries expire 90 days after Record date, or after publication when Record
  date never becomes available.

## Boundaries

- Current quantity remains the calculation basis at approval time.
- The current asset must have opened on or before Record date.
- Manual dividend commands do not create or process SSI history entries.
- Portfolio output remains available when SSI discovery or refresh fails.
- No command registration or parameter changes.

## Definition of Done

- All phase success criteria pass.
- Focused stock and server tests pass, followed by `go test ./...` and
  `go vet ./...`.
- Changed Go files are formatted with `gofmt`.
- `golangci-lint run` passes when the binary is available.
- README and deployment storage documentation match the new persisted schema
  and Record-date behavior.
