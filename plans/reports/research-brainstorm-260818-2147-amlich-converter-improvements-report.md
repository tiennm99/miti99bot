# Research + Brainstorm Report: Improving the amlich/duonglich Converter

Date: 2026-08-18 21:47 (+07)
Scope: `internal/modules/amlich` — what improvements remain, ranked; what to explicitly reject.
Inputs: repo code + `docs/amlich-known-issues.md` (prior 400-year Meeus-vs-HND diff), 5 web lookups (2 WebSearch, 3 WebFetch).

## Executive Summary

Algorithm question is settled and should stay settled: the truncated Hồ Ngọc Đức port is bit-compatible
with the de-facto Vietnamese ecosystem and empirically beat a full Meeus ch.49 engine on both
historically verifiable razor-edge dates. No engine change is justified by any new evidence found.

Remaining improvement space is small and mostly UX/honesty, not math:
(1) caveat line for the 7 razor-edge lunations 2072+, (2) leap-month ambiguity hint in `/duonglich`,
(3) optional golden-table regression hardening. Everything else researched — ΔT model update,
pre-1968 historic mode, table-driven rewrite, range extension — should be rejected; reasons below.

## New Research Findings (2026)

- **ΔT trend confirms doc's suspicion, changes nothing.** Earth set rotation-speed records in 2024–2025;
  IERS added no leap second in 2024; ~30% chance of a first-ever *negative* leap second before 2035.
  ΔT is flat-to-declining vs the polynomial's predicted growth — so the code's ΔT branch overestimates
  future ΔT. But this only matters inside the already-documented razor-edge windows. See "Reject: ΔT".
- **Leap-second abolition adds a *new* far-future uncertainty.** CGPM votes Oct 2026 to replace the leap
  second (possibly retiring it as early as 2027). If UTC stops tracking UT1, civil UTC+7 slowly drifts
  from the astronomical time the algorithm models — same bucket as ΔT: only razor-edge relevant,
  unresolvable until Vietnam's authorities say which timescale the calendar follows.
- **No official Vietnamese tables exist past ~2100.** Nothing from a state calendar bureau covering
  2072+ disputes surfaced. Hồ Ngọc Đức remains the de-facto ground truth; his site claims historic
  reliability "since 1301" and notes official/astronomical calendars coincide since 1976.
- **Pre-1968 is messier than "UTC+8".** Per HND's historic-calendar page: North used UTC+8 1945–67;
  South used UTC+7 until 1959 then UTC+8 1960–67; pre-1945 rests on dynastic tables (Bách trúng kinh
  1624–1799, Khâm định vạn niên thư 1554–1903). A single "UTC+8 historic mode" (open question 2 in
  known-issues) would be *wrong for South Vietnam 1955–59* — the idea is even weaker than documented.
- **Go ecosystem check.** Other Go ports (hungtrd/amlich, buichuongvnua/amlich, go-dyn/vcalendar) are
  straight HND ports without the day-0 overshoot fix or input validation this repo already has.
  Nothing to borrow; this implementation is ahead of them.

## Brainstorm: Evaluated Approaches

### Recommend — small, honest, non-breaking

**1. Razor-edge caveat in bot replies** (resolves known-issues open question 1)
- What: hardcode the 7 disputed month-boundary JDs (09/12/2072, 15/11/2077, 07/05/2130, 26/05/2150,
  17/05/2159, 22/01/2175, 26/01/2199). When a conversion's month start or next-month start is one of
  them, append one line: result near a disputed lunar-month boundary, may differ ±1 day from future
  official tables.
- Affects ~413 of 146,097 days; zero risk to correct output; converts a silent known-wrongness into
  stated uncertainty. Cost: a small table + one condition + tests.
- Trade-off: nobody realistically queries 2130 from a Telegram bot — pure-YAGNI reading says skip.
  But the cost is ~30 LOC and it closes a documented open question permanently.

**2. Leap-month ambiguity hint in `/duonglich`** (fixes the "correct output reported as bug" item)
- What: when the resolved (possibly defaulted) month equals that lunar year's leap month and no
  `nhuan` flag was given, append a hint: "Năm nay có tháng X nhuận — thêm 'nhuan' nếu ý bạn là
  tháng nhuận." Detection is one `getLeapMonthOffset` call on the already-computed a11.
- Alternatives considered: (a) reply with *both* conversions — noisier, two answers where user wants
  one; (b) leave as-is — keeps a documented user-confusion source. Hint is the KISS winner:
  single authoritative answer + self-service disambiguation.

**3. Golden-table regression corpus** (optional hardening)
- What: generate once, from the current verified engine, a compact per-year record (leap-month index +
  12/13 month lengths) for all 400 years; commit as testdata; test decodes and compares.
- Why round-trip isn't enough: `TestSolarLunarRoundTrip` proves *self-consistency*; a future change
  could shift a month boundary consistently in both directions and pass. `knownDates` +
  `TestLeapMonthTable` pin samples only. A golden table freezes the full verified behavior and makes
  any future engine experiment a reviewable one-file diff.
- Cost: ~1 generator run + ~4 KB testdata + one test. Verdict: worth it, do alongside #1.

### Reject — with reasons pinned

**ΔT model update.** New IERS data makes the polynomial *more* wrong, yet updating it is still wrong to
do: the module's value is bit-compatibility with the reference algorithm every Vietnamese app runs.
A "better" ΔT flips razor-edge dates away from ecosystem consensus → user-visible mismatches
reported as bugs, with no authority to say we're right. Revisit only if official 2072+ tables appear
(known-issues open question 4). Approach #1 (caveat) is the correct treatment of this uncertainty.

**Pre-1968 historic mode.** Ground truth fragments by government (North/South differ 1955–67, dynastic
before 1945); a correct implementation is a research project, not a module feature; the proleptic
astronomical calendar is what every reference source shows for those years anyway. Current behavior
already matches the published record on both verifiable disputes. Keep documented, don't build.
This *strengthens* the known-issues answer to open question 2: even a user-reported mismatch should
trigger a doc note, not a UTC+8 mode.

**Table-driven rewrite.** A table must be generated from something. From this engine → just a cache of
identical output (Go float64 JD math has ~4.5e-10-day ulp vs 0.0014-day decision margins; no
platform-flip risk to cache away). From official tables → they don't exist past ~2100. Table-driven
is how you'd start from scratch; with a verified engine it adds a second representation to keep in
sync (DRY violation) for zero accuracy. The useful 20% of this idea is #3 (table as *test* data).

**Range extension beyond 1800–2199.** Bound exists because published references stop there; claims
outside are unverifiable. Nothing found changes that.

**Feature creep** (ngày can-chi, tiết khí, giờ hoàng đạo, holiday lookup): out of scope until a user
asks. Noted so it isn't re-brainstormed.

## Success Criteria (if #1–#3 are implemented)

- All existing tests pass unchanged, incl. `knownDates` pins (20/6/1944, 7/7/1967).
- Caveat appears for a 2072+ razor-edge date, absent for ordinary dates (both directions of conversion).
- `/duonglich 5/5/2028` (leap-5 year) shows hint; `/duonglich 5/5/2028 nhuan` and non-leap years don't.
- Golden table regenerated from HEAD is byte-identical to committed testdata.

## Next Steps

1. Decide which of #1/#2/#3 to implement (recommendation: all three; #2 first — most user-visible).
2. `/ck:plan` with this report as context if proceeding; scope is small enough for a single phase.
3. Update `docs/amlich-known-issues.md` open questions 1–3 with the resolutions above once implemented.

## Sources

- [Hồ Ngọc Đức — Vietnamese lunar calendar](https://www.xemamlich.uhm.vn/vncal_en.html)
- [Hồ Ngọc Đức — Historic Vietnamese lunar calendar](https://www.xemamlich.uhm.vn/histcal.html)
- [Vietnamese calendar — Wikipedia](https://en.wikipedia.org/wiki/Vietnamese_calendar)
- [IERS: no leap second in 2024 — DCD](https://www.datacenterdynamics.com/en/news/no-leap-seconds-added-to-universal-time-in-2024-iers-says/)
- [Earth rotation records spur Oct 2026 CGPM vote — TechTimes](https://www.techtimes.com/articles/320185/20260711/earth-rotation-records-spur-october-vote-avert-negative-leap-second.htm)
- [Negative leap second outlook — timeanddate](https://www.timeanddate.com/time/negative-leap-second-maybe.html)
- [Earth rotation acceleration analysis — Astronomy Reports 2024](https://arxiv.org/html/2404.06343v3)
- Go ports surveyed: [hungtrd/amlich](https://github.com/hungtrd/amlich), [buichuongvnua/amlich](https://pkg.go.dev/github.com/buichuongvnua/amlich), [go-dyn/vcalendar](https://pkg.go.dev/github.com/go-dyn/vcalendar)

## Unresolved Questions

1. Caveat wording/threshold for #1: flag only the two months touching a disputed boundary (proposed),
   or the whole lunar year? Proposed: months only — year-wide is alarmist.
2. Should the leap-month hint (#2) also fire when month+`nhuan` *was* given but the defaulted year
   was filled in (user may have meant a different year)? Proposed: no — over-engineering.
3. If leap seconds are abolished (Oct 2026 vote), does Vietnam's calendar follow civil UTC+7 or
   UT1+7? Unanswerable today; park with known-issues open question 4.
