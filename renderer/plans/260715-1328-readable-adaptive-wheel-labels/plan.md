---
title: Readable Adaptive Wheel Labels
description: >-
  Wrap medium-length wheel labels at a preferred 14px readability threshold
  while preserving bounded full-text fallback for dense wheels.
status: completed
priority: P2
branch: main
tags:
  - bugfix
  - frontend
  - renderer
  - tdd
blockedBy: []
blocks: []
created: '2026-07-15T06:29:40.959Z'
createdBy: 'ck:plan'
source: skill
---

# Readable Adaptive Wheel Labels

## Overview

Fix the 9–13px readability blind spot in radial wheel labels. Keep 8px as the hard full-text fallback, but prefer deterministic two/three-line wrapping when the one-line result is below 14px and slice geometry permits. Preserve short-label behavior, Vietnamese graphemes, API contracts, themes, and wheel geometry.

Approved design: [Readable Adaptive Wheel Labels Brainstorm](../reports/260715-1301-readable-adaptive-wheel-labels-brainstorm.md).

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Tests First and Implementation](./phase-01-tests-first-and-implementation.md) | Completed |
| 2 | [Renderer Verification](./phase-02-renderer-verification.md) | Completed |

## Dependencies

- Cross-plan dependencies: none. The completed [Bold Wrapped Wheel Labels](../260715-1047-bold-wrapped-wheel-labels/plan.md) plan is implementation history, not a blocker.
- Runtime: existing JavaScript + JSDoc, React, Remotion, and CSS renderer stack.
- Quality gates: Node.js 24, `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm render:smoke`, and Vietnamese visual evidence.

## Scope

- Primary touchpoints: `src/remotion/wheel-label-layout.js` and `test/wheel-label-layout.test.js`.
- Fixture input may change only when needed for the Vietnamese visual witness.
- `WheelComposition.jsx` changes only if layout metadata is proven necessary.
- No API/schema, theme, wheel geometry, truncation, custom font, or curved-text changes.

## Acceptance Criteria

- At 512px, `Chiều nay uống CraneTea` wraps for 2–16 options whenever geometry permits; wrapped text targets at least 14px.
- Ordinary short labels remain one line.
- 24–32 option layouts remain bounded and preserve complete text at the smaller fallback size.
- Vietnamese grapheme clusters remain intact.
- No API/schema, theme, winner metadata, or wheel geometry changes.
- Full automated gates and visual renderer evidence pass; generated artifacts are removed.

## Risks

- Estimated width may differ from browser glyph width. Mitigate with exact unit boundaries plus rendered Vietnamese evidence.
- Threshold changes can over-wrap short labels. Mitigate with 384px and 512px one-line regressions.
- Dense slices cannot satisfy large text and non-overlap simultaneously. Mitigate with an explicit bounded full-text fallback.
