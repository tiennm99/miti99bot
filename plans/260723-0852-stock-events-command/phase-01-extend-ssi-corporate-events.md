---
phase: 1
title: Extend SSI Corporate Events
status: completed
priority: P2
dependencies: []
---

# Phase 1: Extend SSI Corporate Events

## Overview
Add an additive SSI corporate-action fetch path for read-only event listing. Preserve the current dividend-specific contract used by `/stock_portfolio` and retained dividend history.

## Requirements
- Functional: fetch all SSI corporate-action rows for one normalized symbol in `(after, through]`, with the same one-day Asia/Saigon overlap and provider page caps already enforced in `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi.go:23` and `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi.go:113`.
- Functional: deduplicate by SSI `CorId`, then return deterministic chronological order by `PublishedAt`, tie-broken by provider ID as today’s dividend path does at `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi.go:160`.
- Functional: preserve SSI raw display fields for the generic command path: symbol, provider ID, SSI type code/label, title/description/name, published/ex/record/payment strings, value, ratio, and source URL.
- Functional: use a private cursor timestamp only for filtering and sorting; do not replace the raw SSI date strings with normalized event dates in the display model.
- Functional: keep `FetchDividendEvents` behavior unchanged for dividend discovery and `/stock_portfolio` notifications (`C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events.go:12`, `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_notifications.go:204`).
- Non-functional: no persistence, no callback/button work, no schema/index/system-state changes.

## Architecture
`SSIDividendProvider` already owns SSI HTTP access, page fetches, date parsing, and source-link generation in `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi.go:92`, `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi.go:172`, `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi.go:247`, and `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi.go:309`. Phase 1 adds a parallel generic event model plus an additive fetch method that reuses those helpers, but does not widen `DividendEvent` validation.

Data flow:
1. Symbol and time window enter the generic provider.
2. SSI pages are fetched with the existing timeout/cap logic.
3. Each raw SSI row is copied into a generic display model with minimal validation: matching symbol, valid provider ID, and a private cursor timestamp via `publicDate` or the existing fallback chain.
4. Generic rows are sorted and returned to the command layer with raw SSI strings intact; dividend-only normalization remains on the separate `FetchDividendEvents` path.

## Related Code Files
- Create: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock_events.go` — introduce the generic stock-event model and provider interface shared by the SSI provider and command handler.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi.go` — add the additive generic SSI fetch method and generic row normalizer while keeping dividend normalization intact.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi_test.go` — cover generic pagination/dedup/range/order behavior and explicit dividend-regression expectations.

## Implementation Steps
1. Define a generic read-only event type and provider interface separate from `DividendEvent` so `/stock_portfolio` persistence rules are not broadened.
2. Add an additive SSI fetch method that reuses `fetchPage`, `startOfSaigonDay`, `parseSSIOptionalDate`, `parseSSIDate`, and `eventSourceURL` instead of introducing a second transport/client stack.
3. Normalize every matching SSI row into a generic display model with title fallback order `EventTitle -> EventDescription -> EventName -> EventListCode`, while requiring only data needed for chronological display.
4. Keep `FetchDividendEvents` and `normalizeEvent` behavior additive-only: no relaxed dividend validation for value/ratio fields, no changes to portfolio history serialization.
5. Add provider tests for cross-page deduplication, one-day overlap filtering, deterministic ordering, generic inclusion of non-dividend corporate actions, and a regression assertion that dividend discovery still excludes non-dividend rows.

## Success Criteria
- [x] Generic SSI fetch returns all eligible corporate actions in deterministic chronological order for one ticker and lookback window.
- [x] Generic fetch propagates upstream/paging/decode errors to the caller instead of silently succeeding with partial pages.
- [x] Existing dividend-provider tests still pass with no behavior drift in `/stock_portfolio` discovery.

## Risk Assessment
- High: weakening dividend validation would change `/stock_portfolio` suggestions and history merges. Mitigation: additive generic method only; keep dividend-specific tests green before Phase 2.
- Medium: malformed SSI rows could create unstable or unsortable output. Mitigation: require a usable publish timestamp and provider ID; skip rows that cannot be placed chronologically.
- Medium: over-refactoring the proven dividend loop increases blast radius. Mitigation: reuse helper functions first; duplicate only the page-scan control flow if that keeps the dividend path unchanged.
- Rollback: revert the generic provider additions and tests only. No stored data or public command contract exists yet, so rollback is code-only and isolated.
