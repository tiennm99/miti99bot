---
phase: 1
title: Tests First and Implementation
status: completed
priority: P1
dependencies: []
effort: small
---

# Phase 1: Tests First and Implementation

## Overview

Lock the desired readability behavior in failing unit regressions, then make the smallest layout-helper change that separates preferred readability from hard minimum fit.

## Context Links

- [Approved brainstorm](../reports/260715-1301-readable-adaptive-wheel-labels-brainstorm.md)
- [Completed wrapping plan](../260715-1047-bold-wrapped-wheel-labels/plan.md)
- [Project README](../../README.md)

## Requirements

- Functional: prefer 14px; wrap below that threshold when `maxLines >= 2`; keep full text at the 8px hard fallback when only one line fits safely.
- Functional: the 512px example wraps for 2–16 options when geometry permits; 24–32 options stay bounded and complete.
- Non-functional: deterministic output; Unicode-safe breaks; no public contract, theme, or geometry change.

## Architecture

Keep decision-making inside `getRadialLabelLayout`. Compute one-line fit first, compare it with a named 14px preferred threshold, and call the existing deterministic `getLabelLines` path only when geometry allows multiple lines. Continue sizing from the longest chosen line and bounding height by tangential arc availability. Do not add renderer state or a second wrapping implementation.

## Related Code Files

| Action | Absolute path | Purpose |
|---|---|---|
| Modify | `C:\Users\miti99\Workspaces\tiennm99\wheelofnames\test\wheel-label-layout.test.js` | Add failing threshold, boundary, Unicode, short-label, and dense-layout regressions. |
| Modify | `C:\Users\miti99\Workspaces\tiennm99\wheelofnames\src\remotion\wheel-label-layout.js` | Separate 14px preferred readability from the 8px hard minimum and geometry-gate wrapping. |
| Conditional modify | `C:\Users\miti99\Workspaces\tiennm99\wheelofnames\src\remotion\WheelComposition.jsx` | Only if implementation proves new layout metadata is required; otherwise leave unchanged. |

## Implementation Steps

### Tests Before

1. Add a table-driven regression for `Chiều nay uống CraneTea` at size 512 and representative supported counts across 2–16. Assert multiple lines only when computed geometry permits, full text reconstruction, bounded height, and wrapped `fontSize >= 14`.
2. Add targeted cases whose current one-line fit lands at 9, 10, 11, 12, and 13px. Assert they no longer remain one line when `maxLines >= 2`.
3. Run `pnpm test -- test/wheel-label-layout.test.js`; record that new regressions fail against the current 8px-only trigger for the intended reason.

### Refactor

4. Introduce a named preferred label font size of 14px beside the existing 8px minimum; keep their responsibilities separate.
5. Change the wrap decision from equality with the hard minimum to `singleLineFontSize < preferredLabelFontSize && maxLines >= 2`.
6. Reuse `getLabelLines`, longest-line sizing, content width, and height bounding. Avoid API changes and avoid touching `WheelComposition.jsx` unless metadata is strictly necessary.
7. Run the focused test file until the tests-before regressions pass.

### Tests After

8. Add or refine boundary coverage: exactly 14px remains one line; 13px wraps when permitted; one-line-only geometry preserves full text at the smaller fallback.
9. Retain regressions for 384px/512px short labels on one line, 24–32 dense layouts bounded, whitespace normalization, unbroken tokens, and Vietnamese/emoji grapheme reconstruction without broken segments.
10. Assert no selected layout exceeds available tangential height and all selected lines fit padded `contentWidth` under the estimator.

### Regression Gate

11. Run `pnpm test -- test/wheel-label-layout.test.js`.
12. Run `pnpm typecheck` and `pnpm lint` before phase completion.

## Todo List

- [x] Tests-before cases fail for the existing 9–13px blind spot.
- [x] Preferred and hard-minimum constants have distinct roles.
- [x] Geometry-gated wrapping passes exact example and boundary tests.
- [x] Unicode, short-label, and dense fallback regressions pass.
- [x] Focused tests, typecheck, and lint pass.

## Success Criteria

- [x] `Chiều nay uống CraneTea` wraps at 512px for 2–16 options whenever `maxLines >= 2`, reconstructs exactly, and targets at least 14px.
- [x] Short labels remain one line at supported sizes.
- [x] 24–32 option layouts preserve full text at the smaller fallback and remain height-bounded.
- [x] Vietnamese graphemes and existing unbroken-token behavior remain intact.
- [x] No API/schema, theme, geometry, or winner metadata code changes.
- [x] Focused tests, typecheck, and lint pass.

## Risk Assessment

- Risk: a hard 14px trigger may wrap labels that are only marginally smaller. Mitigation: exact 13/14px boundary tests and short-label fixtures.
- Risk: wrapping at 16 options may exceed tangential space. Mitigation: derive `maxLines` from existing arc height and assert the final box stays bounded.
- Risk: width estimation can split Vietnamese incorrectly. Mitigation: preserve `Intl.Segmenter` and assert exact grapheme-safe reconstruction.

## Security Considerations

No new input, I/O, authentication, or external dependency surface. Existing request validation remains unchanged.

## Next Steps

Proceed to renderer verification only after the regression gate is green.
