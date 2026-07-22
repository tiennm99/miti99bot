---
title: Mobile Stock and Coin Portfolio Columns
date: 2026-07-22 17:05
component: stock and coin portfolio UI
status: completed
---

# Mobile Stock and Coin Portfolio Columns

## Context

The stock and coin position tables were still too wide for Telegram-sized screens. Long headers and a combined unrealized P&L cell made each row harder to scan on mobile.

## What Happened

Both position tables now use `Sym`, `Qty`, `Avg`, `Now`, `Val`, `P&L`, and `%`. Stock `Avg` and `Now` use thousand VND as an implicit unit without `k`; coin position money omits `$`. P&L amount and percentage are separate cells without parentheses. Summary formatting remains unchanged.

## Reflection

The useful part of this change is that it stayed local to the portfolio renderer. We did not touch command behavior, storage, or trade accounting, so the formatting work did not leak into unrelated output paths.

The review callout about implicit `VND` was intentional, not a bug. The user explicitly removed `(VND)` from the stock title and accepted stock-domain context as sufficient. Keeping full currency labels in the summary would have widened the output and contradicted that decision.

## Decisions

- Keep compact `k/M/B/T` suffixes for position value and P&L amounts.
- Treat stock `Avg` and `Now` as thousand VND without a `k` suffix.
- Treat coin position money as implicit USD without `$`.
- Preserve explicit `VND` in non-portfolio replies where the standalone context is useful.
- Keep existing summary formats unchanged.
- Leave command and storage contracts unchanged.

## Verification

We validated the change with focused tests first, then the full suite. The final checks all passed:

- focused tests
- `go test -count=1 ./...`
- `go build ./...`
- `go vet ./...`
- `golangci-lint run`

Coverage landed at `76.7%` statement coverage, which is acceptable for this change because the work was formatting-heavy and stayed inside existing rendering paths.

## Next

The code change is done, but the commit is still pending user approval. The next step is to package this as a focused commit once that approval lands.
