---
type: brainstorm-report
topic: per-user-dividend-history
conducted_at: 2026-07-22T13:56:06+07:00
status: approved
---

# Brainstorm Report: Per-User Dividend History

## Problem

Stock dividend discovery currently depends on `assets.<ticker>.dividendCheckedAt`
and a global hashed `appliedDividendEvents` ledger. That model offers SSI events
immediately, loses the discovery cursor on a full exit, duplicates trusted event
data in pending buttons, and cannot defer approval reliably until Record date.

The desired behavior is to check SSI when `/stock_portfolio` runs, notify users
about future events, expose approval only from Record date, retain enough event
data to refresh incomplete notices, and remove old history after 90 days.

## Constraints Discovered

- SSI queries are filtered by publication date, not Record date. An event can
  leave a rolling 30-day feed before its Record date arrives.
- SSI `corId` lookup is not reliable enough to make ID-only persistence safe.
- Full sales delete `assets.<ticker>`, so nesting processed history there would
  allow the same event to be rediscovered after a sell and rebuy.
- Existing applied-event keys are one-way hashes without ticker identity and
  cannot be losslessly converted to raw SSI IDs.
- The bot does not persist historical lots. Eligibility can only be bounded by
  the current position and its `openedAt` lifecycle marker.

## Approaches Considered

### Per-user dividend history

Persist full normalized SSI details under
`portfolio.dividends.<ticker>.<ssi-event-id>`, separate from active assets.
This keeps approval self-contained, survives full exits, and gives every user
an explicit notification and processing lifecycle.

### Global event cache with per-user references

Normalize SSI events once globally and keep only user state in each portfolio.
This reduces duplicated provider data but adds cross-document consistency,
retention, and callback failure modes that are unnecessary at current scale.

### Event IDs only with SSI refetch

Persist only IDs and retrieve financial details at approval time. This is the
smallest schema but is unsafe because old publication windows move out of the
recent feed and SSI does not provide dependable lookup by ID.

## Approved Design

Use per-user dividend history:

```text
portfolio
|- assets.<ticker>
|  |- quantity
|  |- base
|  `- openedAt
`- dividends.<ticker>.<ssi-event-id>
   |- normalized SSI kind, dates, amount/ratio, title, and source URL
   `- processed
```

The SSI event ID is the dynamic map key rather than a duplicated record field.
Raw IDs are already constrained to BSON-safe characters by the SSI adapter.

## Discovery and Refresh Flow

On `/stock_portfolio`:

1. Render the portfolio even if SSI later fails.
2. Fetch events published in the exact preceding 30 days for held tickers.
3. Upsert normalized details while preserving local notification and processed
   state.
4. Re-query the original publication-date window for stored events whose Record
   date is missing. Before creating an actionable message, refresh the event
   once more so SSI corrections are captured.
5. Send an informational message for every future unprocessed event after each
   portfolio request.
6. At or after the start of Record date in Asia/Saigon, send a separate approval
   message for every unprocessed eligible event after each portfolio request.

An event with no Record date remains informational-only until SSI fills the
date. Re-fetches match the original ticker and raw SSI event ID.

## Approval and Idempotency

Pending actions retain only the opaque token bindings and references needed to
locate the user dividend record. Financial values are read from the trusted
portfolio record during callback handling.

Approval requires a current position, `openedAt` no later than Record date, a
valid owner/chat/message-bound token, and `processed == false`. Calculations use
the current quantity because dated holdings are outside scope. Portfolio
mutation and `processed = true` are persisted together under the existing
per-user lock. Repeated requests may create multiple buttons; after the first
successful approval, all later buttons are rejected by the processed marker.

## Retention and Migration

- Delete every dividend 90 days after its Record date, processed or not.
- If Record date remains missing, delete it 90 days after publication so
  malformed provider rows cannot persist forever.
- Remove `dividendCheckedAt` from asset positions and manual dividend behavior.
- Remove the old hashed `appliedDividendEvents` field; the owner explicitly
  accepts dropping that legacy duplicate ledger.
- Run an idempotent startup migration over `user:` stock documents and record
  completion in the shared `system` collection, ensuring obsolete MongoDB fields
  are physically removed rather than waiting for organic portfolio writes.

## Boundaries

- Manual dividend commands remain independent because they have no SSI ID.
- No dated-lot or legal-entitlement accounting is introduced.
- No automatic dividend application occurs.
- No command names or parameters change.
- SSI remains a best-effort, replaceable provider.

## Risks and Mitigations

- Provider corrections: refresh incomplete records and refresh again before an
  actionable message.
- Duplicate processing: allow repeated notifications but use the persisted
  processed marker to ensure only the first approved button mutates finances.
- Post–Record date rebuy: reject when the current position opened after Record
  date.
- Partial provider failure: keep the portfolio response and existing stored
  history intact; retry on a later request.
- Concurrent callbacks: recheck state under the user lock and save processing
  atomically with the financial mutation.

## Approval

The project owner selected per-user dividend history and approved storing full
event details outside `assets`, historical refetch for missing Record dates,
Record-date approval gating, and 90-day retention. The owner later revised the
delivery rule so every portfolio request repeats every unprocessed event until
processing or expiry, even if SSI later omits the retained event.
