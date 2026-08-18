---
phase: 2
title: Handler replies
status: completed
effort: S
priority: P2
dependencies:
  - 1
---

# Phase 2: Handler replies

## Overview

Wire the leap-month hint into `/duonglich` and the razor-edge caveat into both commands. Reply
sentences gain optional trailing lines only; existing first-line format is unchanged.

## Requirements

- Functional (`/duonglich` hint): after a successful conversion with `leap == false`, if
  `lunarToSolar(day, month, year, true)` returns nil error, append:
  `Lưu ý: năm âm lịch <year> có tháng <month> nhuận — thêm "nhuan" nếu ý bạn là tháng nhuận.`
- Functional (caveat, both commands): if `nearDisputedBoundary(jd)` — where `jd` is the *solar* JD
  of the queried/resulting date — append:
  `Lưu ý: ngày này gần ranh giới tháng âm lịch chưa chắc chắn; kết quả có thể lệch 1 ngày so với lịch chính thức sau này.`
- Hint NOT shown when `nhuan` was explicit (user already disambiguated) or when the exact leap date
  doesn't exist. Both lines may co-occur (hint first, then caveat), each on its own line after the
  main sentence, separated by `\n`.
- Non-functional: no change to usage strings, error paths, or year-range checks.

## Architecture

- `/amlich`: `jd := jdFromDate(day, month, year)` (already-parsed solar input) → caveat check.
- `/duonglich`: caveat check on `jdFromDate(solarDay, solarMonth, solarYear)` from the conversion
  result; hint check via the leap-variant probe before assembling the reply.
- Message constants live next to the usage constants in `handlers.go`.

## Related Code Files

- Modify: `internal/modules/amlich/handlers.go`
- Modify: `internal/modules/amlich/handlers_test.go`

## Implementation Steps

1. Add the two message constants (hint as format string taking year + month).
2. `/duonglich`: after successful `lunarToSolar`, build reply, conditionally append hint (probe
   with `leap=true` only when input `leap == false`), then conditionally append caveat.
3. `/amlich`: conditionally append caveat after the main sentence.
4. Tests (extend existing fake-reply pattern in `handlers_test.go`):
   - `/duonglich 5/5/2028` → hint present (2028 has leap 5); `/duonglich 5/5/2028 nhuan` → absent;
     `/duonglich 5/5/2027` → absent (no leap 5 in 2027).
   - `/amlich` on a date inside the disputed month at 09/12/2072 → caveat present; ordinary date →
     absent. `/duonglich` case whose result lands in that month → caveat present.
   - Assert exact full reply strings (repo test style asserts exact text — keep it strict).

## Success Criteria

- [ ] Hint behavior matches the three `/duonglich` cases above
- [ ] Caveat fires for disputed-month dates in both commands, silent elsewhere
- [ ] All pre-existing handler tests pass without assertion loosening

## Risk Assessment

- Double-probe cost: one extra `lunarToSolar` call per leap-year query — microseconds, irrelevant.
- Wording is user-visible contract; if the user wants different Vietnamese phrasing, only the
  constants change. Flag wording in the PR description for review.
