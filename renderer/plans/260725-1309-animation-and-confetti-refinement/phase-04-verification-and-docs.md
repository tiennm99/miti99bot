# Phase 04 — Verification and documentation

Runs after phases 1-3. Two jobs: re-measure honestly, and fix a documentation
error already sitting on `main`.

## Documentation correction (do this even if phases 1-3 slip)

`docs/render-benchmarks.md` currently misattributes where the GIF growth came
from. It says the extra bytes are split between denser confetti and grayscale
antialiasing. Measurement says otherwise:

| Build | Bytes | vs baseline |
|---|---:|---:|
| baseline before the celebration rework | 2,066,068 | — |
| every change **except** the AA fix | 2,133,986 | +3.3% |
| every change, AA fix included | 2,597,190 | +25.7% |

The confetti rework, winner pill, spin retune, palette reorder and SVG pointer
together account for +3.3%. Grayscale AA alone accounts for +22.4%, because it
affects all 98 spin frames rather than only the celebration.

The "levers if a deployment needs bytes back" list is wrong for the same reason.
Lowering the `Confetti` particle count from 70 to 40 changes the GIF by **1.1%**
(2,597,190 to 2,568,902), so it is not a lever. Measured alternatives:

| Change | Bytes | vs default |
|---|---:|---:|
| `count` 70 to 40 | 2,568,902 | -1.1% |
| `holdMs` 1200 to 500 | 2,492,491 | -4.0% |
| `holdMs` 1200 to 2500 | 2,859,979 | +10.1% |
| drop the AA fix | 2,133,986 | -17.8% |

The only large lever is the AA fix itself, and keeping it is a decided trade.
Rewrite the section to say that plainly instead of offering a knob that does
nothing.

## Re-measurement after phases 1-3

Adaptive turn count changes total rotation at most option counts, which changes
per-frame pixel deltas and therefore file size — likely downward on dense wheels,
since they now turn less. Confetti streaks and a second wave push the other way.
Neither is predictable; measure rather than estimate.

Capture before and after for each, on one machine, back to back, warm bundle:

- smoke, 384 @12fps, 4 options
- classic, 512 @15fps, 8 options
- vietnamese fixture
- sixteen-options fixture
- mono, 512 @15fps, 6 options
- schema maximum: 32 options, 512, 20fps, 10000+2500

Note on method: render times in the last session varied by more than 2x on
repeat runs of an identical input (one 512 render took 23.5s against a 4.3s
median) because of background load. Sizes are deterministic and trustworthy;
times need a median of at least three runs before any conclusion.

Then update both tables in `docs/render-benchmarks.md`.

## Visual verification checklist

Render and inspect, at 512 and downscaled to 200px:

- Poster frame, classic 8 options: winner pill legible, every label upright,
  winner slice outlined, pointer outlined with margin
- Poster frame, mono: wheel has a silhouette, confetti visible against the
  background
- Reveal frame: burst is a compact pop, not an empty frame
- Mid-burst frame: field crosses the wheel, streaks read as motion
- Final frame at `holdMs` 2500: field still populated
- 24 and 32 options: judge whether the reduced turn count reads as slow or as
  broken, which is the open risk from phase 1
- Long Vietnamese winner name: pill stays two lines and diacritics are complete

## Gates

`pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm api:smoke`, `pnpm render:fixtures`.

## Unresolved questions

1. Does 1 turn at 24-32 options read as intentional? Decided by looking at a
   render, not by argument. Fallback is a floor of 2 turns with the residual
   aliasing documented.
2. Is the `night` theme wanted? Still open from the previous pass, still a
   contract addition, still out of scope here.
3. Does the 14.4MB schema-maximum GIF matter in practice? Depends on whether any
   caller sends those parameters, which cannot be answered from this repo.
