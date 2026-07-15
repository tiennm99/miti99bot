---
phase: 2
title: Verification
status: completed
priority: P1
dependencies:
  - 1
effort: small
---

# Phase 2: Verification

## Overview

Lock the layout contract with focused unit tests, then verify static quality gates and actual Remotion output.

## Requirements

- Functional: tests witness one-line, balanced multi-line, and longest-line sizing behavior.
- Non-functional: all required repository gates pass; rendered smoke output completes without label-layout regressions.

## Architecture

Unit tests exercise the pure helpers so line-selection and sizing regressions fail cheaply. `render:smoke` is the visual/runtime witness for React styles, clipping, Chromium font layout, and GIF generation.

## Related Code Files

- Modify: `C:/Users/miti99/Workspaces/tiennm99/wheelofnames/test/wheel-label-layout.test.js` — focused wrapping and sizing assertions.
- Witness: `C:/Users/miti99/Workspaces/tiennm99/wheelofnames/scripts/render-fixtures.js` via `pnpm render:smoke` — no planned script change.
- Create: none.
- Delete: none.

## Implementation Steps

1. Replace the old whole-label shrink expectation with assertions that a short label remains one line and a long phrase wraps into balanced lines.
2. Assert the returned font size matches sizing based on the longest rendered line and is not based on the unsplit full label.
3. Add edge coverage for whitespace normalization and/or a long unbroken label according to the implemented helper contract.
4. Run `pnpm lint`, `pnpm typecheck`, and `pnpm test`.
5. Run `pnpm render:smoke`; inspect command success and confirm the generated smoke frame/GIF does not overflow or truncate labels unexpectedly. Do not commit generated GIFs or temporary render artifacts.

## Success Criteria

- [x] `pnpm lint` passes.
- [x] `pnpm typecheck` passes.
- [x] `pnpm test` passes with focused wrapping witnesses.
- [x] `pnpm render:smoke` passes and visually witnesses bold, readable labels without clipping.
- [x] `git diff` contains only the planned source, test, and plan changes; no generated GIFs or secrets.

## Risk Assessment

- A smoke fixture may not contain a sufficiently long label. If its existing input cannot witness wrapping, make the smallest fixture-input adjustment without changing production API behavior; never commit the generated GIF.
- Font rendering varies by host. Treat pure-helper tests as the deterministic contract and smoke rendering as integration evidence.

## Security Considerations

No security behavior changes. Ensure test fixtures contain only synthetic labels and no private data.
