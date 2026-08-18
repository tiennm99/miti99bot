---
phase: 3
title: Golden-table testdata and docs
status: completed
effort: S
priority: P3
dependencies: []
---

# Phase 3: Golden-table testdata and docs

## Overview

Freeze the verified 1800–2199 month structure as committed testdata, and close the resolved open
questions in `docs/amlich-known-issues.md`. Independent of phases 1–2.

## Requirements

- Functional: a test recomputes every lunar year's structure from the engine and compares
  byte-exact against `testdata/lunar-years-1800-2199.txt`; `go test -run TestGoldenTable -update`
  regenerates the file (standard `-update` flag idiom).
- File format, one line per lunar year:
  `YYYY L n1 n2 ... nN` — `L` = leap month number (0 = none), `n*` = month lengths (29/30) in
  chronological order, tháng 1 first, leap month inserted in sequence after month `L`
  (12 entries normal year, 13 leap year).
- Non-functional: file lives in `internal/modules/amlich/testdata/`; generation logic lives in the
  test file only (no production code).

## Architecture

- Build each year from `lunarToSolar` month starts: for lunar year Y, JDs of tháng 1..12 (+ leap
  where `lunarToSolar(1, m, Y, true)` succeeds), sorted chronologically; lengths = successive
  start-JD differences, last month's length from tháng 1 of Y+1. Simpler than sweeping days and
  exercises the lunar→solar direction the round-trip test already covers from the other side.
- Rationale over round-trip: `TestSolarLunarRoundTrip` proves self-consistency only; a future
  engine change that shifts a boundary consistently in both conversions passes it. The golden file
  pins the actual verified placement.

## Related Code Files

- Create: `internal/modules/amlich/testdata/lunar-years-1800-2199.txt`
- Modify: `internal/modules/amlich/lunar_test.go` (or new `golden_test.go` if it crowds the file)
- Modify: `docs/amlich-known-issues.md`

## Implementation Steps

1. Write `TestGoldenTable` with `var update = flag.Bool("update", false, ...)`; generator builds the
   full table string; when `-update`, write file and skip compare; else compare byte-exact with a
   diff-friendly failure message (first differing line).
2. Generate the file once; eyeball spot checks: 2025 leap 6, 2028 leap 5, 2033 leap 11, 1944
   leap 4, 1967 no leap month (leap numbers already pinned by `TestLeapMonthTable` — must agree;
   the doc's "30/5 Đinh Mùi" for 1967 is day 30 of the regular month 5, not a leap month).
3. Commit the generated file. Edge-case years must round-trip with the existing suite untouched.
4. Update `docs/amlich-known-issues.md`:
   - Open question 1 → resolved: caveat line added, months touching the boundary only.
   - Open question 2 → closed (don't build): add the South-Vietnam wrinkle (UTC+7 until 1959,
     UTC+8 1960–67 per Hồ Ngọc Đức's historic-calendar page) — a single UTC+8 mode would be wrong
     for the South 1955–59, strengthening the existing conclusion.
   - "`/duonglich` defaults inside leap months" item → note the hint now self-disambiguates.
   - Keep questions 3 (ΔT) and 4 (future official tables) open; add one line noting the 2026 CGPM
     leap-second-abolition vote as a further far-future timescale uncertainty in the same bucket.

## Success Criteria

- [ ] Golden file committed; regeneration from HEAD is byte-identical
- [ ] Golden leap months agree with `TestLeapMonthTable` pins
- [ ] Docs updated; no stale claims left in the two resolved items
- [ ] `docs/amlich-known-issues.md` stays under docs.maxLoc (800)

## Risk Assessment

- Ordering bug when inserting the leap month (nhuận m sorts after regular m) — chronological
  sort by start JD avoids hand-rolled index math.
- `flag.Bool` at package scope collides if a flag named `update` ever exists elsewhere in the
  package's tests — it doesn't today; keep the name.
