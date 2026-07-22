---
title: Stock Dividend History and Notification State
date: 2026-07-22 16:14
component: stock
status: resolved
---

# Stock Dividend History and Notification State

## Context

We needed per-user dividend history that survives repeated `/stock_portfolio` calls without turning SSI’s flaky responses into duplicate credits or missing user notifications. The old shape was not enough: it had no stable per-event history, no clean way to separate notifications from processing state, and no safe way to distinguish fresh SSI data from already-accepted dividends.

## What Happened

We moved dividend history to `dividends.<ticker>.<SSI ID>` so every SSI event has a stable per-user record. We intentionally did not add `dividendCheckedAt`; that timestamp would have encouraged the wrong mental model and still would not have solved idempotency.

Record-date gating now decides whether the portfolio shows an `Apply` button. Events without a Record date stay informational while the bot keeps refreshing the original publication window for SSI updates. Historical refresh stays active until the event is processed or ages out at 90 days.

Notifications now repeat on every `/stock_portfolio` response until the event is processed or expired, even if SSI temporarily omits the event. That was the right call because silence from SSI is not proof that the event disappeared. Multiple buttons for the same event are still expected; only the first successful click can mark the per-user record processed.

We also added a startup migration to initialize the new history layout and preserve existing state. The migration is idempotent and guarded so it can run on every boot without redoing work.

## Decisions

We chose stable per-event history over recalculating from raw SSI responses on every request. That gave us deterministic notification state and idempotent processing.

We accepted the legacy duplicate-credit risk for manual dividend commands versus SSI-backed buttons. That is ugly, but it is explicit: manual commands do not carry SSI event IDs, and trying to retrofit full cross-path deduplication would have been more invasive than the feature warranted.

We also kept notifications visible until processed or 90 days old, instead of suppressing them after one failed SSI lookup. Suppression would have hidden real work from users and made the bot look broken.

## Verification

We verified the history shape and button flow with repeated portfolio refreshes, missing-date refreshes, and repeated clicks on the same event. The key checks were:

- the same SSI event keeps the same `dividends.<ticker>.<SSI ID>` record
- `Apply` only appears for Record-date events
- informational events continue to recheck their original publication window
- repeated portfolio commands keep showing unprocessed notifications
- a second button click does not credit the same event again
- the startup migration runs cleanly more than once

## Risks/Next

The remaining risk is mostly operational, not logical: SSI can still omit or reshuffle events, and the bot has to treat that as an external data problem rather than a local delete signal.

The legacy duplicate-credit risk is still real for manual commands. We are accepting that for now because the alternative was a larger behavioral rewrite across old and new dividend paths.

Next step is to keep watching for edge cases where SSI history and manual adjustments overlap in surprising ways. If that starts showing up in real usage, the processing model will need a dedicated reconciliation pass instead of more ad hoc checks.
