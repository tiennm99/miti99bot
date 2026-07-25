# Phase 02 — Confetti craft

Two additions to a burst that already works: fake the motion blur a GIF cannot
have, and keep long holds populated to the end.

## Context

- `src/remotion/confetti-layout.js` — particle construction and closed-form flight
- `src/remotion/Confetti.jsx` — rendering, opacity envelope, launch gating
- `test/confetti-layout.test.js`

Current measured behaviour at 512/15fps, default hold (22 frame window):
on-screen count `25,37,46,53,59,67,70,...,70,69,68,66,62`, peak step 50.9px
against a 16.3px median body.

## Problem 1: fast chips read as disconnected dots

Peak travel is about 3.1x body length per frame in the opening frames, and a GIF
has no motion blur to tie those samples together. Real footage of a confetti
cannon shows streaks, not dots.

### Fix: elongate along the direction of travel

Stretch each particle along its current velocity vector in proportion to speed,
so fast chips become short ribbons and slow ones stay square. Deterministic and
closed form, since velocity is already analytic:

```js
// velocity at t, from the same closed form as position
const speedNow = Math.hypot(vx, vy) * Math.exp(-drag * t);
const stretch = 1 + Math.min(maxStretch, speedNow / stretchReference);
```

Apply as a `scaleX` in the particle's own rotated frame, with the rotation set to
the direction of travel rather than the tumble angle while `stretch` is
significant. Blending between "oriented along velocity when fast" and "tumbling
when slow" is what sells it.

Constraints:

- `maxStretch` around 1.8. Beyond that the chips look like rain.
- The stretched long axis must stay under about 40px at the reference size, or
  the burst reads as streaks of paint.
- The 3px minimum thickness from `minHeight` still applies to the short axis
  after stretching.

Cheaper alternative if the velocity-orientation blend gets fiddly: scale only the
width along the existing rotation, accepting that the streak direction is
approximate. Try the correct version first; it is a few lines.

## Problem 2: a maximum hold outlives the burst

At `holdMs` 2500 the field drains from frame 33 of 37, leaving the last few
frames nearly empty. Confirmed by probe: `...,12,8,4,4,2,1,0,0`.

### Fix: a second volley sized to the window

Give a fraction of the particles a launch delay in the second half of the
celebration window, but only when the window is long enough to need it. The
particle builder does not currently know the window length, so pass it in:

```js
createConfettiParticles({count, size, colors, random, windowSeconds})
```

Then a particle in the second wave gets
`delay = windowSeconds * lerp(rand('wave'), 0.42, 0.55)`.

At the default 1.47s window the second wave lands around 0.62-0.81s, which
overlaps the first wave's peak and simply thickens it. At a 2.47s window it lands
around 1.04-1.36s and refills the tail. At the 0.5s minimum hold the window is
short enough that the second wave collapses into the first, which is correct
behaviour rather than a special case.

Split roughly 30% of particles into the second wave. Do **not** raise `count` to
compensate: measured byte cost of particle count is negligible (70 to 40
particles changed the GIF by 1.1%), so if density needs a lift, raising `count`
to 90 is nearly free. Confirm that against a render rather than assuming.

## Interaction with the opacity tail

`Confetti.jsx` fades over the last `min(4, round(lifetime * 0.25))` frames and
quantizes opacity to quarters. A second wave arriving late must not still be
mid-flight when the fade starts, or particles vanish in mid-air. Check that the
second wave's latest launch plus its rise time lands before the fade begins; if
not, pull the 0.55 upper bound down.

## Tests

- Stretch factor is 1.0 at rest and bounded by `maxStretch` at launch.
- Short axis stays at or above 3px after stretching, at sizes 384, 480, 512.
- Every particle still launches within the celebration window for `holdMs` 500,
  1200, and 2500.
- Second-wave particles have strictly larger delays than first-wave ones.
- Determinism unchanged: same seed produces identical particles.

## Validation

Re-run the density probe for `holdMs` 500, 1200, 2500 at 384/12fps and 512/15fps.
Target: no frame in the celebration window below 40 live particles, and the last
frame of a 2500ms hold above 25. Then render and view frames at the reveal, mid
burst, and the end.
