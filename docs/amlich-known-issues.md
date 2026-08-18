# Amlich: Algorithm Decision and Known Issues

Reference for `internal/modules/amlich`. Records why the current conversion
algorithm was chosen and which edge cases it is known to get wrong or to
report differently from other sources.

## Algorithm

The module ports Hồ Ngọc Đức's truncated-Meeus algorithm (`lunar.go`), the
de-facto standard behind essentially every Vietnamese calendar app and site.
Supported range is 1800–2199; both commands reject anything outside it with an
explicit message.

A full Meeus *Astronomical Algorithms* ch. 49 implementation (25 periodic + 14
planetary new-moon terms, apparent solar longitude with nutation and
aberration, Espenak–Meeus piecewise ΔT, month 11 anchored by bisecting the
winter solstice) was written and diffed against the current one over every day
of 1800–2199 — 146,097 days. They disagree on 270 days (0.18%), which is 9
lunations, every one a new moon within ±2 minutes of 17:00 UT (= midnight at
UTC+7).

Two of those nine are historically verifiable, and the current algorithm is
right on both: 20/6/1944 (published Vietnamese calendar: 30/4 nhuận) and
7/7/1967 (30/5 Đinh Mùi). Vietnam used the Chinese-calendar convention of UTC+8
before the 1968 reform, which puts these new moons unambiguously on the next
civil day; the higher-precision engine lands on the wrong side of both. The
remaining seven fall in 2072–2199, where ΔT uncertainty exceeds the disputed
margin, so neither engine can claim them.

Both dates are pinned in `lunar_test.go` `knownDates` so a future
"higher-precision" rewrite cannot silently flip them.

## Known issues

**Razor-edge lunations from 2072 on.** New moon within ~2 minutes of UTC+7
midnight, so the month boundary — and therefore output within roughly 30 days
of it — may differ by one day from future official tables: 09/12/2072,
15/11/2077, 07/05/2130, 26/05/2150, 17/05/2159, 22/01/2175, 26/01/2199. Those
are the disputed new-moon days themselves; the engine begins each of these
months on the following day (`disputedMonthStarts` in `lunar.go`). Not fixable
today. Both commands append a caveat when the result falls in a lunar month
that starts or ends on one of these boundaries.

**ΔT extrapolation drift.** The two-branch polynomial diverges from actual
Earth rotation (ΔT has been roughly flat near 69 s since ~2016 against
predicted growth), reaching minutes-level error past 2100. It only matters near
a razor-edge boundary, but it means everything from about 2070 onward is a
best-effort prediction.

**Solar-term precision.** `sunLongitude` returns true longitude without
nutation or aberration, a systematic offset of about 10 minutes in solar-term
timing. A solstice or trung khí within ~10 minutes of midnight could shift
month numbering or leap-month placement for a whole lunar year. The 1800–2199
diff found zero such occurrences — low probability, high impact.

**No override for decreed changes.** The module is pure astronomy. A
state-decreed deviation or rule change would diverge silently. None has
occurred since 1968.

## Correct output that gets reported as a bug

**Divergence from Chinese-calendar sources.** Vietnam uses UTC+7 and China
UTC+8, so the two calendars genuinely differ in some years — historically 1985
and 2007, upcoming around 2030 and 2053. Users cross-checking against a Chinese
source will see a mismatch that is not an error.

**`/duonglich` defaults inside leap months.** A bare day argument fills in the
current month as a regular month; the leap month requires the explicit `nhuan`
flag. In leap years such as 2028 (leap 5) and 2031 (leap 3) this is ambiguous
to the user, but the output is correct for what was entered. When the queried
month is also that year's leap month (and the exact leap date exists), the
reply appends a hint suggesting the `nhuan` flag.

## Handled, with regression tests

- Lunation overshoot that returned lunar day 0 (07/05/2054, 09/04/2062) —
  fixed, covered by `TestSolarToLunar_LunationOvershoot`.
- Leap month 11 in 2033, the classic stress case — correct, covered by
  `TestLeapMonthTable`.
- Gregorian 2100 being a non-leap year — handled by the JD arithmetic.
- Out-of-range years — rejected cleanly at the 1800/2199 bounds.

## Open questions

1. ~~Should the seven razor-edge lunations carry a caveat?~~ Resolved: both
   commands append a caveat for results in the lunar months touching those
   boundaries — months only, not the whole lunar year.
2. ~~Should pre-1968 UTC+8 be modelled explicitly?~~ Closed: don't build. Per
   Hồ Ngọc Đức's historic-calendar notes it is not even a single rule — the
   North used UTC+8 from 1945–67, but the South used UTC+7 until 1959 and
   UTC+8 only from 1960–67, with dynastic tables before 1945. Any concrete
   user-reported mismatch gets a doc note, not a mode.
3. Revisit the ΔT model if the flat IERS trend persists. A data-driven update
   only pays off near razor-edge boundaries — and would diverge from the
   reference algorithm the whole ecosystem runs, so it needs official tables
   as cover before it is worth it.
4. Watch for state calendar bureau tables published past 2100; they would be
   the first ground truth for the 2072+ disputes. Related far-future
   uncertainty in the same bucket: the CGPM votes in late 2026 on abolishing
   the leap second, which would let civil UTC+7 drift slowly from the
   astronomical time the algorithm models.
