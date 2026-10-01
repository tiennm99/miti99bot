# Research: a realistic gravity fall for the gacha meteor

Conducted 2026-10-01. Target: `tiennm99/wheelofnames` `src/remotion/gacha-timeline.js`
(`getMeteorState`) and `GachaComposition.jsx`.

## Outcome

Users said the fall did not look good ("Cái bay xuống chưa đẹp"). The cause:
the meteor moved along a fixed quadratic Bézier with a quadratic ease-in on
the curve parameter. That is not motion under gravity: it starts almost still,
whips to the end, and its trail was spaced along the curve instead of in time,
so it bunched up early and showed no sense of speed.

The fix is plain Newtonian projectile motion, which the sources agree is the
right model: constant horizontal speed, vertical speed growing linearly under
gravity, which traces a true parabola
([Wikipedia: projectile motion](https://en.wikipedia.org/wiki/Projectile_motion),
[GameDev.net](https://www.gamedev.net/forums/topic/629786-projectile-motion-parabola/4970539/)).

## Formula

With launch point `P0`, impact point `P1`, fall duration `T`, and a chosen
launch slope `k` (vertical speed as a fraction of horizontal speed):

```text
vx      = (x1 - x0) / T
vy0     = |vx| * k                         k = -0.3: launches slightly upward
g       = 2 * (y1 - y0 - vy0 * T) / T²     solved so it lands exactly at P1
x(t)    = x0 + vx * t
y(t)    = y0 + vy0 * t + ½ g t²
vy(t)   = vy0 + g t
```

Solving `g` from the endpoints keeps the impact point and timing fixed, so
the flash and reveal choreography is unchanged. With `k = -0.3` the meteor
climbs at about 17°, bends over, and dives at about 54° while speeding up.

## Supporting techniques

- **Motion streak sampled in time:** the trail draws the meteor's own past
  positions every 16 ms, so its length grows with speed, which is how motion
  blur reads ([Wikipedia: motion blur](https://en.wikipedia.org/wiki/Motion_blur_(media))).
- **Velocity-aligned stretch:** the head is rotated to `atan2(vy, vx)` and
  stretched along it while thinning across it to keep its area, the standard
  squash-and-stretch for fast objects
  ([Programmatic squash and stretch](http://www.alexgalbraith.nz/2019/04/05/programmatic-squash-and-stretch/)).
- **Spark physics:** each spark inherits part of the meteor's velocity plus a
  random kick, then falls under its own gravity while fading.

## Verification

A time-lapse of rendered frames shows a clear parabolic arc with gaps that
widen as the meteor accelerates. Tests check constant horizontal speed,
constant vertical acceleration, the exact landing point, and the climb-to-dive
angles. Render time is unchanged (about 9 s for 5★, 7 s for 3★/4★).

## Unresolved questions

- None.
