# Phase 03 — Trading Trade History

**Priority:** P1
**Status:** Complete
**Depends on:** Phase 01

## Overview

Persist every buy/sell in `trading_trades` table. Add `/history [n]` command to show last N trades for the caller (default 10, max 50).

## Requirements

**Functional**
- Every successful buy/sell inserts a row: `(id, user_id, symbol, side, qty, price_vnd, ts)`.
- `/history` → last 10 trades (newest first).
- `/history 25` → last 25 (clamp 1..50).
- Rendered as compact table (HTML-escaped).
- **No inline cap enforcement** — cleanup cron (Phase 04) handles it.

**Non-functional**
- Insert is fire-and-forget from the user's perspective but must complete before `ctx.reply` (use `await`).
- Failure to persist does NOT fail the trade — log + swallow (portfolio KV is source of truth for positions).

## Architecture

```
src/modules/trading/
├── index.js          # export crons[] + commands[] (unchanged shape, new cron + new command)
├── handlers.js       # buy/sell call recordTrade() after portfolio update
├── history.js        # NEW — recordTrade(), listTrades(), /history handler, format
├── migrations/
│   └── 0001_trades.sql   # NEW
```

### Schema (`trading_trades`)

```sql
CREATE TABLE trading_trades (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  symbol TEXT NOT NULL,
  side TEXT NOT NULL CHECK (side IN ('buy','sell')),
  qty INTEGER NOT NULL,
  price_vnd INTEGER NOT NULL,
  ts INTEGER NOT NULL  -- unix ms
);
CREATE INDEX idx_trading_trades_user_ts ON trading_trades(user_id, ts DESC);
CREATE INDEX idx_trading_trades_ts ON trading_trades(ts);  -- for global FIFO trim
```

## Related Code Files

**Create**
- `src/modules/trading/history.js`
- `src/modules/trading/migrations/0001_trades.sql`
- `tests/modules/trading/history.test.js`

**Modify**
- `src/modules/trading/index.js` — register `/history` command, accept `sql` in init
- `src/modules/trading/handlers.js` — call `recordTrade()` on buy/sell

## Todo List

- [x] Migration SQL
- [x] `recordTrade(sql, { userId, symbol, side, qty, priceVnd })`
- [x] `listTrades(sql, userId, n)`
- [x] `/history` command handler + HTML formatter
- [x] Wire into buy/sell handlers
- [x] Tests (Miniflare D1): record + list + max-cap clamp

## Success Criteria

- Buy + sell produce rows in `trading_trades`.
- `/history` returns last 10 newest-first.
- `/history 50` returns 50. `/history 999` clamps to 50. `/history 0` falls back to default.
- Persistence failure is logged but does not break the trade reply.

## Risks

- D1 write latency inside a user-facing handler — measured in tests; if >300ms, consider `ctx.waitUntil(insert)` non-blocking.

## Next Steps

- Phase 04 adds the retention cron consuming this table.
