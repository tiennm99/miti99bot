# Render Benchmarks

Fill this after running:

```sh
pnpm render:fixtures
```

| Fixture | Size | FPS | Duration | Bytes | Render Time | Notes |
|---|---:|---:|---:|---:|---:|---|
| smoke | 384 | 12 | 3.5s | 315661 | 13917ms | Docker `pnpm render:smoke` on 2026-07-06 |
| api-smoke | 384 | 12 | 3.5s | 298374 | 4100ms | Docker `pnpm api:smoke`, after startup bundle warm-up |
| smoke | 384 | 12 | 3.5s | 1495369 | 3608ms | Windows dev box, 2026-07-25 |
| vietnamese | 512 | 15 | 7.7s | 2894104 | 3416ms | Windows dev box, 2026-07-25, label readability |
| sixteen-options | 512 | 15 | 7.7s | 4930088 | 3516ms | Windows dev box, 2026-07-25, dense wheel |

The Docker rows and the dev-box rows are not comparable to each other: different
hardware, and the Docker rows predate the celebration rework below.

## Celebration rework, 2026-07-25

Measured on the same Windows dev box, immediately before and after the change,
so these pairs are comparable:

Bytes grew and render time did not. Measured back to back on one machine,
covering both the celebration rework and the motion refinement that followed:

| Case | Bytes before | Bytes after | Time before | Time after |
|---|---:|---:|---:|---:|
| smoke, 384 @12fps, 4 options | 1251208 | 1495369 | 3730ms | 3608ms |
| classic, 512 @15fps, 8 options | 2066068 | 2623039 | 4359ms | 4830ms |
| vietnamese, 512 @15fps, festival | 2286771 | 2894104 | 3065ms | 3416ms |
| sixteen-options, 512 @15fps | 3834855 | 4930088 | 3368ms | 3516ms |
| mono, 512 @15fps, 6 options | 1745305 | 2564362 | 4173ms | 3775ms |
| schema max: 32 options, 20fps, 12.5s | 10825678 | 11998202 | 9079ms | 7800ms |

The dense cases moved least, and the schema maximum actually shrank, because the
turn count now scales with the frame budget: a 32-option wheel spins 2 turns
instead of 5, so there is far less rotation to encode.

## Where the bytes went

Isolated by rendering with one change removed at a time, 512 @15fps, 8 options:

| Build | Bytes | vs baseline |
|---|---:|---:|
| baseline, before any of this work | 2066068 | — |
| everything except grayscale label AA | 2133986 | +3.3% |
| everything, AA included | 2597190 | +25.7% |

The confetti rework, winner pill, spin retune, palette reorder and SVG pointer
together account for **+3.3%**. Grayscale label antialiasing accounts for
**+22.4%** on its own, because it affects every spin frame rather than only the
celebration.

That AA cost buys a real fix. Chromium's default LCD sub-pixel text
antialiasing emits saturated color fringes on every glyph edge: in the
all-grayscale `mono` theme 2178 pixels of frame 0 carried a visible color cast,
and frame 0 used 255 of 256 palette entries, so everything else in the image had
to dither. Both drop to 0 fringe pixels and 204 colors. Grayscale AA spends more
distinct gray levels per glyph, which costs bytes but stops starving the palette.

Promote the one rotating container, not each label. Per-label promotion gives the
same visual result and doubles render time on a dense wheel — 18112ms against
8999ms for the 32-option case, which would breach the default
`RENDER_TIMEOUT_MS` of 15000. `-webkit-font-smoothing: antialiased` does not work
here; Chromium only honors it on macOS.

## Levers, measured

Particle count is not a lever. Measured at 512 @15fps, 8 options:

| Change | Bytes | vs default |
|---|---:|---:|
| `Confetti` `count` 70 to 40 | 2568902 | -1.1% |
| `holdMs` 1200 to 500 | 2492491 | -4.0% |
| `holdMs` 1200 to 2500 | 2859979 | +10.1% |
| drop grayscale AA | 2133986 | -17.8% |

Cutting 43% of the particles changes the file by about one percent, so reach for
`holdMs` or the AA trade instead. Keeping grayscale AA is a decided trade, not an
oversight.

## Measurement note

Render times on this machine varied by more than 2x across repeat runs of an
identical input — one 512 render took 23.5s against a 4.3s median — depending on
background load. Byte counts are deterministic and comparable; treat any single
timing figure as indicative and take a median of at least three runs before
concluding anything from it.
