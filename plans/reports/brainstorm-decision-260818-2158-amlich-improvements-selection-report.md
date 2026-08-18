# Brainstorm Decision: amlich Converter Improvements — Selected Scope

Date: 2026-08-18 21:58 (+07)
Basis: `plans/reports/research-brainstorm-260818-2147-amlich-converter-improvements-report.md`
Mode: user delegated choice ("choose the best, merge if mergable").

## Decision

All three recommended items merged into one change set — they are complementary (input UX,
output honesty, test hardening), not alternatives. Rejections from the research report stand.

### In scope

1. **Leap-month hint in `/duonglich`** (highest user value, do first)
   - Trigger: resolved month == that lunar year's leap month AND `nhuan` flag absent.
   - Output: append "Năm nay có tháng X nhuận — thêm 'nhuan' nếu ý bạn là tháng nhuận."
   - Impl: `leapMonthOf(year int) (month int, ok bool)` helper in `lunar.go` reusing
     `getLunarMonth11` + `getLeapMonthOffset`; hint assembled in `handlers.go`.
   - NOT triggered when `nhuan` explicit (resolves research report open question 2: no).

2. **Razor-edge caveat in both commands**
   - Data: 7 disputed month-start JDs (new moons of 09/12/2072, 15/11/2077, 07/05/2130,
     26/05/2150, 17/05/2159, 22/01/2175, 26/01/2199) as a package-level set in `lunar.go`,
     JD values derived at implementation time and pinned by test.
   - Trigger: containing month start OR next month start is in the set → append one-line
     caveat: result near disputed lunar-month boundary, may differ ±1 day from future
     official tables.
   - Scope: months touching the boundary only, not the whole lunar year (resolves research
     report open question 1: months-only; year-wide is alarmist).
   - Impl: `nearDisputedBoundary(jdn int) bool` helper; handlers call it with the solar JD.

3. **Golden-table regression testdata**
   - `internal/modules/amlich/testdata/lunar-years-1800-2199.txt`: one line per year —
     year, leap-month index (0 = none), month lengths in order (12 or 13 entries).
   - Test recomputes from engine, compares byte-exact; `-update` flag idiom regenerates.
   - Rationale: round-trip test proves self-consistency only; golden table freezes the
     verified boundary placement against silent drift.

### Out of scope (rejected, verified in research report)

- ΔT model update — breaks bit-compatibility with ecosystem; verified by prior 400-year diff.
- Pre-1968 historic mode — ground truth fragments (North UTC+8 1945–67; South UTC+7→UTC+8 1960).
- Table-driven engine rewrite — no authoritative source beyond current engine; test-data-only instead.
- Range extension beyond 1800–2199; can-chi day names, tiết khí, holiday lookup.

## Touchpoints

- `internal/modules/amlich/lunar.go` — `leapMonthOf`, `nearDisputedBoundary`, disputed-JD set.
- `internal/modules/amlich/handlers.go` — hint + caveat lines in both command replies.
- `internal/modules/amlich/lunar_test.go`, `handlers_test.go` — new cases; existing pins untouched.
- `internal/modules/amlich/testdata/` — new golden file.
- `docs/amlich-known-issues.md` — close open questions 1–2 with these resolutions.

## Acceptance Criteria

- All existing tests pass unchanged, incl. `knownDates` (20/6/1944, 7/7/1967) and full round-trip.
- `/duonglich 5/5/2028` (leap-5 year) shows hint; `/duonglich 5/5/2028 nhuan` and non-leap years don't.
- `/amlich` for a date inside a month adjacent to 09/12/2072 boundary shows caveat; ordinary dates don't;
  `/duonglich` symmetric.
- Golden file regenerated from HEAD is byte-identical to committed version.
- ~100 LOC total incl. tests; no public-contract changes.

## Risks

- Caveat JD derivation error → wrong months flagged. Mitigation: pin the 7 JDs in a test that
  recomputes them from `getNewMoonDay`.
- Reply strings are user-visible contract for handler tests — update expected strings, don't loosen asserts.

## Unresolved Questions

- None blocking. Post-leap-second timescale question stays parked in `docs/amlich-known-issues.md`
  open question 4.
