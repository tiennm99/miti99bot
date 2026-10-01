# Research: matching the gacha meteor to Genshin Impact's motion

Conducted 2026-10-01. Target: `tiennm99/wheelofnames` `getMeteorState` in
`src/remotion/gacha-timeline.js` and the trail in `GachaComposition.jsx`.

## Outcome

Genshin's meteor does **not** follow a gravity arc. Measured from reference
footage, it sweeps in along a **straight line** from the upper left at about
23° below horizontal and **decelerates exponentially** to a near hover just
above the horizon, where the colour reveal and the 5★ rainbow ring play. The
gravity parabola shipped earlier was replaced with this measured curve.

## Method

Reference: the single-pull wish videos (`3star-single.mp4`,
`4star-single.mp4`, `5star-single.mp4`; 1920x1080, 60 fps, 6 s) from the
public fan simulator
[AguzzTN54/Genshin-Impact-Wish-Simulator](https://github.com/AguzzTN54/Genshin-Impact-Wish-Simulator).
They were used only to measure motion; nothing from them is shipped. Frames
were extracted with ffmpeg at 10 fps (timeline) and 30 fps (glide), and the
head was tracked as the brightest cluster above the cloud line.

## Findings

Timeline of the in-game single pull: cloud portal (0–1.2 s), dive with a beam
sweep (1.2–1.6 s), **glide and hover** (1.7–3.4 s), flash (3.5 s), a streak and
a near-vertical drop, then the result card.

Head position during the glide (5★, fraction of frame):

| t (s) | 1.80 | 2.00 | 2.20 | 2.40 | 2.80 | 3.20 |
|---|---|---|---|---|---|---|
| x | 0.485 | 0.523 | 0.547 | 0.549 | 0.535 | 0.559 |
| y | 0.509 | 0.531 | 0.540 | 0.536 | 0.543 | 0.560 |

- **Path:** straight, about 23° below horizontal, from off-screen upper left to
  about (0.56, 0.56).
- **Easing:** exponential ease-out. About 88% of the distance is covered 0.5 s
  after entry, then the head creeps and hovers; rate `k ≈ 4.2 /s`.
- **Trail:** a long straight beam from beyond the frame edge that narrows into
  the head and stays full length while the head hovers; several ribbons fan
  out along it, with sparkles floating above it.
- **Colour:** every tier starts blue. The 4★ beam turns purple around 2.5 s;
  the 5★ head turns gold around 2.8 s and the rainbow ring forms by 3.1 s.

## Formula now used

```text
s(t)   = D * (1 - e^(-k t)) / (1 - e^(-k T))      k = 4.2, T = 2.2 s
p(t)   = hover - u * (D - s(t))                   u = (cos 23°, sin 23°)
v(t)   = u * D * k * e^(-k t) / (1 - e^(-k T))
```

`D` reaches from 15% beyond the left edge to the hover point (0.55, 0.55);
normalising by `1 - e^(-kT)` lands the head exactly on the hover point.

## Not adopted

- **Blue-first colour reveal** (all tiers start blue, then change): this is
  the signature suspense beat in Genshin, but it changes how the tiers read,
  so it is left as an option for the owner.
- The cloud portal, beam dive, and final vertical drop do not fit the 7 s
  clip without restructuring the timeline.

## Unresolved questions

- Should the meteor start blue for every tier and reveal its colour during
  the hover, as Genshin does?
