# Wheel Animation Visual Design Review (advisory)

**Date:** 2026-07-25 · **Scope:** rendered GIF visual + motion design, excluding confetti particle design
**Verified empirically:** yes — 30 PNG stills + 3 GIFs rendered locally (Chromium present), contrast/ΔE/spin-curve computed numerically. No source file modified. No files written outside this report.

---

## 1. Verdict

The wheel is competently built and the winner never lands ambiguously, but it reads as a *pie chart that stopped*, not as a wheel that was spun. Motion is a single cubic ease-out over 7 turns: the first 22 frames move more than a full slice per frame (visual strobe/reverse illusion) and the last ~35 frames crawl under 4°/frame, so nearly 2.5 s of a 7.7 s GIF is visually static with no tick, no anticipation and no settle. Legibility is the bigger problem: roughly half of all labels render **upside down** at rest (`rotation: centerDegrees` with no 90–270° flip), so the poster frame of an 8-option wheel has 3–4 unreadable names; at a realistic ~200 px chat display almost nothing is readable and the winner is communicated only by a 17 px arrow at the frame edge. `mono` is the weakest theme — one slice is literally the background colour, dividers are white-on-near-white (1.10:1), and it reads as an unfinished grey blob. Finally there are three measurable production defects: LCD sub-pixel text antialiasing puts 268 unique colours into a single 90×30 px label crop (starving the 256-colour GIF palette), `pointerStroke`/`boxShadow` on the pointer are silently discarded by `clip-path`, and the max accepted request produces a **14.3 MB GIF in 10.3 s** on a fast dev box.

---

## 2. Contrast table (computed, WCAG 2.1 relative luminance)

`theme.text` on each slice. Pass thresholds: 3:1 large text (≥18.66px bold), 4.5:1 normal.

| Theme | Slice | text:slice | 3:1 | 4.5:1 | slice:bg | ΔE to next slice (normal) | ΔE deutan | ΔE protan |
|---|---|---:|:--:|:--:|---:|---:|---:|---:|
| classic (`#111827` / bg `#f8fafc`) | `#f97316` | 6.33 | pass | pass | 2.68 | 113 | 116 | 97 |
| | `#14b8a6` | 7.13 | pass | pass | 2.38 | 96 | 104 | 92 |
| | `#facc15` | 11.58 | pass | pass | 1.46 | 132 | 124 | 121 |
| | `#60a5fa` | 6.98 | pass | pass | 2.43 | 65 | 92 | 76 |
| | `#f472b6` | 6.70 | pass | pass | 2.53 | 46 | 65 | 59 |
| | `#a78bfa` | 6.52 | pass | pass | 2.60 | 113 | **16** | **31** |
| | `#34d399` | 9.23 | pass | pass | 1.84 | 109 | 79 | 53 |
| | `#fb7185` | 6.59 | pass | pass | 2.57 | 53 | **25** | **30** |
| festival (`#1f2937` / bg `#fff7ed`) | `#fb7185` | 5.45 | pass | pass | 2.53 | 68 | **18** | **25** |
| | `#f59e0b` | 6.83 | pass | pass | 2.02 | 70 | 50 | 35 |
| | `#84cc16` | 7.43 | pass | pass | 1.86 | 100 | 80 | 78 |
| | `#06b6d4` | 6.05 | pass | pass | 2.29 | 67 | **29** | **12** |
| | `#a78bfa` | 5.39 | pass | pass | 2.56 | 46 | 65 | 59 |
| | `#f472b6` | 5.54 | pass | pass | 2.49 | 130 | 51 | 26 |
| | `#22c55e` | 6.44 | pass | pass | 2.15 | 97 | 84 | 62 |
| | `#fb923c` | 6.49 | pass | pass | 2.13 | 50 | **17** | **22** |
| mono (`#18181b` / bg `#f4f4f5`) | `#e4e4e7` | 13.96 | pass | pass | 1.15 | 24 | 24 | 24 |
| | `#a1a1aa` | 6.91 | pass | pass | 2.33 | 19 | 19 | 19 |
| | `#d4d4d8` | 11.99 | pass | pass | 1.34 | **10** | **10** | **10** |
| | `#b8b8bf` | 8.98 | pass | pass | 1.79 | 22 | 21 | 21 |
| | `#f4f4f5` | 16.12 | pass | pass | **1.00** | 17 | 17 | 17 |
| | `#c4c4ca` | 10.20 | pass | pass | 1.58 | **12** | **11** | **12** |

Text-on-slice contrast **passes everywhere** — the 4-direction halo is not needed for contrast (see F-11). The failures are elsewhere:

Non-text contrast (WCAG 1.4.11 needs 3:1 for meaningful graphics):

| Pair | Ratio | Verdict |
|---|---:|---|
| classic pointer `#ef4444` vs bg `#f8fafc` | 3.60 | pass |
| classic pointer vs slice it overlaps `#f97316` | **1.34** | fail — tip vanishes on orange |
| classic pointer vs `#fb7185` | **1.40** | fail |
| festival pointer `#dc2626` vs `#fb7185` | **1.79** | fail |
| festival pointer vs bg `#fff7ed` | 4.55 | pass |
| mono pointer `#27272a` vs bg | 13.55 | pass |
| slice divider `#ffffff` vs mono `#f4f4f5` | **1.10** | fail — dividers invisible |
| slice divider `#ffffff` vs mono `#fafafa` (proposed) | 1.04 | fail |

ΔE guide: <15 = not reliably distinguishable at small display size. `min` cyclic adjacent ΔE across normal+deutan+protan: **classic 15.9, festival 11.7, mono 10.1** — all below the safe floor.

---

## 3. Findings

### F-1 — HIGH — Half of all labels render upside down
`src/remotion/WheelComposition.jsx:144` + `src/remotion/wheel-label-layout.js:208` (`rotation: centerDegrees`).
Any slice whose centre angle falls in 90°–270° gets text rotated past vertical, i.e. mirrored-looking (`ǝɔɐɹƃ`). In the default 8-option render at the final frame, `heidi grace frank erin` are all unreadable; at 16 options 8 names are unreadable; at 2 options `Yes` is unreadable. This is the single largest legibility loss and it is worst in the poster frame.

**Fix** (keeps radial orientation — settled decision untouched). Flip by the *rest-frame* screen angle so every label is upright in the final frame:

```js
// wheel-label-layout.js — new input: winnerCenterDegrees
const restAngle = ((centerDegrees - winnerCenterDegrees) % 360 + 360) % 360;
const flip = restAngle > 90 && restAngle < 270 ? 180 : 0;
// ...
rotation: centerDegrees + flip,
```
Caller: `winnerCenterDegrees: getSliceCenterDegrees(props.options.length, props.winnerIndex)`.
During the spin, upside-down-ness is unchanged (it is fixed in wheel space either way) — pure win at rest. Determinism: unaffected. GIF size: unaffected. Render time: unaffected.
Cheaper variant if you prefer winner-independence: `flip = centerDegrees % 360 > 90 && < 270`, which fixes the left half but leaves ~half the labels upside down at rest. Recommend the rest-frame version.

### F-2 — HIGH — Winner is not legible in the poster frame
`WheelComposition.jsx:32-37, 118`. During the hold the only winner signal is a `drop-shadow` glow on the winner `Pie` plus the edge pointer. At 200 px the pointer is ~17 px and the winner name ~9 px. The frame most chat clients show as a static thumbnail does not say who won.

**Fix** — winner-name pill over the hub during the hold. Reads at 200 px, costs 1–2 DOM nodes for 18 frames, 2 palette colours, and matches wheelofnames.com's winner modal convention:

```jsx
{frame >= holdStartFrame && (
  <div style={{
    alignItems: 'center', background: theme.winnerPillBg, borderRadius: size * 0.03,
    color: theme.winnerPillText, display: 'flex', fontSize: winnerFontSize, fontWeight: 800,
    justifyContent: 'center', left: '50%', lineHeight: 1.15, maxWidth: size * 0.74,
    outline: `${Math.round(size * 0.008)}px solid ${theme.winnerPillRing}`,
    padding: `${size * 0.022}px ${size * 0.042}px`, position: 'absolute', textAlign: 'center',
    top: '50%', transform: `translate(-50%, -50%) scale(${0.72 + 0.28 * pillPop})`,
    whiteSpace: 'pre-line', zIndex: 12,
  }}>{winnerLines.join('\n')}</div>
)}
```
with `const pillPop = interpolate(frame, [holdStartFrame, holdStartFrame + 4], [0, 1], {easing: Easing.out(Easing.cubic), extrapolateLeft: 'clamp', extrapolateRight: 'clamp'})`, `winnerFontSize` derived from the existing `estimateTextWidth` against `size * 0.66` starting at `size * 0.085` (≈43 px at 512) and floored at `size * 0.045`, `winnerLines = getLabelLines(option, winnerFontSize, size * 0.66, 2)`.
New theme tokens: classic `winnerPillBg '#111827'`, `winnerPillText '#ffffff'`, `winnerPillRing '#facc15'`; festival `'#7f1d1d' / '#fffbeb' / '#f59e0b'`; mono `'#18181b' / '#fafafa' / '#a1a1aa'`. Contrast: 17.7:1, 14.2:1, 16.9:1.
`zIndex: 12` puts it above the confetti layer (`Confetti.jsx` uses `zIndex: 10`) so the name is never occluded — that is the only confetti interaction in scope here.
Determinism: pure `frame` function. GIF size: flat solid fill, adds ~2 palette entries; the pill is static for 14 of 18 hold frames so inter-frame delta is near zero. Render time: negligible.

### F-3 — HIGH — Spin curve strobes at the start and dies at the end
`src/remotion/wheel-layout.js:27` (`fullTurns = 7`) and `:52-57` (`1 - (1 - p)**3`).
Measured for the default 8 options / 15 fps / 6500 ms spin (98 spin frames, 2432° total):

| | current cubic, 7 turns | easeOut^2, 4 turns | **proposed: 12% ease-in + easeOut^2.2, 5 turns** |
|---|---:|---:|---:|
| peak Δ/frame | **73.7°** (1105°/s) | 27.4° | 40.6° |
| frames > 45° (one slice) → strobe | **22** (1.47 s) | 0 | 0 |
| frames < 4°/frame → crawl | **23** (1.53 s) | — | 15 |
| frames < 1.5°/frame → sub-perceptible | **14** | — | 6 |
| rotation per second | 954 / 665 / 428 / 243 / 110 / 30 / 1 | — | 233 / 525 / 403 / 286 / 177 / 79 / 9 |

74°/frame against a 45° slice is above the stroboscopic threshold: the wheel appears to jitter or run backwards for the first 1.5 s (19 % of the GIF). Then the last 1.5 s crawl, and the wheel is fully frozen from f91 — 0.47 s dead tail *plus* the 1.20 s hold = 1.67 s of a still image before the GIF loops.

**Fix** — replace `getSpinProgress`:

```js
const accelFraction = 0.12;

export const getSpinProgress = (frame, spinFrames) => {
  const p = clamp(frame / Math.max(1, spinFrames), 0, 1);
  if (p < accelFraction) {
    return 0.5 * (p / accelFraction) ** 2 * accelFraction;
  }
  const q = (p - accelFraction) / (1 - accelFraction);
  return accelFraction / 2 + (1 - (1 - q) ** 2.2) * (1 - accelFraction / 2);
};
```
and `fullTurns = 5` at `wheel-layout.js:27`. Verified numerically: continuous, monotonic, `progress(spinFrames) === 1` exactly, so the winner still lands dead centre under the pointer (checked: `rot(98) = 1687.500` = target). Determinism: still a pure function of `frame`. GIF size: **smaller** — 1712° instead of 2432° means smaller per-frame pixel deltas and better LZW runs. Render time: unchanged.
Existing tests on `getSpinProgress` will need updated expectations — flag as an intentional behaviour change, not a public contract change.

### F-4 — HIGH — LCD sub-pixel antialiasing poisons the GIF palette
`WheelComposition.jsx:124-154`. Sampled pixels at a glyph edge on `#f97316`: `#62B4EC`, `#111F7D`, `#C2F6BC`, `#F97391` — saturated cyan/blue/green fringes where only browns are possible. Chromium is rendering the HTML labels with RGB sub-pixel AA. Remotion already passes `--font-render-hinting=none` (`node_modules/@remotion/renderer/dist/open-browser.js:150`) but **not** `--disable-lcd-text`, and `chromiumOptions` does not accept arbitrary flags.

Measured palette pressure per region (source PNG, before quantisation):

| region | unique colours |
|---|---:|
| one 90×30 label crop (`carol`) | **268** |
| mono 100×30 label crop | 236 |
| shadow band (512×60, bottom) | 119 |
| mono flat slice interior 60×60 | 46 |
| whole 512² still (classic) | 6350 |

Consequence: GIF frame 0 uses **249/256** colours in classic and **254/256** in `mono` — a theme with ~15 intended colours exhausts the palette. Everything else then dithers.

**Fix** — force grayscale AA by promoting the label to a composited layer. Append `translateZ(0)` to the existing transform at `WheelComposition.jsx:144`:
```js
transform: `translate(-50%, -50%) rotate(${label.rotation}deg) translateZ(0)`,
```
Zero DOM/render cost, deterministic. **Confidence high but unverified in this repo** (I could not test it without editing source). Verification recipe: render one still, then
`magick still.png -crop 90x30+345+240 +repage -format "%k\n" info:` — expect the 268 to drop below ~40. If `translateZ(0)` is insufficient, `opacity: 0.999` on the same element has the same layer-promotion effect.
Expected payoff: the largest single lever on both GIF size and dither quality.

### F-5 — HIGH — `mono` theme is not a usable wheel
`src/remotion/themes.js:26-37`. `slices[4] = '#f4f4f5'` is **identical to `background`** (contrast 1.00) so the wheel silhouette disappears wherever that slice sits; `stroke="#ffffff"` (`WheelComposition.jsx:115`) gives 1.10:1 against it so the dividers vanish too; two adjacent pairs are ΔE 10–12. At 200 px it reads as a grey blob, not a wheel.

**Fix** — three changes:
1. Theme-specific divider. Replace the hardcoded stroke with a token: `stroke={theme.sliceStroke}` — classic/festival `'#ffffff'`, mono `'#71717a'` (4.0:1 against the lightest slice).
2. Re-ramp mono to even L\* steps in a band that keeps `#18181b` above 4.5:1, ordered to maximise adjacent ΔE (verified: min adjacent ΔE **16.4**, up from 10.1; darkest slice still 4.97:1 for text):
```js
slices: ['#87878f', '#cacad0', '#9d9da5', '#e1e1e5', '#b3b3ba', '#f9f9fa'],
```
   Text contrasts: 4.97 / 10.86 / 6.58 / 13.59 / 8.50 / 16.84 — all pass 4.5:1.
3. Add the bezel from F-6, which is what actually rescues the silhouette.
Determinism/render time: unaffected. GIF size: fewer, better-separated colours → slightly smaller.

### F-6 — MEDIUM — Large soft `wheelShadow` buys little and costs palette; the wheel has no silhouette
`WheelComposition.jsx:74` with `themes.js:4,16,28` (`0 18px 52px rgba(...)`). At 300 % zoom the ramp is actually smooth — no visible banding — and because the shadow div does not rotate it is byte-cheap after frame 1. But it consumes 119 palette entries and it does nothing for the real problem: on a dark chat background the GIF is a bright square with no defined wheel edge, and in `mono` there is no edge at all.

**Fix** — trade the diffuse shadow for an opaque bezel ring plus a tight drop:
```js
// under the rotating layer, above the shadow div
<Circle fill={theme.bezel} radius={radius + size * 0.016}
  style={{left: center - radius - size * 0.016, position: 'absolute', top: center - radius - size * 0.016}} />
```
tokens: classic `bezel '#1f2937'` (14.0:1 vs bg), festival `'#7c2d12'`, mono `'#3f3f46'`; and shrink the shadow to `'0 6px 14px rgba(15, 23, 42, 0.20)'` (footprint ~4× smaller, ~30 palette entries instead of 119).
Payoff: the wheel reads as a deliberate object on both light and dark chat backgrounds, mono gets an edge, and ~90 palette slots go back to the slices. Determinism: static. Render time: +1 SVG circle.
Note the bezel must be inside the *non-rotating* stack so it does not add per-frame delta.

### F-7 — MEDIUM — Adjacent slices collide under colour-vision deficiency; palettes are orderable for free
`themes.js:12,24,36`. Brute-forced all cyclic orderings, scoring each adjacent pair by `min(ΔE_normal, ΔE_deutan, ΔE_protan)`:

| theme | current min | reordered min | change |
|---|---:|---:|---|
| classic | 15.9 | **48.1** | reorder only, same 8 hexes |
| festival | 11.7 | **37.9** | reorder only, same 8 hexes |
| mono (current hexes) | 10.1 | 11.3 | reorder cannot fix — needs F-5 re-ramp |

```js
// classic
slices: ['#f97316', '#14b8a6', '#f472b6', '#60a5fa', '#facc15', '#a78bfa', '#fb7185', '#34d399'],
// festival
slices: ['#fb7185', '#06b6d4', '#84cc16', '#a78bfa', '#f59e0b', '#f472b6', '#fb923c', '#22c55e'],
```
Zero new colours, zero render/size cost, 3× better worst-case separation. Worst current offenders that disappear: classic `#a78bfa`→`#34d399` (ΔE 16 deutan — purple and green look identical), festival `#06b6d4`→`#a78bfa` (ΔE **12 protan**).

### F-8 — MEDIUM — First and last slice share a colour when `optionCount % paletteLength === 1`
`WheelComposition.jsx:98`. With 9 options on the 8-colour palettes, slice 8 and slice 0 are both `#f97316` and sit adjacent — verified visually, they merge into one double-width wedge with only a 2 px white line between. Same at 17 options, and at 7/13 options in `mono` (6 colours).

**Fix**
```js
const paletteLength = theme.slices.length;
const colorIndex =
  index === props.options.length - 1 && props.options.length % paletteLength === 1
    ? Math.floor(paletteLength / 2)
    : index % paletteLength;
const color = theme.slices[colorIndex] ?? theme.slices[0] ?? '#cccccc';
```
Worth a unit test asserting min adjacent ΔE ≥ 30 for every `optionCount` in 2..32 across all three themes.

### F-9 — MEDIUM — `pointerStroke` and the pointer `boxShadow` are silently discarded
`WheelComposition.jsx:173-175`. `clip-path` clips the element's whole rendering, including border and box-shadow. Result (confirmed in a 500 % crop): the diagonal edges have no outline at all, the box-shadow is entirely gone, and only a 2 px sliver of `#991b1b` survives on the vertical back edge — reading as an accidental dark stripe. Combined with contrast 1.34:1 against `#f97316`, the pointer tip is invisible when the winner slice is orange or salmon.

**Fix** — draw the pointer as SVG so the stroke is real:
```jsx
<svg width={size * 0.10} height={size * 0.105} viewBox="0 0 100 105" style={{
  left: center + radius - size * 0.045, position: 'absolute', top: center - size * 0.0525, zIndex: 4,
}}>
  <polygon points="4,52.5 96,5 96,100" fill={theme.pointer}
    stroke={theme.pointerStroke} strokeWidth={7} strokeLinejoin="round" />
</svg>
```
Drop the `boxShadow` entirely (it never rendered and a blurred shadow costs palette). The rounded-join dark outline gives ≥3:1 against every slice in every theme. 1 extra DOM node, no filter, deterministic.

### F-10 — MEDIUM — Pointer has 0.5 % framing margin and no reaction to slice boundaries
`WheelComposition.jsx:176-180`. Verified geometry: at `size=512`, `left = 256 + 209.92 - 10.24 = 455.68`, `width = 53.76`, so the back edge sits at **x = 509.44 — 2.56 px from the canvas edge**; at `size=384` it is **1.92 px**. Not clipped, but visually flush with the frame, and `AbsoluteFill`'s `overflow: hidden` means any outline or size increase *will* clip. Meanwhile the tip overlaps the rim by only `size*0.02` (10 px) and the pointer never moves — the strongest "this is a real wheel" cue is missing.

**Fix (a) framing** — the F-9 snippet already moves it: `left = center + radius - size * 0.045`, `width = size * 0.10` → right edge 494 px, **18 px** margin, and tip overlap rises to 23 px. The 0.41 radius leaves 46 px margins, so this fits comfortably; no need to change `radius`.

**Fix (b) tick/flapper** — deterministic, zero DOM cost, resolves to exactly 0 at rest (proved below):
```js
// wheel-layout.js
export const getPointerDeflectionDegrees = (rotationDegrees, sliceDegrees, maxDeflection = 9) => {
  const phase = (((rotationDegrees % sliceDegrees) + sliceDegrees) % sliceDegrees) / sliceDegrees;
  return -maxDeflection * Math.max(0, 1 - phase * 3);
};
```
applied as `transform: `rotate(${deflection}deg)`` with `transformOrigin: '100% 50%'` on the pointer SVG.
Rest proof: `finalRotation % sliceDegrees = (turns*360 - winnerIndex*slice - slice/2) % slice`; `360 = n*slice`, so the first two terms are exact multiples and the remainder is always `slice/2` → `phase = 0.5` → deflection `0`. The flapper always parks neutral, whatever the winner or option count.
During the slow tail this produces 4–6 legible flicks — by far the cheapest large gain in "reads as a real spin". Determinism: pure `frame` function via `rotation`. GIF size: a few px of the pointer change per frame; negligible.

### F-11 — MEDIUM — Winner glow is an asymmetric smear, not a glow
`WheelComposition.jsx:118`. `drop-shadow` on a `Pie` renders *outside* the shape, so it is overpainted by every later-indexed sibling `Pie` and only survives on the counter-clockwise neighbour. In the 8-option render the yellow glow appears as a dirty greenish smudge bleeding onto the teal `bob` slice and nowhere else. Also `winnerPulse = Math.sin(settle * Math.PI)` is 0 at both ends of the hold, so the final frame gets the *minimum* glow (10 px) — the poster frame gets the weakest treatment. And `filter`+blur is the worst possible effect for a 256-colour palette.

**Fix** — replace the blur with an opaque stroke on the winner slice; drop the `drop-shadow` line entirely:
```jsx
<Pie
  fill={color}
  stroke={isWinner && frame >= holdStartFrame ? theme.winnerStroke : theme.sliceStroke}
  strokeWidth={isWinner && frame >= holdStartFrame ? 4 + Math.round(winnerPulse * 3) : 2}
  ...
/>
```
tokens: classic `winnerStroke '#111827'`, festival `'#7f1d1d'`, mono `'#18181b'`. Adds 1 colour, no blur, no z-order artefact, 4 discrete pulse states (cheap inter-frame delta). Keep `winnerGlow` only if the confetti reviewer still needs it; otherwise delete the token.
Also rename or repurpose `settle` (`WheelComposition.jsx:32`) — it drives only the pulse; nothing settles.

### F-12 — MEDIUM — `lineHeight: 1` collides Vietnamese diacritics between wrapped lines
`WheelComposition.jsx:137` + `wheel-label-layout.js:206` (`height: Math.min(availableHeight, fontSize * lines.length)`), with `overflow: 'hidden'` at `:138`.
Rendered `Bún đậu mắm tôm Hà Nội` over 3 lines: the dot-below of `ậ` visually merges with the breve of `ắ` on the next line. No hard clipping was observed at 512 with Quicksand (`Ổn`, `Ừ`, `Ạ`, `Nghĩ`, `Phạm Dũng` all render complete), but the box height equals exactly `fontSize × lines`, so the margin is zero — any font fallback with taller metrics *will* clip, and the project explicitly supports Vietnamese.

**Fix** — give the line box breathing room and shrink to fit rather than clip:
```js
// WheelComposition.jsx
lineHeight: 1.16,
// wheel-label-layout.js, after fontSize is chosen
const lineHeightRatio = 1.16;
const blockHeight = fontSize * lineHeightRatio * lines.length;
const fittedFontSize = blockHeight > availableHeight
  ? Math.max(minLabelFontSize, Math.floor(fontSize * (availableHeight / blockHeight)))
  : fontSize;
// return fontSize: fittedFontSize, height: Math.min(availableHeight, fittedFontSize * lineHeightRatio * lines.length)
```
Also update `maxLines` at `:185` to use the same ratio: `Math.floor(availableHeight / (baseFontSize * lineHeightRatio))`.
Does not re-litigate the settled wrap/adaptive-size decisions — it only stops the box from being exactly flush.

### F-13 — MEDIUM — Font size is capped at 24 px, wasting low-option wheels
`wheel-label-layout.js:24,156`. `baseSize = max(10, min(24, radius / max(5.8, optionCount * 0.33)))`. Because the `5.8` floor dominates until `optionCount > 17`, `radius / 5.8 = 36.2` at 512 — so **the 24 cap is active for every option count from 2 to 17**. A 2-option wheel with `Yes` / `No` renders 24 px text inside a 210 px-wide, 417 px-tall track: at a 200 px display that is 9.4 px. Verified in the 2-option still — enormous empty slices, tiny text.

**Fix** — raise the cap and clamp it by the wedge height so dense wheels do not regress:
```js
// in getRadialLabelLayout, before getLabelFontSize
const maxFontSize = Math.min(radius * 0.15, availableHeight * 0.82);   // 31.5px @512, and 21px @32 options
```
and pass it through instead of the hardcoded `24`. At 32 options `availableHeight = 26`, so `maxFontSize = 21` < the current 24 — which also removes a latent vertical clip that exists today. At 2 options it becomes 31.5 px (+31 % legibility at thumbnail size); width-fitting still shrinks long labels exactly as before, so no regression for `Nguyễn Văn 32`-style labels.

### F-14 — MEDIUM — Maximum accepted request produces an unusable artefact
`src/schemas/wheel-request.js:38-41` allows `durationMs 10000 + holdMs 2500 + fps 20 + size 512` = 250 frames. Measured on this dev machine (single render, warm bundle):

| case | bytes | render |
|---|---:|---:|
| classic, 8 opts, 512, 15 fps, 7.7 s (default) | **2,036,868** | 4,255 ms |
| mono, 6 opts, 512, 15 fps, 7.7 s | 1,775,252 | 2,688 ms |
| festival, **32 opts, 512, 20 fps, 12.5 s** (schema max) | **14,261,781** | **10,274 ms** |

`docs/render-benchmarks.md:13-14` still says TBD for the 512 fixtures; these numbers fill that gap. A 14 MB GIF is hostile in a chat client, and 10.3 s here will exceed `RENDER_TIMEOUT_MS=15000` on the documented 1–2 vCPU container. Even the *default* 2.0 MB is heavy for inline playback on mobile.

**Fix, non-breaking first** — F-4 (grayscale AA) and F-6 (smaller shadow) free ~200 palette entries, and F-3 cuts total rotation from 2432° to 1712°; all three shrink per-frame entropy. I could not measure the combined saving without editing source — flag as expected, not proven.
**If that is not enough**, the breaking option is to narrow the schema: reject `durationMs + holdMs` such that total frames > 160 (e.g. `superRefine` with a clear message), or drop `fps: 20`. That rejects requests that are valid today → **breaking change**, only justified if the container actually times out; measure in Docker first.

### F-15 — LOW — Light-only backgrounds look like a bright box on dark chat themes
`themes.js:3,15,27` (`#f8fafc`, `#fff7ed`, `#f4f4f5`). Composited the three 200 px final frames on `#1e1e1e`: all three read as leaked white rectangles, and `mono` additionally has no discernible wheel edge.

**On transparency — do not pursue.** GIF transparency is a single binary palette index: the wheel's antialiased rim, the soft shadow and the text halo cannot blend against an unknown background, so you get hard white/grey fringes on every curve and glyph. On top of that, Telegram's `sendAnimation` re-encodes GIFs to MP4 for most clients, which drops alpha entirely — so the cost is real and the benefit does not survive the transport. (Whether Remotion's `codec: 'gif'` even emits alpha I did not verify; moot given the transport.)

**Recommended instead:** ship F-6's dark bezel — it gives the wheel an intentional edge on any background at the cost of one palette colour, which is 90 % of the benefit for none of the risk.
**Optional, additive:** a 4th `night` theme (`background '#0b1220'`, `text '#0b1220'` on light slices, `textHalo 'rgba(255,255,255,0.9)'`, `bezel '#e2e8f0'`). Adding a value to the `theme` union is backward compatible for existing clients (no old value changes meaning), but it does extend the documented request contract and needs a README/`docs/miti99bot-integration.md` update — flagging it as a contract *addition*, and it conflicts with "three theme names are settled", so it needs your explicit call.

### F-16 — LOW — 4-direction text halo costs 4× text rasterisation for contrast that is already met
`WheelComposition.jsx:142`. Every label paints 4 extra offset copies — 128 extra text draws per frame at 32 options. Section 2 shows `text:slice` is ≥5.39:1 in every theme on every slice, so the halo is not doing contrast work; it earns its keep only where confetti or a slice boundary crosses a glyph. It also thickens glyphs at small sizes.
**Fix (low priority):** drop to 2 offsets (`0 1px 0`, `0 -1px 0`) — keeps the vertical separation that matters against slice borders at half the cost. Alternatively keep as-is; this is the least urgent item and worth re-checking only after F-4 lands, since the halo's colour cost is currently masked by the AA fringing.

### F-17 — LOW — Centre hub is decorative only and will be covered by the winner pill
`WheelComposition.jsx:160-169`. Two concentric circles (`0.09` ring, `0.06` core) look clean and hide the label-track convergence — keep them. Just note they sit *under* the F-2 pill, which is fine (the pill is the payoff). No change needed; recorded so the hub is not "fixed" later by accident.

---

## 4. Prioritised list

### Quick wins (<30 min each)

| # | Change | Payoff | Risk |
|---|---|---|---|
| 1 | F-1 label flip (one derived value + `+ flip`) | Removes 3–8 unreadable names per render; biggest legibility gain | none — deterministic, no size/time cost |
| 2 | F-7 palette reorder (two array literals) | worst-case adjacent ΔE 15.9→48.1 / 11.7→37.9 under CVD | none — same hexes |
| 3 | F-4 `translateZ(0)` on the label | 268→(expected) <40 colours per label crop; unlocks the whole palette | low; **verify with `%k` count** — unproven |
| 4 | F-3 spin curve + `fullTurns = 5` | kills 22 strobe frames and 14 dead frames; also shrinks the GIF | low; existing `getSpinProgress` test expectations change |
| 5 | F-10b pointer tick | strongest "real wheel" cue; provably neutral at rest | none — pure `frame` function |
| 6 | F-11 winner stroke instead of `drop-shadow` | removes the one-sided smear; winner strongest in the poster frame; drops a blur filter | none — fewer effects |
| 7 | F-8 last-slice colour guard | fixes merged wedges at 7/9/13/17 options | none |
| 8 | F-12 `lineHeight: 1.16` + fit-not-clip | ends diacritic collisions; removes latent clipping | low; slightly smaller 3-line labels |
| 9 | F-9 pointer as SVG + F-10a reposition | pointer gets a real outline (≥3:1 on every slice) and 18 px framing margin | low; +1 DOM node |

### Larger reworks

| # | Change | Payoff | Risk |
|---|---|---|---|
| A | F-2 winner-name pill (new tokens, fit logic, pop easing) | The poster frame finally states the winner at 200 px — highest user-visible value in the report | medium: needs a size-fit path and 2-line wrap; +2 palette colours; must sit above `Confetti`'s `zIndex: 10` |
| B | F-5 + F-6 mono re-ramp, `sliceStroke` token, bezel ring | Makes `mono` a real theme; gives all themes a silhouette on dark chat backgrounds; frees ~90 palette entries | medium: touches all three theme objects + the hardcoded `stroke="#ffffff"`; snapshot/visual review needed |
| C | F-13 font-size cap by `radius * 0.15` clamped to `availableHeight * 0.82` | +31 % text size on 2–6 option wheels; removes a latent clip at 32 options | medium: changes sizing for every render — re-check 2, 8, 16, 24, 32 options |
| D | F-14 GIF weight programme (measure in Docker after 3/4/6/B land, then decide) | Default 2.0 MB and max 14.3 MB are both too heavy | high if it ends in a schema narrowing — that is a **breaking change**; do the non-breaking levers first |
| E | F-15 optional `night` theme | Fixes dark-background presentation properly | contract addition + conflicts with the settled 3-theme decision; needs your approval |

Suggested order: 1, 2, 3, 4, 5, 6, 7, 8, 9 → A → B → C → measure D.

---

## 5. Not verified empirically

- **F-4 fix effectiveness.** The sub-pixel fringing is measured and certain; that `translateZ(0)` (or `opacity: 0.999`) removes it in this Chromium build is high-confidence but untested, because testing requires editing `WheelComposition.jsx`. Verification recipe given in F-4.
- **GIF size saving from F-3/F-4/F-6.** Direction is certain (less rotation, fewer colours, smaller shadow footprint); the magnitude is not measured. Palette pressure was measured indirectly (249–254 of 256 colours used in frame 0; 268 colours in one label crop; 119 in the shadow band).
- **Render timeout risk in the deployment target.** 10,274 ms for the schema-max case was measured on this Windows dev machine, not on the documented 1–2 vCPU container. The `RENDER_TIMEOUT_MS=15000` breach is inferred, not observed.
- **Remotion `codec: 'gif'` alpha support.** Not tested — moot, since F-15 recommends against transparency on transport grounds.
- **Motion perception.** Strobing was derived numerically (22 frames above 45°/frame) from still-frame geometry, not judged by watching the GIF frame-stepped at 15 fps in a chat client.
- **Font fallback.** All Vietnamese checks used Quicksand as resolved in this environment. Diacritic clipping margins on the `Inter` / `Noto Sans` / `Segoe UI` fallbacks in the `baseFont` stack (`WheelComposition.jsx:16-17`) were not tested — an extra reason to adopt F-12 rather than rely on `lineHeight: 1` fitting exactly.
- `pnpm render:fixtures` was **not** run (it would rewrite `fixtures/*.gif`). Equivalent and broader coverage was rendered to the scratchpad instead; the repo is clean apart from other agents' reports.

---

## 6. Unresolved questions

1. **Winner pill (F-2)** — is covering the hub during the hold acceptable, or must the wheel stay fully visible? If it must, the only alternative at a square 512 canvas is shrinking `radius` from 0.41 to ~0.35 to open a caption band, which costs label size everywhere.
2. **`night` theme (F-15)** — the three theme names are settled, but "GIFs look like a bright box on dark chat backgrounds" is the concern the brief itself raised. Add a 4th value (additive, backward compatible, needs docs) or accept the bezel-only mitigation?
3. **GIF weight (F-14)** — is the 2.0 MB default acceptable to miti99bot, and has a 14 MB max-parameter render ever been requested in practice? The answer decides whether D ends in a schema narrowing.
4. **Spin curve test expectations (F-3)** — existing `getSpinProgress` tests encode the cubic. Confirm they are treated as implementation detail, not a public contract.
5. **`fullTurns = 5`** — 7 turns is currently hardcoded with a default parameter at `wheel-layout.js:27`. Should turn count stay fixed, or scale with `durationMs` (e.g. `Math.round(durationMs / 1300)`) so a 3 s request does not feel rushed and a 10 s one does not crawl?
