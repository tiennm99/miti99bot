# Phase 01 — Motion truth

Make the spin behave like a physical wheel at every option count: never advance
more than one slice per frame, and stop with a settle instead of an asymptotic
crawl.

## Context

- `src/remotion/wheel-layout.js` — `getSpinProgress`, `getFinalWheelRotationDegrees`,
  `defaultFullTurns`, `getPointerDeflectionDegrees`
- `src/remotion/WheelComposition.jsx:52-62` — computes `finalRotation` and `rotation`
- `test/wheel-layout.test.js` — existing invariant tests

## Problem 1: dense wheels run backwards

Peak spin speed is ~39.9 deg/frame at every option count, because total rotation
is fixed at 5 turns regardless of how wide a slice is. Measured aliasing frames
out of 98, at the 512/15fps default:

| Options | 2 | 8 | 10 | 12 | 16 | 24 | 32 |
|---|---:|---:|---:|---:|---:|---:|---:|
| Frames advancing >1 slice | 0 | 0 | 8 | 21 | 38 | 56 | 65 |

At 32 options two thirds of the spin reads as jitter or reverse rotation.

### Fix: derive turn count from the frame budget

Peak speed in progress units is the slope at the launch/deceleration handoff,
already named `handoffSlope` in `wheel-layout.js`. Requiring
`handoffSlope * turns * 360 / spinFrames <= 360 / optionCount` gives:

```js
export const getSpinTurns = (spinFrames, optionCount) =>
  Math.max(1, Math.min(7, Math.floor(spinFrames / (handoffSlope * Math.max(1, optionCount)))));
```

**Floor, not round.** Rounding to nearest pushes 8 options from 5.63 to 6 turns
and peak to 47.9 deg/frame against a 45 degree slice — worse than today.
Flooring keeps the current 5 turns for the default wheel.

**Must stay an integer.** The winner lands under the pointer only because
`turns * 360 mod 360 === 0`. A fractional 2.8 turns leaves the wheel 288 degrees
off. Any change here must preserve integrality.

Resulting turn counts and outcomes:

| Config (spin frames) | n=2 | n=8 | n=12 | n=16 | n=24 | n=32 |
|---|---|---|---|---|---|---|
| default 6500/15fps (98) | 7 | 5 | 3 | 2 | 1 | 1 |
| 6500/12fps (78) | 7 | 4 | 2 | 2 | 1 | 1 |
| max 10000/20fps (200) | 7 | 7 | 7 | 5 | 3 | 2 |
| min 3000/12fps (36) | 7 | 2 | 1 | 1 | 1 ALIAS | 1 ALIAS |

Only the minimum-duration dense cases still alias: 36 spin frames cannot carry
even one turn past 24 slices. Assert that boundary rather than pretend it is
solved.

Wire it in `WheelComposition.jsx` by passing
`getSpinTurns(spinFrames, props.options.length)` into
`getFinalWheelRotationDegrees`, and keep `defaultFullTurns` as the fallback for
direct callers.

### Risk to validate before accepting

At 32 options with the default duration this yields **1 turn over 6.5 seconds**.
Correct by the aliasing metric, but it may read as a broken or barely-moving
wheel rather than a slow one. Render 24 and 32 options and judge before
committing. If it looks wrong, the fallback is a floor of 2 turns with the
remaining aliasing documented — a deliberate choice of drama over correctness on
dense wheels, not a silent regression.

## Problem 2: the wheel stops without settling

`getSpinProgress` approaches 1 asymptotically, so the wheel eases to a halt with
no impact. Real wheels overshoot the final detent slightly and rock back.

### Fix: damped recoil in the deceleration tail

Add a decaying oscillation to the last portion of the spin that is exactly zero
at `frame === spinFrames`, so the landing guarantee is untouched:

```js
const recoilFraction = 0.18;   // last 18% of the spin
const recoilDegrees = 2.2;     // peak overshoot, scaled below
```

Apply the recoil as a rotation offset in degrees rather than inside
`getSpinProgress`, so progress stays monotonic in [0, 1] and the existing
invariant tests keep their meaning. The offset must:

- be 0 for `frame <= spinFrames * (1 - recoilFraction)`
- be exactly 0 at `frame >= spinFrames`
- peak around half a slice at most, and never more than `recoilDegrees`, so a
  neighbouring slice never touches the pointer

Scale the peak by `Math.min(recoilDegrees, sliceDegrees * 0.25)` so a 2-option
wheel does not visibly swing.

Because the celebration fires at `revealFrame` (about 5 frames before
`spinFrames`), the recoil and the confetti burst overlap. That is the desired
read: impact, recoil, confetti.

### Interaction with the pointer flapper

`getPointerDeflectionDegrees` is driven by `rotation`. Feeding it the recoiled
rotation makes the flapper twitch during the settle, which is free realism, but
the rest-neutral proof depends on the final rotation being exactly the detent
value. Since the recoil is exactly 0 at `spinFrames` and beyond, the proof holds.
Add a test rather than relying on that argument.

## Tests

- `getSpinTurns` returns an integer in [1, 7] for every combination of
  `spinFrames` in {36, 78, 98, 200} and option counts 2..32.
- Winner lands at exactly screen angle 0 for every option count 2..32 and every
  winner index, using the adaptive turn count.
- No frame advances more than one slice, for all option counts, except the
  documented minimum-duration dense cases, which are asserted explicitly as
  known-aliasing so the boundary cannot move silently.
- Recoil offset is exactly 0 at and after `spinFrames`, and never exceeds a
  quarter slice.
- Flapper still parks at exactly 0 at rest with recoil applied.
- Existing tests updated: `caps peak spin speed regardless of option count` and
  `never advances a full slice in one frame at typical option counts` now cover
  all counts, so the "typical" qualifier goes away.

## Validation

Render 2, 8, 16, 24, 32 options at the default, plus the minimum and maximum
duration cases. Confirm the winner sits under the pointer in the last frame of
each, and judge the 24/32 slowness question above.
