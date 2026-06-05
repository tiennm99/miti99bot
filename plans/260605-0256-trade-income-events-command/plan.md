---
title: "Trade Income Events Command"
status: completed
created: 2026-06-05
---

# Trade Income Events Command

## Context
- Trading commands are registered in `internal/modules/trading/trading.go`.
- Trading state is per-user KV portfolio in `internal/modules/trading/portfolio.go`.
- Current market data client style is stdlib HTTP with injectable URL/client.
- FireAnt is the default income-event provider; the bot calls `/symbols/{symbol}/timescale-marks` with `startDate` and `endDate`.

## Requirements
- Add `/trade_income_events [TICKER]`.
- With ticker: show recent income/right events for that stock.
- Without ticker: check all non-zero stock holdings in current user's portfolio.
- Recent means last 30 days by FireAnt mark date.
- Do not mutate portfolio.

## Implementation
1. Add FireAnt timescale-mark client and renderer.
2. Register command in trading module.
3. Add read-only handler.
4. Add focused unit tests.
5. Run `gofmt` and `go test ./internal/modules/trading`.

## Status
- [x] Scout existing trading module.
- [x] Select FireAnt configuration path.
- [x] Implement command.
- [x] Test command support.
- [x] Review changes.

## Unresolved Questions
- None.
