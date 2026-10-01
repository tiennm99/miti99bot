# Research: escalating the gacha wish animation by tier

Conducted 2026-10-01. Target: `tiennm99/wheelofnames` `src/remotion/GachaComposition.jsx`.

## Outcome

Genshin signals rarity mainly through **colour, size, and how much the screen
is taken over**, not through different choreography. Each tier keeps the same
beats (meteor, flash, reveal, star pops), and the higher tier gets more of
everything: a bigger, longer meteor; a halo; more particles; a stronger flash;
and a richer reveal. Our current composition already follows the beats but
scales almost nothing except colour, which is why 3★, 4★, and 5★ feel the same.

## How the source game escalates

| Beat | 3★ | 4★ | 5★ |
|---|---|---|---|
| Meteor | Blue, thin trail | Purple, brighter, wider trail | Gold, largest, longest trail |
| Tell before landing | none | none | rainbow-like ring forms around the star ([community report](https://genshin-impact.fandom.com/f/p/4400000000000309937)) |
| Screen | night sky | night sky | gold light floods the screen, cascade of sparkles ([overview](https://img.krmangalam.edu.in/star-base/genshin-impact-5-star-wish-animation-secrets-1764806225)) |
| Reveal | plain item card | character/weapon reveal with streaks | same, plus the strongest glow and the longest build-up |
| Stars | pop in gold, one at a time | same | same; the count itself is the payoff |

The fandom wiki page for Wish was not reachable (HTTP 402), so the table above
also relies on well-known gameplay behaviour; treat the 4★ "brighter trail"
row as observed convention, not documented spec.

## Techniques that fit our renderer

The renderer draws DOM/CSS frames in Chrome headless and encodes H.264.
Measured cost today is about 6.6–6.9 s per 7 s clip, against a 15 s
production timeout, so there is headroom but not unlimited headroom.

- **Scale the existing layers by tier** (cheapest, biggest effect): meteor head
  size, trail length and sample count, spark count, ray opacity, mote count.
- **Halo ring for 5★** on the meteor before landing, drawn as a
  `conic-gradient` rainbow ring with a radial mask, matching the in-game tell.
- **Screen flood for 5★**: tint the sky gold as the meteor nears the ground and
  make the flash longer and warmer.
- **Camera shake** on impact for 4★ and 5★: decaying `translate` on the whole
  frame; this is a standard impact device (Remotion templates ship one, see
  [remotion-templates](https://github.com/reactvideoeditor/remotion-templates)).
- **Shockwave rings**: one ring for 3★, two for 4★, three plus a starburst for
  5★.
- **Rank letter emblem** (`B`/`A`/`S` per the user's request) instead of the
  label's first character, with a heavier frame and a sheen sweep at 5★.
- **Falling sparkle rain** behind the 5★ reveal ("raining stars").

`@remotion/effects` (glow, lightTrail, starburst; from v4.0.464,
[docs](https://www.remotion.dev/docs/effects/api)) is not installed and applies
only to specific Remotion components. Adding it would be a new dependency for
effects we can already draw with gradients, so it is not recommended.

## Recommendation

Keep one composition and drive every effect from a per-tier "intensity" table
in `gacha-timeline.js`, so the escalation is data, testable, and tunable.
Re-measure render time after the change; keep it under about 10 s.

## Unresolved questions

- None blocking. The exact 4★ visual delta in the source game is convention,
  not documented.
