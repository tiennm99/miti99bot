---
title: >-
  Amlich converter improvements: leap-month hint, razor-edge caveat,
  golden-table tests
description: >-
  Three merged non-breaking improvements to internal/modules/amlich from
  brainstorm decision
status: completed
priority: P2
branch: main
tags:
  - amlich
  - telegram-bot
blockedBy: []
blocks: []
created: '2026-08-18T15:04:11.318Z'
createdBy: 'ck:plan'
source: skill
---

# Amlich converter improvements: leap-month hint, razor-edge caveat, golden-table tests

## Overview

Implement the three improvements selected in
`plans/reports/brainstorm-decision-260818-2158-amlich-improvements-selection-report.md`
(research: `plans/reports/research-brainstorm-260818-2147-amlich-converter-improvements-report.md`):

1. `/duonglich` leap-month ambiguity hint — when the entered month is also that year's leap month
   and no `nhuan` flag given, append a one-line hint.
2. Razor-edge caveat — both commands warn when the result's lunar month touches one of the 7
   disputed month boundaries from 2072 on (documented in `docs/amlich-known-issues.md`).
3. Golden-table regression testdata — freeze the verified 1800–2199 month-structure output.

Explicitly out of scope (rejected with verification in the research report): ΔT model update,
pre-1968 historic mode, table-driven engine rewrite, range extension, extra calendar features.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Lunar core helpers](./phase-01-lunar-core-helpers.md) | Completed |
| 2 | [Handler replies](./phase-02-handler-replies.md) | Completed |
| 3 | [Golden-table testdata and docs](./phase-03-golden-table-testdata-and-docs.md) | Completed |

## Dependencies

None cross-plan. Phase 2 depends on phase 1; phase 3 is independent.

## Acceptance Criteria (whole plan)

- All existing tests pass unchanged — especially `knownDates` pins (20/6/1944, 7/7/1967),
  `TestSolarLunarRoundTrip`, `TestLeapMonthTable`.
- `/duonglich 5/5/2028` (leap-5 year) shows the hint; `/duonglich 5/5/2028 nhuan` and non-leap
  years do not.
- Conversions inside a lunar month adjacent to the 09/12/2072 boundary show the caveat (both
  commands); ordinary dates do not.
- Golden file regenerated from HEAD is byte-identical to the committed one.
- No public-contract changes; reply format only gains optional trailing lines.
- `go vet ./...` and `staticcheck` clean (repo standard).

## Validation

`go test ./internal/modules/amlich/` after each phase; full `go test ./...` + lint at the end.
