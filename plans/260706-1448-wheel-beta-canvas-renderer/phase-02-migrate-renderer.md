---
phase: 2
title: Migrate Renderer
status: completed
effort: ''
priority: P1
dependencies:
  - 1
---

# Phase 2: Migrate Renderer

## Overview

Move wheel beta frame drawing onto `tdewolff/canvas` while retaining the current frame-generation functions and observable command behavior.

## Requirements

- Functional: `renderWheelBetaFrame` returns frames with same size, palette expectations, pointer alignment, status band, labels, and celebration states.
- Non-functional: keep renderer headless, deterministic, and local to misc module.

## Architecture

The spin profile and GIF assembly stay unchanged. The per-frame renderer creates a canvas-backed raster image, draws the wheel using paths/text/transforms, converts the result to the existing GIF palette, then returns `*image.Paletted` for `gif.EncodeAll`.

```text
spin profile -> canvas frame renderer -> paletted frame -> image/gif.EncodeAll
```

## Related Code Files

- Modify: `internal/modules/misc/wheelofnames_beta.go`
- Modify: `internal/modules/misc/wheelofnames_beta_drawing.go`
- Modify: `internal/modules/misc/handlers_test.go` only if internal rendering tests need adaptation.

## Implementation Steps

1. Add canvas frame creation and raster-to-paletted conversion helpers.
2. Port slices, rim, pointer, highlight, status band, title/status text, labels, and celebration drawing to canvas-backed helpers.
3. Preserve current constants where possible so tests and behavior remain stable.
4. Keep `renderWheelBetaFrame`, `renderWheelBetaFrameWithStatus`, and `renderWheelBetaFrameWithCelebration` call sites intact.
5. Remove obsolete manual text-mask helpers only if no tests depend on them; otherwise update tests to target observable behavior.

## Success Criteria

- [x] Existing GIF timing/frame transition tests pass.
- [x] Existing pointer and winner slice tests pass.
- [x] Existing label placement/rotation and Vietnamese glyph tests pass.
- [x] No command registration, caption, filename, dimensions, or duration changes.

## Risk Assessment

Risk: palette conversion can shift exact color indexes and break tests. Mitigation: keep final paletted frame using `wheelBetaPalette` and adjust only tests that were over-coupled to manual-pixel internals.

Risk: canvas text APIs may require font face plumbing that is heavier than expected. Mitigation: keep embedded font bytes and use canvas text/font APIs only within misc renderer.
