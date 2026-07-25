# Phase 03 — Label orientation at the vertical case

## Context

- `src/remotion/wheel-label-layout.js` — `getRadialLabelLayout`, the rest-frame flip
- `src/remotion/WheelComposition.jsx` — renders `label.rotation`
- `test/wheel-label-layout.test.js`

The rest-frame flip added in the previous pass makes every label upright when the
wheel stops, except at the boundaries. A label's rest angle is
`(centerDegrees - winnerCenterDegrees) mod 360`, and the flip triggers strictly
inside (90, 270). A label sitting at exactly 90 or 270 is not mirrored, so it is
not flipped, but it renders stacked vertically.

Observed in the 8-option sample: `heidi` and `dave` read vertically while every
other label is horizontal.

## When it happens

Rest angles are `360 * (i - winnerIndex) / optionCount`. Exactly 90 or 270
requires `(i - winnerIndex) / optionCount` to be 1/4 or 3/4, so:

- `optionCount` divisible by 4: exactly two labels land vertical, every time.
- Otherwise: no label is exactly vertical, but the nearest is within
  `180 / optionCount` degrees of it, which is still steeply tilted on dense
  wheels.

So this is not a rare edge case. Every 4, 8, 12, 16, 20, 24, 28, 32-option wheel
has two vertical labels in its poster frame.

## Options

### A. Consistent vertical direction (cheap)

Move the flip boundary so 90 and 270 resolve the same way, e.g. flip on
`restAngle >= 90 && restAngle < 270`. Both vertical labels then read in the same
direction instead of opposite ones, so a viewer tilts their head one way for
both. Two characters of change, no layout work, no new failure modes. Does not
make them horizontal.

### B. Tangential fallback for near-vertical labels (real fix)

When a label's rest angle is within a threshold of vertical, lay it out
tangentially — rotated 90 degrees from radial — so it reads horizontally at rest.
This means a different track geometry for those labels: the available box becomes
the slice's chord width rather than its radial length, so the existing
`getLabelTrack` and wrap logic do not directly apply.

Cost: a second layout path in a module that is already the most intricate in the
codebase, and it partially reopens the settled radial-labels decision. Benefit:
every label horizontal in the poster frame.

### C. Accept it

Radial labels on a wheel are conventional, including on wheelofnames.com, and a
vertical label is legible with a head tilt. Document and move on.

## Recommendation

Start with A, then judge B from a render. A is nearly free and strictly better
than today. B is only worth its complexity if the vertical labels actually look
wrong in the poster frame at 200px, which is the size that matters — check that
before building it.

Whichever is chosen, it must not regress the rest-frame flip guarantee: the
existing test asserting no label rests mirrored has to keep passing.

## Tests

- No label rests with a screen angle strictly inside (90, 270), unchanged.
- Labels at exactly 90 and 270 resolve to the same reading direction.
- If B is built: no label rests within 15 degrees of vertical, for option counts
  2..32 and winner indices across the range.

## Validation

Render 4, 8, 12, 16 options and inspect the poster frame at both 512 and a
200px downscale, which is the realistic chat display size.
