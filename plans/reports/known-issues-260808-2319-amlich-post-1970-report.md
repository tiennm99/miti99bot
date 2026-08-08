# Amlich Known Issues — Dates ≥1970 (Future Reference)

**Date:** 2026-08-08 | Scope: `internal/modules/amlich`, usage restricted to dates from 1970 onward.
Context: follows `algorithm-comparison-260808-2257-amlich-truncated-vs-meeus49-report.md` (decision: keep HND truncated-Meeus).

Restricting to ≥1970 removes the pre-1968 UTC+8 convention issue entirely (UTC+7 official since 1968 reform).

## Real risks

1. **Razor-edge lunations ≥2072** — new moon within ~2 min of UTC+7 midnight; month boundary (±~30 days of output) may differ ±1 day from future official tables. Dates: 09/12/2072, 15/11/2077, 07/05/2130, 26/05/2150, 17/05/2159, 22/01/2175, 26/01/2199. Inherently undecidable today (see 2); not fixable, only documentable.
2. **ΔT extrapolation drift** — two-branch polynomial (and NASA's) diverge from actual Earth rotation (ΔT ≈ 69 s, flat since ~2016 vs predicted growth). Minutes-level error by 2100+; only matters in case-1 situations. Everything ≥ ~2070 = best-effort prediction.
3. **Solar-term precision** — `sunLongitude` = true longitude, no nutation/aberration (~10 min systematic in term timing). Solstice/trung khí within ~10 min of midnight could flip month numbering / leap-month placement for a whole lunar year. A/B diff 1800–2199 found zero occurrences (all diffs were case-1 new moons); low probability, high impact.
4. **No official-override hook** — pure astronomy; a state-decreed deviation or rule change would silently diverge. None post-1968 to date.

## Support noise (correct, looks like bugs)

5. **VN–China divergence years** (~2030, ~2053; historical 1985, 2007): bot differs from Chinese-calendar sources by design; expect user reports.
6. **`/duonglich` default in leap months** (2028 leap 5, 2031 leap 3, …): bare day arg fills current month as regular month, leap flag requires explicit `nhuan`. UX ambiguity, not wrong output.

## Already handled

- Overshoot day-0 dates ≥1970 (07/05/2054, 09/04/2062): fixed, regression-tested (`TestSolarToLunar_LunationOvershoot`).
- 2033 leap month 11: correct, covered by `TestLeapMonthTable`.
- Gregorian 2100 non-leap: handled by JD math.
- Bounds 1800–2199: clean rejection outside range.

## Unresolved questions

1. Should the 7 razor-edge lunations be surfaced to users (e.g., a caveat in bot reply for affected months ≥2072)? Currently silent.
2. Revisit ΔT model if IERS trends persist (ΔT flat/decreasing); a data-driven update only pays off near case-1 boundaries.
3. Monitor state calendar bureau publications past 2100 for future ground truth.
