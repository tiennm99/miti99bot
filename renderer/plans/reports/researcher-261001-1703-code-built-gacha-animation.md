---
title: Open-source gacha animations built in code (no video playback)
date: 2026-10-01 17:03 (Asia/Saigon)
---

# Open-source gacha animations built in code (no video playback)

## Answer

No mature open-source web project renders a Genshin-quality gacha pull in
code under a licence we can use. The web projects that draw effects in code
are general effect or particle libraries, or card-pack reveals, not wish
sequences. Non-web projects, such as a Unity Arknights roll recreation, are
excluded. The most promising building block is **rollshade** (MIT): a three.js
effect library with meteors, impacts, bloom, and camera shake. It is two days
old, though, and its update loop is stateful, which conflicts with Remotion's
frame-by-frame rendering.

## Web candidates (code-built, no video)

Data comes from the GitHub API and the repositories' READMEs on 2026-10-01.
"Video files" counts `.mp4`/`.webm`/`.mov` files in each repository tree.

| # | Project | Stars | Stack | Licence | Video files | What it is | Fit |
|---|---|---|---|---|---|---|---|
| 1 | [tsuyatt/rollshade](https://github.com/tsuyatt/rollshade) ([gallery](https://rollshade.tsuyatt.com/gallery/)) | 0 | three.js r186, WebGPU/TSL with WebGL2 fallback | MIT (npm `rollshade@0.1.0`) | 0 | 25 seeded spell effects (meteor, beam, impact, portal…) × 10 elements, with bloom and camera shake | Best effect source. Risks below. |
| 3 | [briamkin/card-pack-animations](https://github.com/briamkin/card-pack-animations) | 0 | TypeScript, React wrappers | MIT | 0 | Card-pack tear and reveal | Different genre, small |
| 4 | [paubineau/pack-cards](https://github.com/paubineau/pack-cards) | 0 | Vanilla JS and CSS | MIT | 1 (demo) | Pack wrapper, reveal animation, card materials | Different genre |
| 5 | [Tmauc/prismo](https://github.com/Tmauc/prismo) | 0 | React, CSS-only | None | 0 | Holographic foil card effects for 10+ rarities | No licence |
| 6 | [Takuro-U/gacha-animation](https://github.com/Takuro-U/gacha-animation) | 0 | React, Tailwind | None | 0 | Small gacha UI demo | No licence |
| 7 | [drawcall/Proton](https://github.com/drawcall/Proton) | 2477 | JS particle engine (canvas, WebGL, pixi) | MIT | — | General particle library | Building block only |
| 8 | [pixijs-userland/particle-emitter](https://github.com/pixijs-userland/particle-emitter) | 852 | PixiJS | MIT | — | Particle emitter with a visual editor | Building block only |

## Using rollshade inside Remotion

```js
// rollshade's own loop: time is wall-clock deltas, state accumulates.
fx.play(effect('meteor', 'fire', {seed: 7}), {from: start, to: target});
fx.update(timer.getDelta());
```

The risks, roughly in order of severity:

- **Determinism.** Remotion renders frames out of order across several
  browser tabs, but `fx.update(dt)` accumulates state. Each frame would have
  to replay from frame 0 with fixed steps (`1/fps`), so the cost grows with
  clip length, or renders would have to run with concurrency 1.
- **Rendering speed.** The server has no GPU. WebGPU will fall back to WebGL2
  on SwiftShader, which is likely several times slower than our CSS frames.
  The current beta already takes about 15s at 854px and 30fps against the
  15s timeout.
- **Maturity.** Version 0.1.0, first published this week, pinned to one three
  release (r186), and the API can still change.
- **New dependencies.** `three`, `rollshade`, and `@remotion/three` with
  `@react-three/fiber`. That conflicts with our "CSS over custom rendering"
  rule in AGENTS.md, so you'd need to approve it.

## Options

1. **Prototype one shot with rollshade.** Render the meteor and impact shot in
   a separate composition, measure the render time on this server, and check
   frame determinism before committing to a rewrite. About one session of
   work, with no changes to `/api/gachabeta` until it proves out.
2. **Keep our own code and borrow techniques.** Port ideas such as rollshade's
   seeded variations, bloom-like layering, and impact timing into the existing
   CSS and canvas renderer. No new dependencies, and no render-time risk.
3. **Painted textures plus our current motion** (from the previous report).
   Not code-only, but the biggest visual gain for the cost.

## Unresolved questions

- Would you accept adding three.js to the renderer if the prototype renders
  inside the timeout?
- I couldn't browse rollshade's live gallery here (no browser on this server),
  so I haven't seen its meteor effect myself.
