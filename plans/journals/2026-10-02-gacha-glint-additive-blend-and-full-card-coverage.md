---
title: "Gacha glint: additive blend and full-card coverage"
date: 2026-10-02
summary: "Mirror glint now blends additively in the card's accent tone and sweeps every corner of the card"
---

# Gacha glint: additive blend and full-card coverage

## What happened
- The user asked for an additive blend on the gacha card's mirror glint. If that cost too much GPU, they wanted lower alpha and a colour that matches the card's tone instead.
- They also noticed the glint did not cover the whole card.

## Root cause (coverage)
- `.wish-glint` was a card-sized box carrying a 135deg gradient, translated from (-100%,-100%) to (100%,100%).
- A band only paints inside its own box, so as the box slid diagonally the top-right and bottom-left corners were never inside it at the moment the band passed. With a 63:88 card, the top-right corner sits about 5px outside the box when the band reaches it.

## Changes
- `src/gacha/page/page.css`:
  - The glint clip uses `mix-blend-mode: plus-lighter`.
  - Band and flare colours are tinted with `--glint`, the card accent, with peak alpha around 0.75.
  - The band box is now 300% of the card, centred on it, with gradient stops rescaled by 1/3. The flare width goes from 46% to 15.3%.
- `src/gacha/page/page.js`:
  - `--glint` is set on the clip itself, because the card flies in an overlay outside #stage.
  - The sweep now runs from translate(-26.7%) to translate(26.7%), which is 0.8 card sizes each way.

## Evidence
- Docker render timings showed additive and normal blending within noise: 14.2s vs 14.4s and 11.6s vs 11.5s per clip. Headless Chrome composites in software here, and blending costs nothing measurable.
- Frames at 3.8s (3-star) and 3.75s (5-star) show the band reaching the card edges, tinted blue and gold.

## Next steps
- The user reviews the fixtures before commit.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
