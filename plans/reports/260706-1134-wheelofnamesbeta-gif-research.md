---
type: research
date: 2026-07-06T11:34:00Z
topic: wheelofnamesbeta gif generation and Telegram delivery
skills:
  - ck:research
  - ck:project-organization
---

# Research Report: Wheelofnamesbeta Gif Generation

## Executive Summary

Use Go stdlib `image/gif` for GIF encoding. It supports multi-frame GIFs via
`EncodeAll`, frame delay in 1/100s, and loop control. Add only
`golang.org/x/image/font/basicfont` for self-contained bitmap text if labels or
winner text must appear inside frames. Avoid ffmpeg/cgo/runtime binaries.

Send the generated GIF with Telegram `sendAnimation`, not `sendPhoto`.
Telegram Bot API defines `sendAnimation` for GIF or silent H.264/MPEG-4
animation files, max 50 MB at current docs. Generate in memory, upload with
`models.InputFileUpload`, and set caption to the winner.

Recommended `/wheelofnamesbeta` behavior: parse same comma input as
`/wheelofnames`, choose winner once, render 6s spin plus 1s hold. Keep canvas
small, frame count low, and palette fixed to avoid CPU/file-size spikes.

## Research Methodology

- Sources consulted: 4
- Date: 2026-07-06
- Gemini: disabled by `.ck.json`; used WebSearch
- Key terms: Go animated GIF EncodeAll, golang.org/x/image font basicfont,
  Telegram Bot API sendAnimation, GIF optimization frame count palette

## Key Findings

### 1. Technology Overview

`image/gif` is in Go stdlib and implements GIF decoding/encoding. `EncodeAll`
writes multiple paletted images as one GIF. The `GIF` struct stores successive
frames, delays in 1/100s, and loop count. This maps directly to "6s spin + 1s
hold": e.g. 30 frames at 20/100s each, then 1 final frame at 100/100s.

`golang.org/x/image/font` provides text drawing interfaces. `basicfont` provides
fixed-size font faces. `Face7x13` is self-contained, uses printable ASCII, and
needs no font files.

Telegram Bot API `sendAnimation` sends GIF or silent H.264/MPEG-4 animation
files and returns a sent message. Current limit: 50 MB. It supports normal
`chat_id` and `message_thread_id`, so it works for groups/topics unlike
`sendMessageDraft`.

### 2. Current State And Trends

- GIF is simple but inefficient compared to MP4. For this bot, low-res generated
  vector-ish frames are small enough.
- Go stdlib avoids external binary dependencies and is deploy-safe on Coolify.
- `x/image/basicfont` is enough for readable short text; no custom TTF asset.

### 3. Best Practices

- Render fixed small canvas, e.g. 320x320.
- Use fixed palette and `image.NewPaletted`.
- Keep frame count low: 30 spin frames + 1 hold frame.
- Delay: spin frames 20/100s = 6s total; final hold 100/100s = 1s.
- Set `LoopCount: -1` so the GIF plays once. Telegram clients may still expose
  replay behavior, but file intent is one pass.
- Put exact winner in caption too, because GIF autoplay/download can vary by
  client settings.

### 4. Security Considerations

- No user-provided image decoding; generated pixels only, so image bomb risk low.
- Bound input names and rendered text length to avoid huge memory/caption work.
- Never write temp files; generate in memory.
- Avoid shelling to ffmpeg from command handler.

### 5. Performance Insights

GIF frame memory is width * height * frame count. At 320x320 and 31 frames, raw
paletted data is about 3.2 MB before encoding plus overhead. Acceptable for one
long-polling handler.

Drawing simple wedges/lines/text in Go is CPU-cheap. Avoid antialiasing and
large canvases. 20/100s frame delay keeps total frames modest.

## Comparative Analysis

| Option | Pros | Cons | Verdict |
|---|---|---|---|
| stdlib `image/gif` | No new heavy dependency, pure Go, direct timing control | Basic drawing only | Use |
| `x/image/font/basicfont` | Small official Go subrepo, no font files | ASCII-ish visual, small font | Use for labels |
| `gg` | Easier antialiased drawing/text | Adds third-party dependency | Reject for beta |
| ffmpeg MP4/GIF | Better compression/quality | External binary, shell/runtime complexity | Reject |

## Implementation Recommendations

### Quick Start Guide

1. Add `/wheelofnamesbeta` command in `misc`.
2. Reuse `splitWheelOptions` and `pickWheelOption`.
3. Generate GIF bytes in memory.
4. Send using `b.SendAnimation` with `models.InputFileUpload`.
5. Caption final winner.
6. Add tests that assert `sendAnimation` and encoded GIF timing.

### Code Sketch

```go
buf := bytes.Buffer{}
err := gif.EncodeAll(&buf, &gif.GIF{
    Image: frames,
    Delay: delays, // 30 x 20, then 100
    LoopCount: -1,
})
_, err = b.SendAnimation(ctx, &bot.SendAnimationParams{
    ChatID: msg.Chat.ID,
    MessageThreadID: msg.MessageThreadID,
    Animation: &models.InputFileUpload{
        Filename: "wheelofnamesbeta.gif",
        Data: bytes.NewReader(buf.Bytes()),
    },
    Duration: 7,
    Caption: "Winner: " + winner,
})
```

### Common Pitfalls

- `sendPhoto` can flatten GIF to a static image. Use `sendAnimation`.
- GIF delays are centiseconds, not milliseconds.
- Large canvases or many frames can bloat files fast.
- Telegram client autoplay is user-controlled; caption must contain winner.

## Resources And References

- Go `image/gif` docs: https://pkg.go.dev/image/gif
- Go `golang.org/x/image/font` docs: https://pkg.go.dev/golang.org/x/image/font
- Go `basicfont` docs: https://pkg.go.dev/golang.org/x/image/font/basicfont
- Telegram `sendAnimation`: https://core.telegram.org/bots/api#sendanimation

## Next Steps

1. Implement pure-Go GIF renderer in `internal/modules/misc`.
2. Add `/wheelofnamesbeta` command surfaces and tests.
3. Run focused misc tests, full `go test ./...`, and `go vet ./...`.

## Unresolved Questions

- None.
