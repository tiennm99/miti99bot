---
phase: 2
title: Synchronize, Refresh, and Retain SSI Events
status: completed
priority: P1
dependencies:
  - phase-01-introduce-dividend-history-and-migration
effort: large
---

# Phase 2: Synchronize, Refresh, and Retain SSI Events

## Overview

Build deterministic synchronization around the rolling 30-day publication
window, targeted historical refreshes, provider corrections, and 90-day cleanup.

## Requirements

- Fetch an exact `[now-30d, now]` publication window for currently held tickers.
- Upsert by ticker and raw SSI ID while preserving local processing state.
- Re-fetch missing-Record-date events from their stored publication-day window.
- Refresh an event before it becomes actionable.
- Delete history 90 days after Record date, or after publication when Record
  date remains missing.
- Continue displaying the portfolio and preserve stored data on provider errors.

## Related Code Files

- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_events.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_events_ssi.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_events_ssi_test.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_notifications.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_flow_test.go`
- Create: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_history.go`
- Create: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/dividend_history_test.go`

## Implementation Steps

1. Add failing tests for exact boundaries, overlapping SSI day queries, merge
   semantics, corrections, missing-date refresh, grouped refresh requests,
   partial failures, and both retention clocks.
2. Isolate conversion between `DividendEvent` and the persisted record so raw
   SSI data is normalized once and the local processed field cannot be overwritten.
3. Make the caller enforce the exact 30-day `PublishedAt` window after the SSI
   adapter's intentional prior-day overlap.
4. Group incomplete refreshes by ticker and original publication day, fetch
   bounded windows, and update only matching raw event IDs.
5. Refresh due events before action creation when possible; if SSI omits the
   matching event, keep using the retained record until processing or expiry.
6. Prune expired records using Asia/Saigon date boundaries and remove empty
   ticker maps before saving.
7. Keep fetch concurrency bounded and preserve the current behavior where one
   ticker failure does not suppress successful results for other tickers.

## Success Criteria

- [x] Events outside the exact 30-day publication interval are not discovered.
- [x] Repeated fetches are idempotent and preserve `processed`.
- [x] Missing Record dates can be filled after the event leaves the recent feed.
- [x] SSI corrections replace unprocessed provider details before approval.
- [x] Processed state is never reverted by a provider refresh.
- [x] Record-dated and permanently incomplete events are deleted on their
      respective 90-day boundaries.
- [x] Provider failures do not delete history or block portfolio rendering.

## Risk Assessment

Historical refreshes can amplify SSI traffic. Group requests by ticker/day,
reuse existing pagination limits and timeouts, and refresh only incomplete or
newly due unprocessed records. Boundary tests must use the Asia/Saigon location
to avoid UTC-dependent behavior.
