# Amlich Converter: Algorithm Comparison & Decision

**Date:** 2026-08-08 | **Verdict: KEEP current implementation** (Hồ Ngọc Đức truncated-Meeus port, `internal/modules/amlich/lunar.go`)

## Process

1. Two parallel research agents:
   - Algorithms: `plans/reports/researcher-260808-2252-lunar-conversion-algorithms-report.md`
   - Calendar rules: `plans/reports/researcher-260808-2252-vietnamese-lunisolar-rules-report.md`
2. Implemented candidate B manually from the rules spec: Meeus *Astronomical Algorithms* ch. 49 full new-moon series (J2000 epoch, 25 periodic + 14 planetary terms), apparent solar longitude with nutation+aberration (ch. 25), Espenak–Meeus piecewise ΔT, month 11 anchored by bisecting the winter-solstice instant.
3. Diffed A (current) vs B over every day of 1800–2199 (146,097 days); validated both against external truth (Tết dates, leap-month tables incl. 2033 leap month 11, national holidays, Tết 2007 = Feb 17, Tết 1968 = Jan 29).

## Results

- B internally sound: full-range round-trip + day-continuity pass; all external vectors pass; leap-month table identical to A.
- A vs B differ on **270 days (0.18%)** = **9 lunations**, every one a new moon within **±2 min of 17:00 UT (= UTC+7 midnight)**: 1944-06-20, 1967-07-07, 2072-12-09, 2077-11-15, 2130-05-07, 2150-05-26, 2159-05-17, 2175-01-22, 2199-01-26. Espenak's phase catalog lists the two historical ones at exactly 17:00 UT.

## Adjudication of the 9 disputes

- **20/6/1944**: published VN calendar = 30/4 nhuận (new month starts Jun 21). A ✓, B ✗.
- **7/7/1967**: published VN calendar = 30/5 Đinh Mùi (month 6 starts Jul 8). A ✓, B ✗.
  - Independent confirmation: pre-1968 Vietnam used the Chinese-calendar convention (UTC+8; reform decree 8/8/1967 effective 1968). At UTC+8 these ~17:00 UT new moons land unambiguously on the next civil day (01:00 local) — HKO tables agree. A's output coincides with the historical record; B's sharper instant (16:58–17:00 UT) lands on the wrong side.
- **Remaining 7 (2072–2199)**: unverifiable. ΔT extrapolation uncertainty (tens of seconds to ±100 s) exceeds the 1–2 min margins, so neither engine can claim those days. No demonstrated benefit for B.

## Decision rationale

1. A matches published Vietnamese calendars on **all** verifiable dates, including both razor-edge historical disputes; B fails those two.
2. A is the de-facto standard (HND algorithm) used by essentially all Vietnamese calendar apps/sites — bot output agrees with what users cross-check against.
3. B's precision advantage only manifests in 2072+ where truth is unknowable (ΔT); YAGNI.
4. A is simpler (no bisection, fewer series terms); KISS.

Candidate B (`lunar_rules.go`) deleted after evaluation; its value is captured as regression tests.

## Changes kept in repo

- `lunar_test.go`: two new `knownDates` entries pinning 20/6/1944 → 30/4 nhuận and 7/7/1967 → 30/5 (guards against future "higher-precision" rewrites flipping them).
- Earlier this session (prior task): lunation-overshoot fix in `solarToLunar` + full-range round-trip/continuity tests + leap-month table test. Full suite green.

## Unresolved questions

1. Should the module model pre-1968 UTC+8 convention explicitly? A matches the published record on the two known razor-edge cases by numeric coincidence, not by rule. A true UTC+8 pre-1968 mode would change ~1/24 of pre-1968 month boundaries vs current output and diverge from the HND JS standard everyone else uses — recommend NO unless a user reports a concrete mismatch.
2. 2072+ razor-edge lunations (7 dates listed above) are inherently uncertain; if the state calendar bureau publishes tables past 2100, revisit.
3. xemlicham.com's engine may itself derive from HND (partial circularity); mitigated by HKO cross-check and the pre-1968 UTC+8 argument, but a scan of Trần Tiến Bình's printed 1901–2100 tables would be the gold-standard confirmation.
