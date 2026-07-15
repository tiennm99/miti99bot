---
date: 2026-07-15
session: readable-adaptive-wheel-labels-brainstorm
---

# Journal: 2026-07-15 — Readable Adaptive Wheel Labels

## Context

The label `Chiều nay uống CraneTea` remains a single 9px line on a 512px wheel. The current layout wraps only when text reaches the 8px hard minimum, so it treats technical fit as sufficient even when the result is difficult to read.

## What Happened

- Reframed the issue as a readability-threshold problem rather than simply a newline problem.
- Approved 14px as the preferred readable font-size threshold.
- Agreed to wrap labels into two or three balanced lines when slice geometry safely permits.
- Preserved the complete smaller label as the intentional fallback for dense wheels where multiline text would overlap adjacent slices.
- No code was implemented during this brainstorm.

## Reflection

The existing 8px rule protects against overflow but not readability. Separating the 14px preferred threshold from the 8px hard minimum makes the intended behavior explicit: use available vertical space when geometry permits, while acknowledging that dense wheels cannot provide large text, full content, and non-overlap simultaneously.

## Decisions Made

| Decision | Rationale | Impact |
|---|---|---|
| Use 14px as the preferred threshold | Fixes the unreadable 9–13px one-line range | Medium-length labels wrap sooner |
| Wrap only when two or three lines fit | Prevents adjacent-slice overlap | Layout remains bounded across supported densities |
| Preserve full smaller text for dense wheels | Avoids truncation and API changes | Complete labels remain visible when geometry cannot support wrapping |

## Next Steps

- Create a tests-first implementation plan with `/ck:plan --tdd` using the approved brainstorm report.
