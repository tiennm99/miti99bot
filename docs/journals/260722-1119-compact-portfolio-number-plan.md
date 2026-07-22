---
date: 2026-07-22
component: stock-and-coin-portfolios
status: completed
---

# Compact Portfolio Number Plan

## Context

Full monetary values make the stock and coin Telegram portfolio position tables
wide and difficult to scan.

## What Happened

The compact format work is done. Automatic `k/M/B/T` suffixes now apply to
stock and coin position values with at most three trimmed fractional digits.
Stock keeps Vietnamese separators; coin keeps USD separators and `$`. Only the
position `Avg`, `Now`, `Value`, and `Unrealized P&L` fields were compacted.

- [Brainstorm report](../../plans/reports/260722-1112-compact-portfolio-number-format.md)
- [Implementation plan](../../plans/260722-1114-compact-portfolio-numbers/plan.md)

## Reflection

Per-value scaling still beats fixed units per column or unit labels in headers.
It keeps the table readable without changing summaries or non-portfolio output.

## Decisions

- Compact only position `Avg`, `Now`, `Value`, and `Unrealized P&L` amounts.
- Keep percentages, summaries, `N/A`, and non-portfolio replies unchanged.
- Focused tests, full tests, `go vet`, and `golangci-lint` all passed.
- A pre-existing extreme-finite overflow in legacy full-value formatters was
  observed separately and left as optional follow-up, not part of this change.

## Next

No further action required for this change set.
