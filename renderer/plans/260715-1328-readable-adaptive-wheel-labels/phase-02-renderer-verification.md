---
phase: 2
title: Renderer Verification
status: completed
priority: P1
dependencies:
  - 1
effort: small
---

# Phase 2: Renderer Verification

## Overview

Run the full project gates and produce a targeted Vietnamese rendering witness that confirms readable wrapping without clipping or adjacent-slice overlap.

## Context Links

- [Approved brainstorm](../reports/260715-1301-readable-adaptive-wheel-labels-brainstorm.md)
- [Project README](../../README.md)
- [Render benchmarks](../../docs/render-benchmarks.md)

## Requirements

- Functional: verify exact Vietnamese text visually at 512px and low/medium supported density.
- Non-functional: all repository gates pass; public API contracts remain unchanged; no generated GIF remains in version control or the worktree.

## Architecture

Verification exercises the pure layout helper through Vitest and the existing Remotion render pipeline through project scripts. Prefer changing only a Vietnamese fixture input to include the exact phrase; do not add a new renderer path. API compatibility is established through existing schema/route tests and diff inspection.

## Related Code Files

| Action | Absolute path | Purpose |
|---|---|---|
| Verify | `C:\Users\miti99\Workspaces\tiennm99\wheelofnames\src\remotion\wheel-label-layout.js` | Confirm final threshold and geometry logic. |
| Verify | `C:\Users\miti99\Workspaces\tiennm99\wheelofnames\test\wheel-label-layout.test.js` | Confirm all targeted and regression cases. |
| Conditional modify | `C:\Users\miti99\Workspaces\tiennm99\wheelofnames\scripts\render-fixtures.js` | Smallest fixture-input-only change needed to render `Chiều nay uống CraneTea`. |
| Verify only | `C:\Users\miti99\Workspaces\tiennm99\wheelofnames\src\remotion\WheelComposition.jsx` | Confirm bold rendering consumes existing `lines`, `fontSize`, and bounded height unchanged. |
| Cleanup | `C:\Users\miti99\Workspaces\tiennm99\wheelofnames\fixtures\*.gif` | Remove generated visual artifacts after inspection. |

## Implementation Steps

1. Run `pnpm lint`, `pnpm typecheck`, and `pnpm test`; require zero failures.
2. Run `pnpm render:smoke` to protect the 384px renderer, bundle reuse, and GIF output path.
3. Render a 512px Vietnamese witness containing `Chiều nay uống CraneTea`. Prefer the existing `vietnamese` fixture with the smallest input-only edit; run `pnpm render:fixtures` when that fixture path is used.
4. Inspect the witness at a stable frame: exact graphemes present, bold text, two/three balanced lines where geometry permits, font visually readable, no radial clipping, and no adjacent-slice overlap.
5. Confirm dense behavior through unit cases for 24–32 options; render an additional dense witness only if unit bounds or visual inspection are ambiguous.
6. Run existing route/schema tests as part of `pnpm test`; inspect the diff to confirm no `/api` request/response schema, response headers, themes, or wheel geometry changed.
7. Remove all generated GIFs and temporary render artifacts. Run `git status --short` and confirm only intended source/test/optional fixture input and plan files remain.
8. Re-run any gate affected by cleanup or fixture edits.

## Todo List

- [x] Full lint, typecheck, and test suite pass.
- [x] `render:smoke` passes.
- [x] Vietnamese 512px witness passes visual inspection.
- [x] Dense 24–32 fallback remains bounded and complete.
- [x] API/schema/theme/geometry contracts remain unchanged.
- [x] Generated artifacts are removed.

## Success Criteria

- [x] `pnpm lint`, `pnpm typecheck`, and `pnpm test` pass.
- [x] `pnpm render:smoke` produces a valid GIF.
- [x] Targeted Vietnamese visual evidence confirms readable wrapping and intact graphemes without clipping/overlap.
- [x] 24–32 dense layouts retain complete labels within computed height.
- [x] Winner metadata and `/api` behavior remain covered and unchanged.
- [x] No generated GIF or temporary renderer artifact remains.

## Risk Assessment

- Risk: visual verification is subjective. Mitigation: pair it with numeric font-size, width, reconstruction, and height assertions.
- Risk: full fixture rendering is slow. Mitigation: use the existing targeted fixture and avoid adding redundant fixtures.
- Risk: generated outputs get committed. Mitigation: cleanup plus final `git status --short` inspection.

## Security Considerations

No security behavior changes. Verification must not expose tokens or persist environment files; production API authentication tests remain untouched.

## Next Steps

Mark the plan complete only after both automated and visual gates pass and artifacts are clean.
