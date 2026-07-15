---
phase: 1
title: Implementation
status: completed
priority: P1
dependencies: []
effort: small
---

# Phase 1: Implementation

## Overview

Add deterministic wrapped-label metadata to the pure layout helper and consume it in the Remotion composition. Keep the change local to wheel label presentation.

## Requirements

- Functional: split long labels into balanced lines, retain one line for short labels, size against the longest output line, render at bold weight, and clip overflow.
- Non-functional: deterministic output across frames; JavaScript + JSDoc; no public API, theme, or wheel geometry changes.

## Architecture

`getRadialLabelLayout()` remains the composition-facing boundary. Extend its return value with the rendered line list and a bounded text-block height. A pure wrapping helper should normalize whitespace, prefer word boundaries, and choose the most balanced split whose longest estimated line best fits the radial track. Preserve text for unbroken long tokens with a deterministic fallback rather than relying on browser-dependent wrapping. Pass the longest returned line to `getLabelFontSize()` so wrapping reduces shrinkage without removing the existing minimum-size safeguard.

`WheelComposition.jsx` renders the precomputed lines in a centered block using `fontWeight: 700`, compact line height, and explicit width/height plus `overflow: hidden`. Do not use ellipsis or `nowrap`, because the helper owns the line breaks.

## Related Code Files

- Modify: `C:/Users/miti99/Workspaces/tiennm99/wheelofnames/src/remotion/wheel-label-layout.js` — wrapping, longest-line sizing, returned layout metadata/JSDoc.
- Modify: `C:/Users/miti99/Workspaces/tiennm99/wheelofnames/src/remotion/WheelComposition.jsx` — bold multi-line rendering and clipping.
- Create: none.
- Delete: none.

## Implementation Steps

1. Add a small exported pure helper for label-line selection. Return `[text]` when the base-size estimate fits; otherwise evaluate sensible word-boundary splits and select balanced lines by minimizing the longest estimated line, with deterministic handling for whitespace and single long tokens.
2. Update `getRadialLabelLayout()` and its JSDoc shape to return `lines` and a bounded label-block height while computing `fontSize` from the longest returned line.
3. Update the label element in `WheelComposition.jsx` to render the prepared lines at weight `700`, center them with compact multi-line spacing, and clip to the returned width/height.
4. Preserve radial coordinates, rotation, slice rendering, color contrast, and all component props.

## Success Criteria

- [x] Short text yields one line and keeps the normal base size when space permits.
- [x] Long phrase yields balanced multiple lines and a larger readable size than whole-label fitting would allow.
- [x] Font size is calculated from the longest returned line.
- [x] Rendered labels are bold, centered, and clipped inside their label box.
- [x] No API/schema, theme, or wheel geometry file changes.

## Risk Assessment

- Dense wheels can have limited cross-track height. Mitigate with a bounded line count/height and clipping derived from existing label geometry.
- Width estimates are approximate and bold glyphs are wider. Keep conservative padding and verify the rendered smoke GIF.
- Unbroken or non-Latin text may not have word boundaries. Use deterministic grapheme-safe fallback behavior and cover it with a focused unit case if the helper supports splitting it.

## Security Considerations

No new input surface or HTML injection path. React continues to escape label text; preserve request validation and do not use raw HTML.
