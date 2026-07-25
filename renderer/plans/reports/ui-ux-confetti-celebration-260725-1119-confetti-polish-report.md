# Winner Celebration Visual Review — Confetti Burst

**Date:** 2026-07-25 · **Scope:** rendered motion design of the winner celebration (`Confetti.jsx`, `confetti-layout.js`, winner emphasis in `WheelComposition.jsx`, theme palettes)
**Mode:** advisory only, no source files modified.

---

## 1. Verdict

The celebration currently reads as a **small grey-to-colored dust puff at the hub**, not a confetti burst. Three compounding reasons: the burst fires 8-10 frames *after* the wheel has already visually stopped (so it has no beat to land on), it empties the frame in ~0.8s of a 1.13s window (70 → 3 on-screen particles), and it spends its densest frames clustered on top of the hub in colors identical to the wheel slices, so it looks like speckle noise rather than paper. The winner emphasis that is supposed to carry the moment — an animated `drop-shadow` glow — is at the perceptual threshold (mean ΔRGB 15 vs no-glow, ΔRGB 10 for the whole pulse) while paying for a dithered gradient ring on every hold frame. In `mono` the confetti is effectively invisible: 5 of 6 palette colors sit at 1.00-1.79:1 against the background. Net: the two celebration signals do not compete, because neither is strong enough to compete.

---

## 2. Measured baseline (empirical, this machine)

Verified by rendering, not by reading:

| Fact | Value |
| --- | --- |
| Default composition (`durationMs` 6500 + `holdMs` 1200 @15fps) | 116 frames; `spinFrames` 98; confetti `lifetimeFrames` **17** (1.133 s) |
| Render time, 512@15fps, 6 options | **4.1 s** (`smoke` 384@12fps: 2.5 s) — well inside `RENDER_TIMEOUT_MS` 15000 |
| GIF size, default | **1.80 MB**; spin phase 1.64 MB (88%), **hold phase 226 KB (12%)** |
| GIF loop count | `Iterations: 0` → **loops forever** |
| Particle bodies @512 | width 6.0-12.0 px, height 2.48-11.2 px |
| Particle speed | 326-802 px/s → **21.7-53.5 px per frame** at 15fps |
| On-screen particles, f+0 … f+17 | 70,70,70,70,70,70,68,63,57,51,45,36,26,23,14,11,6,**3** |
| Opacity, f+0 | **0.00** (first burst frame renders 70 invisible divs) |
| Winner-glow salience (60 probe points on winner arc, r=213..230) | no-glow→glow: mean ΔRGB 15.3 / max 46. glow-base→glow-peak: mean ΔRGB **10.2** / max 31 |
| Wheel rim motion before the burst | 3.1 px/frame at f=87, 1.6 px/frame at f=90, 0.2 px/frame at f=95 |

Commands used (outputs deleted afterwards; `fixtures/*.gif` is git-ignored):
`pnpm render:smoke`, then `node scripts/render-local.js -o fixtures/rev-<theme>.gif --option ... --winner 1 --theme <theme>`, frames extracted with `magick <gif> -coalesce`.

---

## 3. Findings

### F1 — HIGH · Ballistic-only physics empties the frame before the window ends
`src/remotion/confetti-layout.js:56` (`speed 320..820`), `:66` (`gravity: 1500`), `:81-85` (pure ballistic `x = x0 + vx·t`, no drag).

Measured depletion: 70 on-screen through f+5, then 45 at f+10, 26 at f+12, **3 at f+17**. The last third of the hold is a static wheel. Real confetti decelerates hard in air, then flutters; this launches like buckshot and leaves.

**Fix — closed-form exponential drag (still a pure function of `t`, no state):**

```js
// confetti-layout.js — replace getConfettiParticleState
const decay = 1 - Math.exp(-particle.drag * t);
const terminalY = particle.gravity / particle.drag;
x = particle.originX + (particle.vx / particle.drag) * decay;
y = particle.originY + ((particle.vy - terminalY) / particle.drag) * decay + terminalY * t;
```

Constants: `gravity: 1300 * scale`, `speed = lerp(rand('speed'), 700, 1500) * scale`, `drag = lerp(1 - sizeNorm, 2.4, 3.6)` (small chips decelerate faster — free depth cue, one line).

Simulated result with the same 17-frame window: on-screen `0,46,62,74,83,89,88,84,83,81,82,82,80,77,77,75,75,75`. Frame-to-frame step drops from 53 px at f+1 (the pop) to 14-25 px from f+7 on (readable flutter).

### F2 — HIGH · The first burst frame is fully transparent
`src/remotion/Confetti.jsx:28` — `interpolate(progress, [0, 0.06, 0.7, 1], [0, 1, 1, 0])` with `progress = localFrame / lifetimeFrames`, so at `localFrame = 0` opacity is exactly 0. Confirmed in the render: frame 98 shows zero confetti.

The only frame where all particles are still compact at the origin — the actual "pop" — is thrown away, and by f+1 they have already travelled 35 px average. Nothing in the GIF ever shows a dense cluster.

**Fix:** no fade-in. Opacity starts at 1 (see F9 for the tail).

### F3 — HIGH · `mono` confetti is invisible
`src/remotion/themes.js:36`, consumed at `src/remotion/WheelComposition.jsx:186` (`colors={theme.slices}`). Contrast vs background `#f4f4f5`:

`#f4f4f5` **1.00** · `#e4e4e7` **1.15** · `#d4d4d8` **1.34** · `#c4c4ca` **1.58** · `#b8b8bf` **1.79** · `#a1a1aa` 2.33

5 of 6 colors are below 1.8:1. The render confirms it: the burst reads as dust or sensor noise on a light grey field.

**Fix — add a `confetti` key per theme** (internal, no request-schema change), fall back to `slices` if absent:

```js
classic:  ['#f97316', '#14b8a6', '#f59e0b', '#3b82f6', '#ec4899', '#8b5cf6', '#10b981', '#f43f5e'] // min 2.05 vs #f8fafc
festival: ['#f43f5e', '#ea580c', '#65a30d', '#0891b2', '#7c3aed', '#db2777', '#16a34a', '#d97706'] // min 2.91 vs #fff7ed
mono:     ['#18181b', '#3f3f46', '#52525b', '#71717a', '#a1a1aa']                                   // 16.1 / 9.5 / 7.0 / 4.4 / 2.3
```

`classic` also drops `#facc15` (1.46 vs `#f8fafc`) and `#34d399` (1.84) — both effectively invisible as 8 px chips today even though they work as large slices. Cost: zero extra palette pressure (flat colors), no new DOM.

### F4 — HIGH · The burst fires ~0.6 s after the wheel has visually stopped
`src/remotion/wheel-layout.js:56` — `1 - (1 - p) ** 3`. Any polynomial ease-out has zero terminal velocity, so the last frames are motionless:

| frame | rim motion | remaining rotation |
| --- | --- | --- |
| 87 | 3.1 px/frame | 3.41° |
| 90 | 1.6 px/frame | 1.31° |
| 93 | 0.6 px/frame | 0.32° |
| 95 | 0.2 px/frame | 0.07° |

`Confetti startFrame = holdStartFrame = spinFrames = 98` (`WheelComposition.jsx:31,186`). The viewer perceives the stop around frame 88-90, then watches ~9 dead frames, then confetti appears out of nowhere. The celebration has no cause.

**Fix (cheap, no schema change):** one shared reveal frame, 5 frames earlier at 15fps:

```js
const revealFrame = Math.max(0, spinFrames - Math.round(fps * 0.33));
```

Cost: the wheel is 0.32° (1.2 px at the rim) short of final when the burst starts — imperceptible, and the winner slice is already fully under the pointer. Payoff: burst lands on the perceived stop *and* the window grows from 17 to 22 frames (+29%) for free. Drive both `Confetti startFrame` and the winner pulse from `revealFrame`.

### F5 — MEDIUM · Particles strobe at 15fps
Measured step 26-35 px average over f+0..f+5 against 6-12 px bodies — 3-5× their own length per frame, with no motion blur available in a GIF. They read as discrete jumping dots, not motion.

Fixed by F1 (step falls to 14-25 px from f+7). Optional extra, one line, deterministic: elongate the body along the launch direction in proportion to launch speed, `width *= 1 + speedNorm * 0.5`, which fakes motion blur with a static shape.

### F6 — MEDIUM · Confetti wears the wheel's own colors, over the wheel
`WheelComposition.jsx:186` passes `theme.slices` as the confetti palette, and the origin is the wheel center, so the densest frames sit entirely on the disc. With 6-8 slices, roughly 1 particle in 6-8 is on a slice of its own color at any moment — orange chips vanish on the orange slice. This is the main reason frames 99-103 read as "dirt on the wheel" rather than paper in the air.

**Fix — hard 1 px light ring, no blur:**

```js
boxShadow: '0 0 0 1px #ffffff',
```

`#ffffff` is already in the palette (slice strokes, `WheelComposition.jsx:115`), and a zero-blur spread produces no gradient, so this costs **zero new palette entries and zero dithering** while guaranteeing separation over any slice. Side benefit: it grows the effective particle by 2 px, which is exactly what the 384 px / 15 fps case needs.

### F7 — MEDIUM · The emission box is entirely inside the hub
`confetti-layout.js:63-64` — origin jitter is `±0.06 · size` = ±31 px at 512. The hub ring is `centerRingRadius = 0.09 · size` = 46 px (`WheelComposition.jsx:41,161`). Every particle is born inside the hub disc, so the launch reads as "the axle exploded".

**Fix, two options:**

- *Quick (2 lines):* origin on the winner slice, pointer side. `originX = center + radius * 0.55` (≈371 px at 512), `originY = center + lerp(rand, -0.03, 0.03) * size`, fan `-175..-45°`. Ties the burst to the thing being announced.
- *Better (small rework):* two cannons framing the wheel. Even index → `origin (0.06·size, 0.90·size)`, fan `-75..-30°`; odd index → `(0.94·size, 0.90·size)`, fan `-150..-105°`. With F1 constants (`v0` 700-1500, `drag` 3.0, `g` 1300) particles crest around `y ≈ 0.40 · size`, sweeping diagonally across the disc. The launch frames happen over the plain corner background, where the pop is legible, and the classic party-popper read is unmistakable.

### F8 — MEDIUM · Winner glow is near-threshold and fights the wheel's own shadow
`WheelComposition.jsx:118` — `drop-shadow(0 0 ${10 + winnerPulse * 10}px rgba(250,204,21,0.45))`, sitting in the same 15-25 px band outside the disc as `theme.wheelShadow` `0 18px 52px rgba(15,23,42,0.22)` (`:75`). The yellow at 45% alpha over a cool grey shadow over a near-white background produces a muddy cream tint: measured mean ΔRGB **15.3** vs no glow, and the entire 10→20 px pulse is only mean ΔRGB **10.2**. A contrast-stretched crop shows the halo is visibly **dithered speckle** — the worst possible payload for a 256-color GIF, animated on every hold frame.

**Fix — make the emphasis geometric instead of gradient.** Scale-pop the winner `Pie` about the wheel center:

```js
const pop = frame >= revealFrame
  ? Math.sin(Math.min(1, (frame - revealFrame) / Math.round(fps * 0.4)) * Math.PI) * 0.045
  : 0;
// on the winner Pie only
transform: `scale(${1 + pop})`, transformOrigin: '50% 50%',
```

A 4.5% scale on a 210 px radius moves the arc edge **9.5 px** — an order of magnitude more legible than ΔRGB 10, and it moves existing flat colors instead of creating new blended ones. Then either delete the animated glow or make it a fixed, tighter, more opaque halo (`drop-shadow(0 0 8px rgba(250,204,21,0.75))`, constant for the whole hold) so it compresses as one static delta rather than 18 changing gradients. For `mono`, `winnerGlow: rgba(161,161,170,0.45)` on `#f4f4f5` is hopeless — use `rgba(24,24,27,0.35)` there.

### F9 — MEDIUM · The fade-out window is wasted and progress-relative
`Confetti.jsx:28` — fade starts at `progress 0.7`. At `holdMs` 1200 that is f+11.9, when only 36 of 70 particles remain, and it reaches 0 with 3 left. At `holdMs` 2500 it starts at f+26, when **zero** remain. So the fade never does visual work; it only spends 5 frames of alpha-blended particles (new blend colors → palette pressure) on nothing. It does not "fight the winner reveal" today — it is simply inert.

**Fix — frame-relative, quantized tail:**

```js
const fadeFrames = Math.min(4, Math.max(1, Math.round(lifetimeFrames * 0.25)));
const raw = interpolate(localFrame, [lifetimeFrames - fadeFrames, lifetimeFrames], [1, 0],
  {extrapolateLeft: 'clamp', extrapolateRight: 'clamp'});
const opacity = Math.ceil(raw * 4) / 4; // at most 5 alpha levels → bounded palette cost
```

Quantizing to quarters caps the number of new blended colors instead of generating a fresh set per frame. It also guarantees a confetti-free final frame, which matters because the GIF loops forever (F15).

### F10 — MEDIUM · Long holds buy dead air, not a longer celebration
Rendered `--hold 2500` (37 hold frames): confetti is gone by f+19 and the last **18 frames (1.2 s, half the hold)** are a completely static wheel. Burst length is governed by physics, not by the composition, so raising `holdMs` today makes the GIF worse. Do not raise the `holdMs` default as a fix. With F1 constants the same window drains at f+33, leaving 4 dead frames instead of 18.

### F11 — LOW · Sub-3px dimension at `size: 384`
`confetti-layout.js:62` — `height = width * lerp(rand('ratio'), 0.4, 1)`. At 384 (`scale` 0.75) minimum height is **1.87 px**, and 5 of 70 particles are under 2.5 px. After PNG→GIF quantization these are a single dim row of pixels.

**Fix:** `height: Math.max(3, width * ratio)`.

### F12 — LOW · One shape, linear spin, no air
All 70 particles are the same rounded rect, rotating at a constant `±540 deg/s` (`:65`) — 36°/frame at 15fps, which samples as flicker rather than tumble — with no lateral motion at all.

**Fix, three numbers, no new abstraction:**

- Aspect ratio `lerp(rand('ratio'), 0.25, 2.6)` with `height = clamp(width * ratio, 3, 26)`. One distribution yields chips, near-squares, and long ribbons — no `type` field, no branching.
- Size skew `width = lerp(rand('width') ** 1.7, 6, 18) * scale` — mostly 6-10 px with a few 14-18 px anchors that survive 15 fps and quantization.
- `rotationSpeed = lerp(rand('spin'), -300, 300)` — max 20°/frame, reads as a coherent tumble.
- Sway, added to `x`: `Math.sin(phase + t * hz * 2π) * amp * Math.min(1, t * 2)`, with `hz = lerp(rand('hz'), 0.5, 0.9)` and `amp = lerp(rand('sway'), 8, 20) * scale`. At 0.5-0.9 Hz that is a 17-30 frame cycle — well above the Nyquist limit at 15 fps, unlike a per-frame flip. **Do not** add 3D foreshortening (`scaleY(cos …)`): at 15 fps the flip aliases, and it drives the short axis below the 3 px floor from F11.

### F13 — LOW · `borderRadius: 1` on 6-12 px chips
`Confetti.jsx:50`. Invisible at this size but produces ~12 antialiased corner pixels per particle (≈850/frame at 70 particles) that all need palette slots. Set `0`.

### F14 — LOW · Non-winner labels are upside-down at rest
Not confetti, but it is what the reveal frame looks like. Final rotation is `7·360 − sliceCenter` (`wheel-layout.js:27`), e.g. 225° for winner 1 of 6, so labels in the upper-left read inverted (visible in every rendered hold frame). The **winner** label is always upright by construction, so the announcement itself is legible — the surrounding frame just looks sloppy. Out of scope here; worth a separate look.

### F15 — LOW · Infinite loop with a non-empty last frame
`identify` reports `Iterations: 0` and `render-gif.js:51-61` sets no `numberOfGifLoops`. At `holdMs` 1200 the final frame still carries 3 particles, so the loop point snaps confetti away at the same instant the wheel jumps back to `-24°`. F9's guaranteed-empty final frame removes half of that discontinuity for free.

---

## 4. Prioritized recommendations

### Quick wins (each well under 30 min, no contract change)

| # | Change | Files | Visual payoff | Risk |
| --- | --- | --- | --- | --- |
| 1 | Opacity starts at 1; frame-relative quantized tail (F2, F9) | `Confetti.jsx:28` | Recovers the pop frame; removes an inert 5-frame ramp | none to determinism; **reduces** palette churn |
| 2 | `theme.confetti` palettes (F3) | `themes.js`, `WheelComposition.jsx:186` | `mono` goes from invisible to legible; `classic` loses two dead colors | none; flat colors, no size change |
| 3 | Hard 1 px `#ffffff` ring (F6) | `Confetti.jsx:48-58` | Particles stop reading as dirt on the wheel face | none; `#ffffff` already in palette, no blur |
| 4 | `revealFrame = spinFrames − round(fps·0.33)` for confetti **and** winner pulse (F4) | `WheelComposition.jsx:31-37,186` | The burst finally lands on a beat; +29% window | 0.32° rotation error, imperceptible |
| 5 | `Math.max(3, …)` height floor; `borderRadius: 0` (F11, F13) | `confetti-layout.js:62`, `Confetti.jsx:50` | Nothing vanishes at 384; crisper edges | none; slightly smaller GIF |
| 6 | Shape/size/spin/sway numbers (F12) | `confetti-layout.js:55-70` | Ribbons + chips + tumble instead of uniform specks | none (all seeded); +1 `Math.sin` per particle per frame |

### Larger reworks

| # | Change | Payoff | Risk |
| --- | --- | --- | --- |
| A | **Exponential drag physics** (F1) — the single highest-payoff change | 75-90 particles on screen at the last frame instead of 3; punchy launch then real flutter; long holds stop being dead air | Determinism: safe, closed form in `t`. Render time: unchanged (same node count, 2 extra `Math.exp`). GIF: more moving pixels for ~10 extra frames; hold phase is 12% of bytes today, so expect roughly +5-8% total, i.e. ~1.9 MB. Needs `pnpm render:fixtures` re-measurement and a `docs/render-benchmarks.md` update. |
| B | **Two-cannon origin** (F7, option 2) | The clearest "celebration" read; launch frames happen over plain background where they are legible; stops the hub-explosion look | Determinism safe. Composition risk: verify particles do not pile on the winner label — with 90 particles at ~90 px² each, total coverage is ~3% of the frame, so occlusion is not a practical concern |
| C | **Geometric winner emphasis** (F8) — scale-pop, glow made static or removed | Winner emphasis becomes actually visible (9.5 px edge move vs ΔRGB 10); removes 18 frames of animated dithered gradient | Changes the winner's silhouette by 4.5% — check the 2 px white stroke and slice seams do not tear at `size: 384`. Likely a **net GIF-size win** |
| D | **Particle count 70 → 90** | Density during the good frames | +20 divs/frame; render cost negligible at 4.1 s total. Do this only after A, since A alone triples effective on-screen density |

### Explicitly do NOT do

- Per-particle opacity fades, blur, soft shadows, or gradient particle fills — each multiplies palette entries and produces the same dither speckle already visible in the winner glow.
- A back/front depth split (confetti behind the wheel). The disc occupies ~82% of the canvas width, so a back layer would be hidden for most of its life. Rejected on payoff, not on cost.
- Raising the `holdMs` default. F10 shows the current burst cannot fill even 1.2 s; fix physics first. Changing schema defaults would also be a user-visible behavior change.
- Any particle-system abstraction. Every finding above is a constant, a distribution range, or one closed-form expression in the two files that already exist.

### Suggested implementation order

1, 2, 3, 5 (independent, all low-risk) → A → 4 → C → B → D, re-rendering all three themes plus `--hold 500` and `--hold 2500` and `--size 384` after A, B and C.

---

## 5. Not verified empirically

- **GIF size and render time after the proposed changes.** All size/time figures are baseline measurements of the current code. The +5-8% estimate for drag physics is extrapolated from the measured 226 KB / 12% hold-phase share, not measured.
- **The two-cannon and scale-pop designs were not rendered** (advisory-only, no source edits). Trajectory claims for the cannons come from the same closed-form simulation used to tune the drag constants, not from a Chromium render.
- **Perceptual thresholds** — statements like "the viewer perceives the stop around frame 88-90" derive from rim displacement in px/frame (3.1 px at f=87, 1.6 px at f=90), not from user testing.
- **`fps: 12` and `fps: 20` with the proposed constants** were simulated (both hold density well) but not rendered.
- **Frame timing granularity**: `identify` shows delays alternating 6/7 centiseconds for 15 fps. Players that clamp low delays could shift the perceived envelope; not tested in any real client.

---

## 6. Unresolved questions

1. Is the infinite GIF loop intentional? The wheel snapping from the winner back to `-24°` forever undercuts the reveal. `numberOfGifLoops: 1` in `render-gif.js` would end on the winner — but that is a user-visible behavior change and some chat clients handle non-looping GIFs poorly.
2. Should the confetti palette stay tied to the theme, or become a fixed festive palette in all three themes? A fixed palette is the strongest visual answer for `mono`, but it breaks `mono`'s design intent (a deliberately restrained theme). The proposal above keeps `mono` monochrome by inverting to darks; confirm that reads as intended rather than as "black litter".
3. Is a spin easing with non-zero terminal velocity (or a small settle recoil) acceptable? That is the root cause of F4; the `revealFrame` shift is a workaround that leaves the wheel visually frozen for ~5 frames before the pop.
4. Is `mono`'s `winnerGlow` supposed to be visible at all? At `rgba(161,161,170,0.45)` on `#f4f4f5` it is unmeasurable, so today `mono` has no winner emphasis whatsoever.
