---
phase: 1
title: Lunar core helpers
status: completed
effort: S
priority: P2
dependencies: []
---

# Phase 1: Lunar core helpers

## Overview

Add the two pure helpers the handlers need: the disputed-boundary set + membership check, and
leap-month existence probing. No behavior change to existing conversion functions.

## Requirements

- Functional: `nearDisputedBoundary(jdn int) bool` reports whether the lunar month containing
  solar day `jdn` starts or ends on one of the 7 disputed month boundaries.
- Leap-variant existence needs NO new helper: handlers probe `lunarToSolar(day, month, year, true)`
  and treat `err == nil` as "leap variant exists" (DRY — reuses existing validation; also naturally
  suppresses the hint when e.g. day 30 doesn't exist in the 29-day leap month).
- Non-functional: helpers stay in `lunar.go`, unexported, zero allocations on the hot path
  (package-level set built once).

## Architecture

- `disputedMonthStarts` — package-level `map[int]bool` built in a `var` initializer from
  `jdFromDate` on the 7 dates recorded in `docs/amlich-known-issues.md`:
  09/12/2072, 15/11/2077, 07/05/2130, 26/05/2150, 17/05/2159, 22/01/2175, 26/01/2199.
- `nearDisputedBoundary(jdn int) bool` — mirror `solarToLunar`'s month-start search:

```go
k := floorInt((float64(jdn) - jdNewMoonEpoch) / newMoonCycle)
monthStart := getNewMoonDay(k + 1)
for monthStart > jdn {
    k--
    monthStart = getNewMoonDay(k + 1)
}
return disputedMonthStarts[monthStart] || disputedMonthStarts[getNewMoonDay(k+2)]
```

  Rationale for checking next start too: if the *end* boundary of the containing month is disputed,
  the month's length (and day numbers near its end) is what may shift.

## Related Code Files

- Modify: `internal/modules/amlich/lunar.go`
- Modify: `internal/modules/amlich/lunar_test.go`

## Implementation Steps

1. Add `disputedMonthStarts` set with a comment explaining provenance (new moon within ±2 min of
   UTC+7 midnight; see docs/amlich-known-issues.md) — no plan/audit labels in comments.
2. Add `nearDisputedBoundary`.
3. Test: each of the 7 JDs is an actual month start per the engine — for each date, assert
   `solarToLunar(d,m,y)` returns lunar day 1. If any assertion fails ±1 day, the doc date and the
   engine's boundary disagree: adjust the set entry to the engine's month start and flag the doc
   discrepancy in the phase report.
4. Test `nearDisputedBoundary`: true for a mid-month day of the month starting 09/12/2072, true for
   a day in the month *before* it (whose end boundary is disputed), false for an ordinary 2072 date
   far from the boundary and for a 2024 date.

## Success Criteria

- [ ] 7 set entries verified as engine month starts by test
- [ ] `nearDisputedBoundary` true/false cases covered as above
- [ ] Existing lunar tests untouched and green

## Risk Assessment

- Doc dates might be the full-Meeus engine's boundaries rather than the current engine's (off by
  one day). Mitigation: step 3's pin test resolves it mechanically; the ±1-day neighborhood is the
  same disputed lunation either way.
