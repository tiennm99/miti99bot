---
type: researcher
date: 2026-07-10
conducted_at: 2026-07-10T06:32:00Z
---

# Research Report: In-House Wheel Rendering with GIF or Video

## Navigation

- [Executive Summary](#executive-summary)
- [Research Scope](#research-scope)
- [Current Constraints](#current-constraints)
- [Technical Options](#technical-options)
- [Format Decision](#format-decision)
- [Recommended Architecture](#recommended-architecture)
- [Performance and Quality](#performance-and-quality)
- [Security and Reliability](#security-and-reliability)
- [Implementation Impact](#implementation-impact)
- [Validation Plan](#validation-plan)
- [Resources and References](#resources-and-references)
- [Next Steps](#next-steps)
- [Unresolved Questions](#unresolved-questions)

## Executive Summary

Best technical direction: **keep the existing React/Remotion wheel composition,
render silent H.264 MP4 locally, and continue sending it with Telegram
`sendAnimation`.** This preserves the current geometry, colors, shadows,
winner glow, fonts, label layout, easing, caption, and inline/autoplay behavior.
It removes the renderer HTTP service and its URL/token, while avoiding a risky
visual rewrite.

"All in house" should mean one deployed container, no renderer HTTP listener,
no `WHEELOFNAMES_API_URL`, no renderer authentication token, no runtime browser
download, and no external render service. The Go bot starts a local persistent
Node/Remotion worker and communicates through stdin/stdout plus controlled
temporary files. Telegram upload remains the only media network step.

H.264 is preferred over GIF. Telegram explicitly accepts silent H.264/MPEG-4
AVC as an animation. H.264 preserves far more color detail than GIF's
256-color palette and should compress the mostly-static wheel frames much more
efficiently. Exact size and latency remain benchmark gates; no successful
H.264 benchmark exists yet.

Main trade-off: exact visual reuse requires Node, Chromium, fonts, and Remotion
inside the bot container. The current tiny distroless/static image cannot run
them. Expect a much larger image and materially higher render-time CPU/RAM.

## Research Scope

- Conducted: 2026-07-10 UTC
- Goal: local rendering, GIF or video, current visual fidelity, no renderer API
- Sources: current `miti99bot`; `tiennm99/wheelofnames` at commit
  `f13055651e7a18a59b024f7b22266611cc843389`; Telegram Bot API; Remotion
  renderer docs; Go `image/gif` docs
- Compared: pure-Go GIF, Go frames plus FFmpeg, local Remotion worker
- Boundary: research only; no project implementation

### Definition of "No API"

Recommended interpretation:

- no HTTP call to a wheel renderer
- no renderer TCP port or public/private service
- no external rendering vendor
- no runtime download of code, fonts, Chromium, or media tools
- local child-process IPC is allowed

If "no API" instead means one Go process with no child runtime, exact parity
with the current Remotion composition is not realistic. The pure-Go option can
approximate it but becomes a separate renderer requiring visual maintenance.

## Current Constraints

### Bot Deployment

The current image is:

```text
Go 1.26 static build -> distroless/static non-root runtime -> /server only
```

It contains no shell, Node, Chromium, fonts, or FFmpeg. It cannot execute the
current renderer locally without a Docker runtime redesign.

### Wheel Renderer

The current `wheelofnames` project uses:

- React 19 and Remotion 4.0.485
- Chromium-rendered DOM/CSS frames
- Remotion `renderMedia()`
- PNG frame rendering, then GIF encoding
- Noto fonts, including Vietnamese and color emoji
- one render at a time by default

Current bot profile:

| Setting | Value |
|---|---:|
| Canvas | 512x512 |
| Spin | 6000ms |
| Winner hold | 1000ms |
| Total | 7000ms |
| FPS | 20 |
| Frames | 140 |
| Full turns | 7 |

Recorded renderer measurements cover only a smaller fixture:

| Fixture | Output | Bytes | Render time |
|---|---:|---:|---:|
| 384px, 12 FPS, 3.5s cold | GIF | 315,661 | 13.9s |
| 384px, 12 FPS, 3.5s warm service | GIF | 298,374 | 4.1s |

No production-equivalent 512px/20 FPS/7s GIF measurement or H.264 measurement
is recorded.

## Technical Options

### Option A: Pure-Go Raster Renderer and GIF

Flow:

```text
Go wheel geometry + Go font/layout + raster frames + image/gif -> sendAnimation
```

Advantages:

- retains one static Go binary and distroless image
- no child process or browser
- simplest production runtime
- deterministic, fully offline renderer

Disadvantages:

- reimplement every current visual and animation behavior
- browser-quality Unicode shaping, fallback fonts, and color emoji are hard
- must recreate anti-aliased wedges, radial text, shadows, glow, and CSS effects
- GIF allows at most 256 palette colors per frame
- palette quantization/dithering can damage text edges, shadows, and glow
- standard `image/gif.EncodeAll` holds all paletted frames before writing
- duplicates visual logic now owned by the Remotion composition

Verdict: good only if minimal runtime matters more than exact visual parity.

### Option B: Pure-Go Frames Piped to Local FFmpeg H.264

Flow:

```text
Go wheel geometry + Go font/layout -> raw frames -> local FFmpeg -> MP4
```

Advantages:

- video size/color advantages
- raw frames can stream to FFmpeg with bounded memory
- no Chromium or Node
- no HTTP API

Disadvantages:

- still requires the full visual rewrite
- adds FFmpeg executable and Linux runtime libraries
- loses current distroless/static simplicity
- child-process lifecycle, cancellation, temp/output bounds still required
- font shaping remains the largest fidelity risk

Verdict: inferior to reusing Remotion; it pays both rewrite and runtime costs.

### Option C: Existing Remotion Composition as a Local Worker

Flow:

```text
Go bot -> local JSON-lines worker -> Remotion/Chromium -> H.264 MP4
       <- result metadata + controlled temp path
Go bot -> Telegram sendAnimation
```

Advantages:

- highest visual parity; reuses the current composition directly
- browser keeps current Unicode/font behavior
- H.264 is already a supported Remotion codec
- no renderer HTTP surface, URL, token, or network failure mode
- one composition can still render GIF during benchmarks/debugging
- current text fallback remains available if rendering fails

Disadvantages:

- combined runtime needs Node 24, Chromium, fonts, and browser libraries
- image size grows from a tiny static runtime to likely hundreds of MB
- render CPU/RAM now shares the bot container's cgroup
- local worker protocol and restart supervision must be implemented
- Remotion bundle/browser cold start remains expensive unless reused

Verdict: best match for "current beauty + no renderer API."

### Option D: One-Shot Local Remotion CLI per Command

This also removes HTTP, but it repeats Node startup, bundling, and browser
launch per command. Existing measurements show a large cold/warm gap. Remotion
officially supports reusing an open browser to speed multiple renders.

Verdict: acceptable prototype, poor production architecture.

## Format Decision

### GIF versus H.264 MP4

| Criterion | GIF | Silent H.264 MP4 |
|---|---|---|
| Telegram method | `sendAnimation` | `sendAnimation` |
| Inline/autoplay UX | Yes | Yes |
| Caption/spoiler | Yes | Yes |
| Colors | 256 per frame | Full-color input, compressed video output |
| Shadows/glow | Palette banding/dither risk | Better gradients; lossy artifact risk |
| Temporal compression | Limited | Strong |
| Expected bytes | Larger | Smaller; must benchmark |
| Current Remotion support | Existing | Native `codec: "h264"` |
| Pure-Go production | Possible | Not with standard library |

**Choose H.264 MP4 as the production format.** Keep GIF only as a temporary
benchmark/debug option, not an automatic production fallback. Re-rendering GIF
after an MP4 failure doubles latency and resource use; retain the existing text
winner fallback instead.

Suggested first parity profile:

```js
await renderMedia({
  codec: 'h264',
  composition,
  crf: 18,
  imageFormat: 'png',
  inputProps,
  muted: true,
  outputLocation,
  overwrite: true,
  pixelFormat: 'yuv420p',
  puppeteerInstance: sharedBrowser,
  serveUrl,
});
```

Notes:

- Preserve 512px, 20 FPS, 6s spin, 1s hold, seven turns for the first parity
  release. Shortening is a separate UX decision.
- Start at CRF 18; compare CRF 18, 20, and 22 visually and by bytes.
- PNG source frames preserve browser rendering before H.264 encoding.
- `yuv420p` is the conservative compatibility target; inspect small radial text
  and saturated slice edges for chroma-subsampling artifacts.
- Do not add an audio track.

## Recommended Architecture

```text
┌──────────────────── one Coolify container ────────────────────┐
│                                                               │
│  Go /server                                                   │
│    command handler                                            │
│       │ bounded request                                       │
│       ▼                                                       │
│    render supervisor ── JSON lines over pipes ──► Node worker │
│       ▲                                      │                │
│       │ metadata + temp path                  ▼                │
│       │                         cached Remotion bundle         │
│       │                         shared Chromium instance       │
│       │                         H.264 render, concurrency=1    │
│       │                                      │                │
│       └──────── read + validate + delete ◄── temp MP4          │
│                                                               │
│  Go SendAnimation(MP4 + existing spoiler caption) ─► Telegram │
│                                                               │
│  no renderer HTTP listener; no renderer network call          │
└───────────────────────────────────────────────────────────────┘
```

### Worker Lifecycle

1. Go starts one Node worker lazily or warms it asynchronously after startup.
2. Worker bundles Remotion once and opens one reusable browser.
3. Go serializes render requests through a capacity-one queue.
4. Worker renders to a unique directory under a controlled temp root.
5. Worker returns request ID, path, MIME, dimensions, duration, and byte count.
6. Go verifies path prefix, MP4 signature/metadata, and maximum size.
7. Go reads/sends the file and always removes the temp directory.
8. On timeout/crash, Go kills and restarts the worker, then sends text result.

Use stdout only for machine protocol and stderr for renderer logs. Never send
binary/base64 through JSON lines; base64 adds memory and about 33% transfer
overhead.

### Container Design

Recommended multi-stage build:

```text
stage 1: golang:1.26.5 -> build static /server
stage 2: node:24-bookworm-slim -> pnpm production install + browser download
runtime: node:24-bookworm-slim + Chromium libs + Noto fonts
         copy /server, renderer bundle/source, node_modules, pinned browser
entrypoint: /server (starts local Node worker)
```

Requirements:

- install the same Chromium libraries/fonts as current `wheelofnames` Dockerfile
- download/pin the Remotion browser during image build, never at runtime
- run bot and worker as non-root
- keep port 8080 only for bot health; expose no renderer port
- allow controlled writable temp space; keep application/root filesystem
  read-only where practical
- retain one bot replica and one render at a time

This removes the current distroless security/minimal-size benefit. That is the
principal architecture cost.

## Performance and Quality

### What Is Verified

- Telegram accepts GIF and silent H.264/MPEG-4 AVC through `sendAnimation`, up
  to 50 MB.
- Remotion `renderMedia()` supports `codec: "h264"`, in-memory or file output,
  cancellation, CRF, pixel format, and reusable browser instances.
- Current warm GIF fixture render is 4.1s; cold is 13.9s.
- The current production container lacks all browser/media runtime pieces.

### What Is Not Yet Verified

- H.264 bytes and render latency for this wheel
- production 512px/20 FPS/7s GIF bytes and latency
- Telegram-side appearance after upload/transcoding on Android, iOS, Desktop
- combined container image size and idle/peak memory

A temporary local benchmark was attempted. Dependency installation and browser
preparation succeeded, but Chromium could not launch because the research host
lacks the shared libraries listed in the renderer Dockerfile. No codec numbers
were fabricated from that failed run.

The temporary checkout plus installed dependencies occupied about 600 MB, and
the downloaded headless browser executable alone was about 188 MB. These are
research-host figures, not a final compressed container-image estimate, but
they confirm the runtime increase is material.

### Expected Behavior Requiring Measurement

H.264 should beat GIF for this animation because most pixels remain spatially
and temporally related while only wheel rotation and winner emphasis change.
GIF stores palette-based images and has much weaker temporal compression. The
size win must still be measured on real fixtures.

## Security and Reliability

- Preserve option count/length limits before invoking the worker.
- Never interpolate option text into shell commands; send structured JSON over
  an already-open pipe.
- Use request IDs and reject unsolicited/out-of-order worker responses.
- Enforce render timeout, output byte cap, temp-root path containment, and file
  signature/ffprobe-style metadata checks.
- Keep concurrency at one on the recommended 1-2 vCPU/1-2 GB starting profile.
- Cancel render on Telegram/request context cancellation where practical.
- Restart the worker after crash or repeated render failures.
- Avoid worker stdout logging that can corrupt the IPC stream.
- Pin Node, pnpm lock, Remotion, and browser versions.
- Run without renderer network access if container policy supports it; the
  worker needs local files only.
- Preserve current plain-text winner fallback on any render/send failure.

An out-of-memory kill can still terminate the whole container even though the
renderer is a child process. Resource limits, serial rendering, and production
peak-memory measurement are mandatory.

## Implementation Impact

If selected later, expected scope:

### Repository/runtime

- bring the required `wheelofnames` composition into this repository or a
  version-pinned build input
- add Node renderer package/lock files and local worker entry point
- replace distroless runtime with combined Node/Chromium runtime
- remove wheel API URL/token configuration and deployment instructions

### Go bot

- replace HTTP wheel client with a local render supervisor
- preserve command parsing, local winner selection, spoiler caption, thread ID,
  and text fallback
- change upload filename from `.gif` to `.mp4`
- keep `SendAnimationParams`; no Telegram method change
- validate local worker output and clean temporary files

### Tests

- unit-test supervisor framing, timeouts, crash/restart, output validation, and
  cleanup with a small fake child executable
- retain handler tests for caption, thread ID, and text fallback
- test renderer composition and winner alignment in Node
- add deterministic smoke renders for GIF/H.264 comparison
- add Docker smoke test proving browser launch under the final non-root runtime
- perform Telegram client visual matrix before rollout

### Maintainability Boundary

Extract renderer-neutral request/timing constants, but do not rewrite wheel
geometry in Go. The Remotion composition remains the sole visual source of
truth. This avoids GIF/video visual drift.

## Validation Plan

Before choosing implementation:

1. Build the combined runtime prototype with required browser libraries.
2. Render identical fixtures as GIF and H.264:
   - 2, 8, 16, and 32 options
   - ASCII, Vietnamese, long labels, emoji
   - all themes
   - current 512px/20 FPS/7s profile
3. Record output bytes, cold/warm latency, CPU time, peak RSS, and temp bytes.
4. Compare H.264 CRF 18/20/22 at 512px.
5. Inspect labels, slice edges, shadows, glow, easing, and winner frame.
6. Upload representative files to a development bot and inspect Android, iOS,
   Desktop, slow network, reduced-motion/autoplay settings.
7. Require the selected H.264 profile to be visually equivalent and materially
   smaller than GIF before removing GIF production output.

Suggested acceptance gates:

- no renderer network socket or runtime download
- exact winner and caption behavior
- no obvious text/chroma artifacts at normal Telegram display size
- warm render finishes within existing 30s command timeout
- output remains well below Telegram's 50 MB limit and current 12 MB bot cap
- worker crash always yields text fallback and leaves no temp files
- bot stays responsive during a render

## Resources and References

### Official Documentation

- [Telegram Bot API: `sendAnimation`](https://core.telegram.org/bots/api#sendanimation)
- [Remotion `renderMedia()`](https://www.remotion.dev/docs/renderer/render-media)
- [Remotion server-side rendering](https://www.remotion.dev/docs/ssr)
- [Go `image/gif`](https://pkg.go.dev/image/gif)

### Current Source and Evidence

- [`wheelofnames` GIF renderer](https://github.com/tiennm99/wheelofnames/blob/f13055651e7a18a59b024f7b22266611cc843389/src/render/render-gif.js)
- [`wheelofnames` composition](https://github.com/tiennm99/wheelofnames/blob/f13055651e7a18a59b024f7b22266611cc843389/src/remotion/WheelComposition.jsx)
- [`wheelofnames` container runtime](https://github.com/tiennm99/wheelofnames/blob/f13055651e7a18a59b024f7b22266611cc843389/Dockerfile)
- [`wheelofnames` benchmarks](https://github.com/tiennm99/wheelofnames/blob/f13055651e7a18a59b024f7b22266611cc843389/docs/render-benchmarks.md)

## Next Steps

No implementation now. If approved later:

1. Build a disposable combined-container benchmark first.
2. Measure identical GIF and H.264 outputs.
3. Confirm H.264 Telegram client quality.
4. Only then plan the local persistent-worker integration.

## Unresolved Questions

- Does "all in house" allow a local persistent Node child process, or require
  one Go process only?
- Is the larger Node/Chromium container acceptable on the Coolify host?
- Must the first version retain the current 7-second timing exactly, or use the
  previously researched 2.95-second profile?
