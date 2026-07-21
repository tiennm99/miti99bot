---
type: project-status-report
topic: stock-dividend-event-buttons
reported_at: 2026-07-21T18:15:00+07:00
status: completed
---

# Stock Dividend Event Buttons Completion

## Summary

`/stock_portfolio` now checks SSI dividend events for held tickers after sending
the portfolio. Relevant events receive owner-bound, expiring Apply buttons;
successful checks with no events remain silent.

## Completed

- [x] Replaceable SSI provider with pagination, local classification, exact
  share ratios, event-ID deduplication, and bounded date overlap.
- [x] Per-ticker cursor checks that preserve concurrent portfolio writes.
- [x] Opaque 24-hour pending actions bound to caller, chat, and message.
- [x] Current-holding cash/share application with replay-safe event ledger.
- [x] Position lifecycle binding that rejects buttons after full sale/reopen.
- [x] Shared callback registration, visibility validation, and authorization.
- [x] README and regression tests updated.

## Verification

| Gate | Result |
|---|---|
| Focused stock/modules tests | Passed |
| Full `go test -count=1 ./...` | Passed |
| Stock race and concurrency stress tests | Passed |
| `go vet ./...` | Passed |
| `golangci-lint run` | Passed, 0 issues |
| `go build ./...` | Passed |
| Independent final review | Passed, no blockers |

## Plan Sync

No active phase plan maps to this incremental request. Existing stock-dividend
and cost-basis plans were already completed before this work and remain
unchanged. Session task tracking is fully complete.

## Remaining Risks

- SSI iBoard is undocumented and best-effort.
- SSI publication timestamps are day-granularity; a bounded overlap prevents
  same-day loss, while pending actions and applied IDs suppress duplicates.
- The portfolio has no dated-lot ledger, so acceptance uses holdings at click
  time rather than legal record-date entitlement.
- Manual dividend commands cannot attach SSI event IDs; users must not apply
  the same event both manually and through its button.

## Next Step

Commit only after explicit user approval.
