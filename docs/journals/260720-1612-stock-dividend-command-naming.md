# Stock Dividend Command Naming Journal

## Context

Researched official VSDC dividend notices and brainstormed clearer stock command
contracts for cash, share, and mixed dividend events.

## What Happened

- Approved `/stock_cash_dividend <vnd_per_share> <TICKER>` for cash dividends.
- Approved `/stock_share_dividend <owned:new> <TICKER>` for share dividends.
- Approved `/stock_dividend <vnd_per_share> <owned:new> <TICKER>` for mixed
  cash-and-share dividends.
- Official examples support ratios such as `4:1` and `100:10`.
- No implementation changes made yet.

## Decisions

- Share ratios use `owned:new`; both parts must be positive whole numbers.
- New shares use integer floor division. Fractional entitlements discarded.
- Cash input means VND paid per pre-event share.
- Mixed dividends calculate cash and new shares from the same pre-event holding,
  then persist both results atomically.
- Stats history needs a one-time, idempotent migration: existing
  `/stock_dividend` usage becomes `/stock_cash_dividend`, and `/stock_bonus`
  usage becomes `/stock_share_dividend`. The new mixed `/stock_dividend` starts
  with fresh stats.

## Next Steps

- Create an implementation plan covering handlers, registration, user-facing
  text, command menu, atomic storage behavior, stats migrations, and tests.
- Implement the approved zero-result behavior: reject share-only payouts, but
  still credit cash and report zero new shares for mixed payouts.
