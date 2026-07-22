---
phase: 3
title: Gate Notifications and Approval by Record Date
status: completed
priority: P1
dependencies:
  - phase-02-synchronize-refresh-and-retain-events
effort: large
---

# Phase 3: Gate Notifications and Approval by Record Date

## Overview

Split future notices from actionable suggestions and make repeated notifications
safe through idempotent per-user dividend processing.

## Requirements

- Send one informational message before Record date after every portfolio request.
- Do not expose approval while Record date is missing.
- At or after the start of Record date in Asia/Saigon, send one separate
  actionable message per eligible event after every portfolio request.
- Require a current position whose `openedAt` is no later than Record date.
- Calculate from current quantity and atomically mark the stored event processed.
- Store only event references and Telegram security context in pending actions.

## Related Code Files

- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_notifications.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_callback.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/pending_dividend.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_flow_test.go`

## Implementation Steps

1. Add failing tests for before/on/after Record date, missing Record date,
   repeated future notices, failed-send retries, multiple event messages, and
   multiple pending buttons guarded by processed-state idempotency.
2. Add callback tests for owner/chat/message binding, sold positions,
   post–Record date rebuys, current-quantity calculations, double clicks,
   concurrent calls, save failures, and already processed history.
3. Split rendering into informational and actionable event messages while
   retaining source dates and the SSI disclaimer.
4. Reload under the per-user lock before every delivery so processed events stop
   immediately while unprocessed events repeat on every request.
5. Reduce `PendingDividendAction` to owner/chat/message/token timing plus ticker,
   SSI ID, and current-position lifecycle binding.
6. During callback, load the authoritative persisted dividend, revalidate Record
   date and eligibility, apply cash/share changes, and set `processed = true` in
   the same portfolio save.
7. Keep cleanup, callback acknowledgement, and keyboard removal best-effort
   after the financial save, preserving safe retry behavior on save failure.

## Success Criteria

- [x] Future events have no approval control and notify after every portfolio
      request until processed or expired.
- [x] Missing Record dates never become actionable.
- [x] Every due event receives its own message and button.
- [x] A post–Record date position lifecycle cannot claim an old event.
- [x] Financial values come from portfolio dividend history, not pending data.
- [x] Successful processing and `processed = true` are atomic and idempotent.
- [x] Multiple or recreated pending actions remain safe because only the first
      successful callback can process the event.

## Risk Assessment

Telegram delivery and portfolio persistence cannot be one transaction. Preserve
the existing conservative sequence, recheck state under the user lock, and test
ambiguous failure paths. Opaque random tokens and exact owner/chat/message
binding remain mandatory.
