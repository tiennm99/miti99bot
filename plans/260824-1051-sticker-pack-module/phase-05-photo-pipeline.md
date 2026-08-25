---
phase: 5
title: "Phase 5: Photo pipeline and pack icon"
status: todo
priority: P1
effort: "8h"
dependencies: [1, 2, 3, 4]
---

# Phase 5: Photo pipeline and pack icon

## Overview

Turn a replied-to photo (or image document) into a valid static sticker, and add
`/setpackicon`, which needs the same resizing machinery at a different output size. This is
the only phase doing network I/O and CPU work, and the only one handling the bot token.

## Requirements

- Functional: a replied photo or image document becomes a sticker with a 512px long edge and
  a preserved aspect ratio.
- Functional: `/setpackicon` sets a pack's thumbnail from a sticker already in that pack.
- Non-functional: bounded in bytes and time — it blocks every other user while it runs (C1).
- Non-functional (**security**): the download URL embeds the bot token and must never reach a
  log, a reply, or a returned error.

## Architecture

### Bounds

Plan rule 1 already puts `handlerTimeout = 10s` on every handler, which is the outer bound.
This phase adds:

| Bound | Value | Why |
|---|---|---|
| Source file size | reject above **2 MB** before downloading | Telegram-compressed `photo` sizes are typically well under 500 KB |
| Decoded dimensions | reject above 4096×4096 via `DecodeConfig` | Bounds peak allocation before any pixel buffer exists |
| HTTP client | explicit per-request timeout, not `http.DefaultClient` | The library's shared client is 60s (`bot.go:17-18`) — too long to inherit |

Worst case is a ~10s bot-wide stall (C1). Bounded and observable, not zero.

### Telegram's static-sticker format

PNG or WEBP; **one side exactly 512px**, the other ≤512px. Pack thumbnails differ: PNG or
WEBP, exactly **100×100**, ≤128 KB.

There is no documented file-size limit for static stickers — the widely-repeated 512 KB
figure appears in no current official page (plan R4). It is a client-side ceiling only and
must not be described as spec in code comments or user-facing text.

### Source selection

From `msg.ReplyToMessage`:

- `Photo []PhotoSize` — pick the largest by `FileSize`; do not rely on Telegram's ordering.
- `Document` — accept only `MimeType` of `image/png`, `image/jpeg`, `image/webp`. Reject
  anything else *before* downloading.

Reject when `FileSize > 2<<20`.

### Download — and the token-leak trap

`GetFile{FileID}` → `b.FileDownloadLink(f)` (`bot.go:180-182`) → `http.Get`.

`FileDownloadLink` returns `https://api.telegram.org/file/bot<TOKEN>/<path>`. **Every
transport failure from `http.Client.Do` returns a `*url.Error` whose `Error()` embeds the
full URL**, and `internal/modules/dispatcher.go:136-138` logs a handler's returned error
verbatim. A timeout mid-transfer — trivially reachable — would therefore print the bot token
to stdout, the Coolify log store, and any log shipper.

The earlier draft's mitigation ("log the `file_id` instead") covered only deliberate logging
and missed this path entirely; its success criterion would have passed while the leak shipped.

Correct handling, per plan rule 5 — **no error from this package may escape raw**:

```go
var errDownloadFailed = errors.New("sticker: download failed")

// ...
if err != nil {
    log.Error("sticker download", "file_id", fileID, "reason", classify(err))
    return nil, fmt.Errorf("file_id=%s: %w", fileID, errDownloadFailed)
}
```

The original error is discarded, never wrapped — wrapping would keep the URL reachable
through `errors.Unwrap` and `%v`. `classify(err)` maps to a coarse label (`timeout`,
`transport`, `status`) that cannot contain a URL.

Other rules:

- `io.LimitReader(body, 2<<20)`; never trust `Content-Length`.
- Read fully into memory — bounded at 2 MB, so no temp files.

### Decode / resize / encode

`toStickerPNG(src []byte) ([]byte, error)`:

1. `image.DecodeConfig` first — reject above 4096×4096 before allocating pixels.
2. `image.Decode` with `image/jpeg`, `image/png`, `image/gif` registered, plus
   `golang.org/x/image/webp` (read-only decoder, same module).
3. Scale so the long edge is exactly 512 and the short edge is `round(short*512/long)`,
   clamped to ≥1. A square input yields 512×512.
4. `draw.CatmullRom.Scale` into a fresh `*image.NRGBA` — preserves alpha.
5. `png.Encode`. Above the 512 KB client-side ceiling, retry with
   `png.Encoder{CompressionLevel: png.BestCompression}`; if still over, step the long edge
   down (448, 384, 320). Give up after 320.

`toThumbnailPNG(src []byte) ([]byte, error)` runs the same pipeline to exactly 100×100,
padding the short edge with transparency to preserve aspect ratio.

Both are pure functions over `[]byte` so they test without a network.

### Upload

`UploadStickerFile{UserID: ownerID, Sticker: &models.InputFileUpload{Filename: "sticker.png", Data: bytes.NewReader(png)}, StickerFormat: "static"}`
→ use the returned `File.FileID` as `InputSticker.Sticker`.

Two steps, not stylistic: plan C6 shows the form builder honours `attach://` only for
`[]models.InputSticker`, so the single `InputSticker` in `AddStickerToSetParams` cannot carry
raw bytes; `*models.InputFileUpload` *is* handled (`build_request_form.go:87`).
`uploadStickerFile` still takes `sticker_format` even though `createNewStickerSet` lost its
top-level equivalent in Bot API 7.2.

The returned `file_id` is consumed immediately, so its undocumented validity window never
matters. Do not restructure into upload-now-use-later.

### `/setpackicon`

No arguments; reply to a sticker in the caller's pack.

1. `resolveOwned` (Phase 4).
2. `GetFile` + download that sticker's image, then `toThumbnailPNG`.
3. `SetStickerSetThumbnail{Name: pack.Name, UserID: ownerID, Thumbnail: &models.InputFileUpload{...}, Format: "static"}`.

The API does accept a `file_id` string for `thumbnail` — the only documented restriction bars
HTTP URLs for animated/video. The reason to resize is the documented 100×100 requirement,
which a 512px sticker's `file_id` does not meet. Confirm in Phase 6's smoke test; if a raw
`file_id` is accepted and auto-resized, this collapses to one call.

## Related Code Files

- Modify: `go.mod`, `go.sum` — add `golang.org/x/image`
- Create: `internal/modules/sticker/download.go`, `image.go`, `setpackicon.go`,
  `download_test.go`, `image_test.go`
- Modify: `internal/modules/sticker/resolve.go` (photo branch), `sticker_handlers.go`
  (`/addsticker` photo path), `pack_handlers.go` (`/newpack` photo path), `sticker.go`
- Reference: `go-telegram/bot@v1.20.0` `bot.go:180-182`, `build_request_form.go:87,105`
- Reference: `internal/modules/dispatcher.go:136-138` (the log path the sentinel protects)

## Implementation Steps

1. `go get golang.org/x/image`; confirm a direct require and a clean `go mod tidy`.
2. `download.go` — bounded fetch, own client timeout, sentinel error conversion.
3. `image.go` — `toStickerPNG`, `toThumbnailPNG`.
4. Wire the photo branch into `resolveSource`, then `/addsticker` and `/newpack`.
5. `setpackicon.go` + registration.
6. Tests per the Todo list.

## Todo

- [ ] Add `golang.org/x/image`; verify `go mod tidy` produces no diff
- [ ] `download.go` with 2 MB `LimitReader`, own client timeout, sentinel conversion
- [ ] `classify(err)` returning a coarse label that cannot contain a URL
- [ ] `toStickerPNG` with DecodeConfig guard, CatmullRom scale, PNG size ladder
- [ ] `toThumbnailPNG` at exactly 100×100 with transparent padding
- [ ] Photo/document source selection with mime allowlist and 2 MB pre-check
- [ ] Wire photo branch into `/addsticker` and `/newpack`
- [ ] `/setpackicon` handler and registration
- [ ] `image_test.go` with in-test generated fixtures (no committed binaries)
- [ ] `download_test.go` asserting no token or URL in any returned error

## Success Criteria

- [ ] 1024×512 → 512×256; 300×900 → 171×512; 512×512 → 512×512
- [ ] 1×5000 extreme aspect: short edge clamped to ≥1, no panic, no zero-dimension image
- [ ] Alpha channel preserved through the resize
- [ ] Source above 2 MB rejected with zero HTTP requests made
- [ ] Decoded dimensions above 4096×4096 rejected before pixel allocation
- [ ] Unsupported document mime rejected before download
- [ ] `toThumbnailPNG` output is exactly 100×100
- [ ] **A forced transport failure against an `httptest` server yields an error whose text contains neither `"bot"` nor the URL** — asserted, not assumed
- [ ] `go mod tidy && git diff --exit-code go.mod go.sum` clean

## Risk Assessment

**R1 — bot-wide stall.** Under C1 every photo request blocks all users for up to
`handlerTimeout`. Tight bounds cap the damage but do not remove it; a user in a loop can keep
the bot substantially stalled.

- Signal: reply latency for unrelated commands spikes with image traffic.
- Response, in order: (a) lower `handlerTimeout` and the 2 MB cap; (b) offload the pipeline
  to a detached goroutine that acks immediately and replies on completion, mirroring
  `dispatcher.go:80-84`; (c) if neither suffices, revisit the Public visibility decision with
  the user — their call, not a unilateral change.
- **If (b) is ever taken, C1's single-in-flight guarantee disappears**, and two things become
  mandatory together: a package-level semaphore around image decoding, and a real look at the
  `/newpack` quota check, which becomes genuinely racy rather than merely lock-protected. The
  two are linked deliberately so neither is done without the other.

**Untrusted image decoding.** Bounded by size and dimension checks; Go's decoders are
memory-safe. Peak allocation is ~64 MB per conversion at the 4096² cap, and C1 guarantees one
at a time. Phase 1's panic barrier is the backstop for a decoder panic — but it is a backstop,
not a licence to skip the dimension guard.

**New dependency.** `golang.org/x/image` is the only one in the plan and the repo's first
*direct* `golang.org/x/*` requirement. Confined to `image.go`. If resampling disappoints,
swapping `CatmullRom` for `ApproxBiLinear` is one line (plan R9).
