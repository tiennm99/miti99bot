---
date: 2026-07-15
session: bold-wrapped-wheel-labels
---

# Journal: 2026-07-15 — Bold Wrapped Wheel Labels

## Context

Wheel labels needed bold typography and automatic line wrapping so long entries remained readable without changing the GIF API or wheel geometry.

## What Happened

- Added bold, Unicode-safe multiline label layout with balanced word-boundary wrapping and bounded line height.
- Independent review exposed a 384px regression: short labels such as `alpha` wrapped because the first implementation wrapped before trying the supported one-line font-size range.
- Corrected the layout order to preserve one-line text whenever it fits at 8px or larger, then wrap only when necessary.
- Measured fit against content width after excluding horizontal padding, preventing padding from being counted twice.
- Validation passed: lint, typecheck, 29 tests, render smoke and visual inspection, a 135-case layout probe, and independent review scored 9.7/10.

## Reflection

The initial wrapping feature handled long content but optimized the wrong constraint first. Minimum-size rendering needed explicit regression coverage at the smallest supported wheel size. Separating content width from padding and defining the 8px one-line threshold made the behavior deterministic across short, long, and unbroken labels.

## Decisions Made

| Decision | Rationale | Impact |
|----------|-----------|--------|
| Attempt one-line fit before wrapping | Short names should remain intact at supported sizes | Prevents unnecessary wrapping at 384px |
| Use 8px as the one-line fit threshold | Preserves readability while allowing modest shrinking | Long labels wrap only after the readable one-line range is exhausted |
| Exclude horizontal padding from content width | Layout measurements must represent the actual text box | Avoids double-counting padding and false overflow |
| Keep the public API unchanged | The change is internal presentation behavior | Existing clients, routes, and winner metadata remain compatible |

## Next Steps

- No follow-up required; retain the minimum-size and multiline cases as regression coverage.
