---
phase: 2
title: Integrate Portfolio Renderers
status: completed
priority: P1
dependencies:
  - 1
effort: small
---

# Phase 2: Integrate Portfolio Renderers

## Overview

Wire the approved compact formatters into the four monetary position columns
of both portfolio renderers and lock down user-visible output.

## Requirements

- Functional: compact `Avg`, `Now`, `Value`, and position `Unrealized P&L`.
- Functional: keep `Qty`, percentages, title currency markers, and `N/A` intact.
- Functional: keep stock and coin summary tables at their existing full formats.
- Non-functional: do not change quote fetching, valuation, sorting, Telegram
  HTML escaping, reply truncation, or dividend notification behavior.

## Architecture

Only the position-row construction changes. Stock calls its VND compact helper;
coin calls its USD compact helper. Summary row construction continues using the
existing full-value formatters, isolating the behavior to the requested cells.

## Related Code Files

- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/handlers.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/stats_test.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/coin/views.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/coin/handlers_test.go`
- Inspect: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/coin/views_reply_budget_test.go`

## Implementation Steps

1. Replace stock position monetary cells with automatic compact formatting,
   including the missing-price branch's available average.
2. Apply compact USD formatting to the equivalent coin position cells.
3. Keep both summary slices unchanged and verify full values remain present.
4. Strengthen renderer assertions for magnitude suffixes, P&L percentages,
   currency conventions, and the absence of compaction in summaries.
5. Confirm partial-price paths still emit `N/A` and unavailable Account P&L.

## Success Criteria

- [x] Stock position cells select `k/M/B/T` independently and use VND separators.
- [x] Coin position cells select `k/M/B/T` independently and retain `$`.
- [x] Position P&L amounts compact; percentages remain unchanged.
- [x] Summary values and non-portfolio replies remain byte-for-byte compatible.
- [x] Missing and overflowed quotes retain current `N/A` behavior.
- [x] Telegram reply-budget tests remain valid or improve due to shorter cells.

## Risk Assessment

Main regression risk is accidentally compacting summary or standalone output.
Use private position-only wrappers and assertions that check both compact rows
and full summary values. No authorization, concurrency, persistence, or network
boundary changes.
