---
type: technical-journal
topic: stock-dividend-event-buttons
conducted_at: 2026-07-21T18:15:00+07:00
status: complete
---

# Stock Dividend Event Buttons Journal

## Context

`/stock_portfolio` needed to surface recent dividend events for held tickers
without obscuring the portfolio or silently changing financial state. SSI
iBoard was selected as a best-effort discovery source, while the existing
manual dividend commands remain available.

## What Happened

- The portfolio response is sent first. Each discovered cash or explicit share
  dividend follows in its own message with an `Apply dividend` button; a ticker
  with no relevant event produces no extra message.
- Added a replaceable SSI provider with pagination, local event classification,
  stable event IDs, deduplication, and exact ratio handling.
- Added callback registration to the shared module framework and persisted
  short-lived pending actions separately from portfolios.
- Successful checks advance `dividendCheckedAt`; failed checks leave the cursor
  unchanged and never prevent the portfolio from being displayed.
- Added `openedAt` to distinguish the holding lifecycle and retained applied SSI
  event identities for auditability and replay protection.

SSI discovery accepts only day-granularity date windows. Queries therefore use
a cursor overlap at the lower boundary, then locally filter and deduplicate the
returned events. This avoids missing events around a date boundary while stable
SSI event IDs prevent overlap from producing repeated actionable suggestions.

## Key Decisions

- Callback data carries an opaque random token, not trusted dividend values.
  Pending actions expire after 24 hours and are bound to the requesting
  Telegram user, originating chat, and event message.
- A callback reloads the current portfolio and calculates from the holding at
  click time. Another group member, an expired action, or a holding sold and
  reopened after suggestion creation cannot apply it.
- Applied SSI event IDs are recorded atomically with the portfolio mutation, so
  duplicate clicks and retries cannot apply the same provider event twice.
- Event fetches occur outside the per-user mutation lock. Cursor persistence
  reloads and merges under that lock, preserving concurrent buys, sells, and
  manual dividends without holding a lock across network calls.
- Notification or provider failures do not advance the affected cursor. A later
  portfolio request can safely retry rather than lose an event.

## Verification

- Focused stock and module tests passed.
- Full suite passed: `go test -count=1 ./...`.
- Stock race tests and 20-iteration concurrency stress tests passed.
- `go vet ./...` and `go build ./...` passed.
- `golangci-lint run` completed with zero issues.
- Independent testing, debugging, and code review found no blockers.

## Risks and Limitations

- SSI iBoard is undocumented and has no published stability or availability
  guarantee; the provider can require replacement if its contract changes.
- The portfolio has no dated transaction lots, so current quantity is not proof
  of record-date entitlement. Users must verify the issuer notice.
- Manual commands do not record an SSI event ID. Applying manually and later
  accepting the related button can count the same dividend twice.

## Next

Observe SSI response stability and callback behavior in real use. Commit these
changes only if the user requests it.
