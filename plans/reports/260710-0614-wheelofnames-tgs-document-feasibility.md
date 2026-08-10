---
type: researcher
date: 2026-07-10
conducted_at: 2026-07-10T06:14:00Z
---

# Research Report: Wheel of Names as a TGS Document

## Navigation

- [Executive Summary](#executive-summary)
- [Research Methodology](#research-methodology)
- [Current Architecture](#current-architecture)
- [Key Findings](#key-findings)
- [Comparative Analysis](#comparative-analysis)
- [Timing Recommendation](#timing-recommendation)
- [Implementation Impact](#implementation-impact)
- [Recommendation](#recommendation)
- [Resources and References](#resources-and-references)
- [Next Steps](#next-steps)
- [Unresolved Questions](#unresolved-questions)

## Executive Summary

Sending `wheelofnames.tgs` through Telegram `sendDocument` is technically
possible. Bots may multipart-upload general files up to 50 MB, and this repo's
Go library already supports `SendDocument` plus `InputFileUpload`.

The exact proposal is not attractive for the current user experience. Telegram
defines `sendDocument` as general-file delivery and does not promise inline or
auto-playing TGS document previews. Inline TGS animation is part of the sticker
contract. Therefore users may see a downloadable `.tgs` attachment rather than
the wheel animation.

Producing TGS is also not a GIF conversion setting. The existing
`tiennm99/wheelofnames` renderer uses React/Remotion, Chromium-rendered PNG
frames, and Remotion's GIF encoder. Remotion does not emit Lottie/TGS. A TGS
version needs a second vector renderer/serializer. Dynamic arbitrary labels are
the hardest part because Telegram TGS disallows text layers; glyphs would need
conversion to vector paths, threatening the 64 KB sticker-format target.

Verdict: **transmission feasible; useful inline wheel experience not reliably
feasible with `sendDocument`; engineering/value ratio poor.** If the goal is a
smaller inline animation with caption, H.264 MP4 through the existing
`sendAnimation` method is the strongest option to benchmark first.

## Research Methodology

- Conducted: 2026-07-10 UTC
- Sources: current `miti99bot`, `tiennm99/wheelofnames` at commit
  `f13055651e7a18a59b024f7b22266611cc843389`, Telegram Bot API and sticker
  specification, Remotion renderer documentation
- Criteria: Telegram UX, file size, renderer compatibility, timing quality,
  implementation scope, operational cost
- Boundary: research only; no implementation or live Telegram client test
- Search terms: `sendDocument`, TGS, Lottie, Remotion codec, 3-second wheel,
  Telegram document preview

## Current Architecture

```text
miti99bot
  POST /api/gif (options, winner, 6000ms spin, 1000ms hold, 20 FPS, 512px)
      -> wheelofnames React/Remotion composition
      -> Chromium renders PNG frames
      -> Remotion encodes GIF
  sendAnimation(GIF + spoiler caption)
      -> Telegram inline animation
```

Current facts:

- Bot requests a 6.0s spin plus 1.0s result hold: 7.0s total.
- Bot requests 20 FPS and 512x512.
- Renderer accepts only 12, 15, or 20 FPS.
- Renderer schema currently requires at least 3.0s spin and 0.5s hold, so its
  shortest accepted total is 3.5s.
- Renderer supports up to 32 options with 40 characters each.
- Recorded 384px/12 FPS/3.5s GIFs are about 298-316 KB. A production-equivalent
  512px/20 FPS/7s benchmark is not recorded.
- Current bot sends the result as a spoiler caption attached to the animation.

The proposed architecture is materially different:

```text
miti99bot
  POST /api/tgs (new contract and timing)
      -> new vector wheel model
      -> new Lottie JSON serializer
      -> validate Telegram-supported features
      -> gzip as .tgs
  sendDocument(TGS + spoiler caption)
      -> Telegram document attachment
      -> inline playback not guaranteed
```

## Key Findings

### 1. Telegram Can Transport the File

`sendDocument` accepts an uploaded file of any type up to 50 MB and returns a
message whose media is a `Document`. The pinned Go dependency exposes all
required fields, including document upload, caption, thread ID, and optional
content-type detection control.

The animated-sticker restrictions (3s, 64 KB, 512x512, 60 FPS) are not Bot API
limits on a generic document. The document limit is 50 MB. Following the TGS
restrictions still makes sense if the artifact should remain a valid Telegram
animation or may later be sent as a sticker.

### 2. `sendDocument` Does Not Promise TGS Playback

Telegram documents inline animation for `Animation` (GIF or silent H.264 MP4)
and `Sticker` (`.WEBP`, `.TGS`, `.WEBM`) message types. It documents
`sendDocument` only as sending general files.

Inference from the official message contracts: a `.tgs` uploaded as a document
cannot be relied on to auto-play or render like a sticker. Client behavior may
vary and is not an API guarantee. This is the proposal's decisive risk. A
downloadable file users must open elsewhere is worse than the current GIF.

### 3. TGS Requires a New Renderer

The current renderer's output path is frame-based:

```text
React DOM/CSS -> Chromium -> PNG frames -> GIF encoder
```

TGS is gzip-compressed Lottie vector-animation JSON. There is no reliable
GIF-to-TGS conversion: raster frames do not contain the vector geometry,
transforms, or typography needed for compact Lottie output. Remotion's media
codecs include GIF and video formats such as H.264; TGS is not an output codec.

A TGS renderer would need to independently serialize:

- wedge vector paths and colors
- hub and pointer vector paths
- wheel rotation keyframes/easing
- winner indication using supported vector properties
- every label as glyph outlines, or omit labels
- JSON normalization, gzip, and TGS validation

The current CSS shadow, drop-shadow winner glow, HTML text, and font fallback
cannot be transferred directly because Telegram's TGS requirements disallow
layer effects and text.

### 4. Dynamic Labels Undermine the Size Advantage

The wheel's value comes from arbitrary user labels, including Vietnamese and
other Unicode text. Telegram-compatible TGS cannot use text layers. Converting
every unique glyph to vector paths requires font shaping, path generation, and
font coverage. Up to 32 x 40 characters creates a large worst case.

Consequences:

- 64 KB is plausible only for a simplified wheel with few short labels, or no
  labels; it is not a safe general contract without empirical prototypes.
- Font files and fallback behavior become renderer concerns.
- Unicode shaping and emoji are substantially harder than current browser text.
- Repeated glyph/path data can erase much of TGS's size advantage.

If sent only as a document, exceeding 64 KB is allowed, but then the proposal
loses both guaranteed playback and its strongest size target.

### 5. Operational and Security Considerations

- Keep current option-count and character-count bounds; vector path generation
  expands attacker-controlled text into CPU and memory work.
- Validate generated JSON, compressed and uncompressed sizes, layer count,
  duration, frame rate, and unsupported features before upload.
- Generate TGS internally; do not accept user-provided TGS payloads without
  decompression limits and schema validation.
- A second render path duplicates layout logic unless wheel geometry is first
  extracted into a renderer-neutral model.

## Comparative Analysis

| Option | Telegram UX | Renderer effort | Expected size | Caption | Main risk |
|---|---|---:|---:|---|---|
| Current GIF + `sendAnimation` | Inline/autoplay | Existing | Largest | Yes | Bandwidth/render time |
| 3s GIF + `sendAnimation` | Inline/autoplay | Low | Lower than current | Yes | Still GIF compression |
| TGS + `sendDocument` | Generic attachment; playback not guaranteed | High | Unknown; potentially small | Yes | Users may not see animation |
| TGS + `sendSticker` | Inline sticker | High | Must target 64 KB | No sticker caption | Labels/features may fail limits |
| H.264 MP4 + `sendAnimation` | Inline/autoplay | Low-medium | Likely much smaller than GIF; benchmark needed | Yes | Encoding/client tuning |

H.264 is a closer fit to current architecture: Remotion already renders H.264,
and Telegram explicitly accepts silent H.264/MPEG-4 AVC through
`sendAnimation`. It preserves the spoiler caption and browser-rendered labels.
No size claim should be finalized until identical 3s fixtures are benchmarked.

## Timing Recommendation

If a 3-second-compatible animation prototype is created, use:

| Phase | Duration | 60 FPS frames | Behavior |
|---|---:|---:|---|
| Spin/decelerate | 2300ms | 138 | Three full turns, cubic ease-out to winner |
| Winner hold | 650ms | 39 | Stationary wheel; simple winner emphasis |
| Total | 2950ms | 177 | 50ms safety margin below 3 seconds |

Why three turns: the current wheel uses seven turns over six seconds with a
cubic ease-out. Compressing seven turns into 2.3s would start near nine
rotations/second and be visually noisy. Three turns preserves roughly the
current initial angular speed while making deceleration readable.

The 650ms hold is short but sufficient for confirmation because the caption
also states the result. If no caption is present, favor 2200ms spin plus 750ms
hold. Timing has little UX value when TGS is sent as a non-previewed document.

Required renderer changes even for a GIF/MP4 timing experiment:

- lower `durationMs` minimum below 3000
- permit total duration below 3500ms
- add 60 FPS only if targeting valid TGS; GIF/MP4 need not use 60 FPS
- reduce `fullTurns` from 7 to 3 for the short profile
- verify the winner lands exactly after frame rounding

## Implementation Impact

If this exact TGS-document option is later chosen:

### `tiennm99/wheelofnames`

- Add a renderer-neutral wheel geometry/timing model.
- Add `/api/tgs` and a response MIME/validation contract.
- Implement Lottie vector serialization and gzip packaging.
- Convert supported text to glyph paths or define label restrictions.
- Simplify unsupported shadow/glow effects.
- Add format, timing, size, Unicode, and winner-alignment tests.
- Benchmark sparse and worst-case option sets.

### `miti99bot`

- Change the API URL/Accept/content-type and magic validation from GIF to TGS.
- Change the filename and response byte limit.
- Replace `SendAnimationParams` with `SendDocumentParams`, retaining caption,
  parse mode, thread ID, and text fallback.
- Update recording-bot assertions and all wheel handler/client tests.
- Update user-facing GIF wording and deployment docs/config contract.

No dependency change is necessary in the bot.

## Recommendation

Do not implement **TGS via `sendDocument`** as the primary wheel response unless
a real Telegram Android/iOS/Desktop client matrix proves acceptable inline
playback. The Bot API provides no such guarantee.

Decision order:

1. Reduce the existing animation to the 2.3s spin + 0.65s hold profile.
2. Benchmark identical GIF and silent H.264 MP4 outputs at 512px.
3. Prefer H.264 with `sendAnimation` if it materially reduces bytes while
   retaining inline playback and caption.
4. Prototype TGS only if sticker-style delivery or vector rendering is itself
   a product requirement, and first test a no-label/few-label wheel against
   Telegram's validator and clients.

## Resources and References

### Official Documentation

- [Telegram Bot API: `sendDocument`](https://core.telegram.org/bots/api#senddocument)
- [Telegram Bot API: `sendAnimation`](https://core.telegram.org/bots/api#sendanimation)
- [Telegram Bot API: `sendSticker`](https://core.telegram.org/bots/api#sendsticker)
- [Telegram animated TGS requirements](https://core.telegram.org/stickers#animated-stickers-and-emoji)
- [Remotion `renderMedia()`](https://www.remotion.dev/docs/renderer/render-media)

### Current Implementations

- [`wheelofnames` GIF renderer](https://github.com/tiennm99/wheelofnames/blob/f13055651e7a18a59b024f7b22266611cc843389/src/render/render-gif.js)
- [`wheelofnames` request limits](https://github.com/tiennm99/wheelofnames/blob/f13055651e7a18a59b024f7b22266611cc843389/src/schemas/wheel-request.js)
- [`wheelofnames` Remotion composition](https://github.com/tiennm99/wheelofnames/blob/f13055651e7a18a59b024f7b22266611cc843389/src/remotion/WheelComposition.jsx)
- [`wheelofnames` recorded benchmarks](https://github.com/tiennm99/wheelofnames/blob/f13055651e7a18a59b024f7b22266611cc843389/docs/render-benchmarks.md)

## Next Steps

No implementation now. If the idea advances:

1. Decide whether inline/autoplay is mandatory.
2. Run a Telegram client-matrix spike with one hand-authored TGS document.
3. Benchmark 2.95s GIF versus H.264 before building a new vector renderer.
4. Only then estimate TGS implementation from a few-label vector prototype.

## Unresolved Questions

- Must the wheel auto-play inline on Telegram clients?
- Must all option labels remain visible inside the wheel?
- Is the target specifically under 64 KB, or only smaller than the current GIF?
- Is silent H.264 MP4 acceptable if it preserves the current message UX?
