# Phase 04 — Retention Cron

**Priority:** P1
**Status:** Complete
**Depends on:** Phases 02 + 03

## Overview

Daily cron trims `trading_trades` to enforce caps:
- **Per user:** keep newest 1000 rows, delete older.
- **Global:** keep newest 10000 rows across all users, delete oldest (FIFO).

## Requirements

**Functional**
- Schedule: `"0 17 * * *"` (daily 17:00 UTC = 00:00 ICT).
- Per-user pass first (bounded by user count), then global FIFO.
- Report count deleted via `console.log` (shows in Cloudflare logs).

**Non-functional**
- Safe to retry — idempotent (just deletes excess rows).
- Bounded execution time — large backlog handled in batches of 1000 deletes per statement if needed.

## Architecture

Added to `src/modules/trading/index.js`:

```js
crons: [{
  schedule: "0 17 * * *",
  name: "trim-trades",
  handler: trimTradesHandler,
}]
```

### Per-user trim query

```sql
DELETE FROM trading_trades
WHERE id IN (
  SELECT id FROM trading_trades
  WHERE user_id = ?
  ORDER BY ts DESC
  LIMIT -1 OFFSET 1000
);
```

Run once per distinct `user_id` from a `SELECT DISTINCT user_id FROM trading_trades`.

### Global FIFO trim

```sql
DELETE FROM trading_trades
WHERE id IN (
  SELECT id FROM trading_trades
  ORDER BY ts DESC
  LIMIT -1 OFFSET 10000
);
```

## Related Code Files

**Create**
- `src/modules/trading/retention.js` — `trimTradesHandler(event, { sql })`
- `tests/modules/trading/retention.test.js`

**Modify**
- `src/modules/trading/index.js` — wire cron entry
- `wrangler.toml` — add `"0 17 * * *"` to `[triggers] crons`

## Todo List

- [x] `retention.js` with per-user + global trim
- [x] Wire cron entry
- [x] Tests: seed >1000 per user + >10000 global, assert trimmed counts

## Success Criteria

- Given 1500 rows for user A, cron leaves 1000 newest.
- Given 15000 rows globally across users, cron leaves 10000 newest.
- Console logs deletion counts.

## Risks

- Per-user pass cost scales with user count. At current scale (hobby) irrelevant; document limit for future.
- Competing writes during trim — acceptable since trades append-only.
