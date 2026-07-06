---
type: research-report
created: 2026-07-06 14:41 UTC
topic: wheel beta gif rendering library options
status: complete
---

# Research Report: Wheel Beta GIF Rendering Library

## Summary

Best fit: use `github.com/tdewolff/canvas` for frame rendering, keep the current stdlib `image/gif` encoder first, and evaluate `gifski` only if GIF color quality remains poor.

Reason: this bot runs server-side as a Go Telegram bot. The rendering stack should stay deterministic, headless, and low-ops. `canvas` gives higher-level vector drawing, affine transforms, text handling, gradients, and raster output without bringing a GUI/game runtime. `gifski` is excellent for final GIF encoding quality, but it adds CLI/C library operational complexity.

Important correction: current code does not use `x/image` to encode GIFs. It uses standard `image/gif` for animation encoding and `golang.org/x/image/font/opentype` for font rasterization. Replacing direct `x/image` usage mostly means replacing our low-level text/glyph-mask drawing path.

## Method

- Scope: Go libraries suitable for rendering animated wheel GIF frames for `/wheelofnamesbeta`.
- Criteria: server deploy friction, text quality, transforms, gradients, animation frame pipeline, dependency size, license, maintenance, integration cost.
- Sources: project code, pkg.go.dev, GitHub READMEs, project homepages.
- Search limit: within `ck:research` 5-call max.

## Current State

Current wheel beta stack:

```text
spin math -> render each frame into image.Paletted -> image/gif.EncodeAll
                         |
                         +-> direct pixel loops for slices, rim, pointer
                         +-> x/image/opentype for title/status/label text
```

Pain points:

- Manual pixel loops make anti-aliasing and gradients awkward.
- Rotated text is custom mask stamping.
- Fixed 14-color palette restricts smooth highlights and anti-aliased text.
- Better animation feel is already mostly math; better visual output needs a higher-level renderer and possibly better quantization.

## Recommendation

### 1. Primary Renderer: `tdewolff/canvas`

Use `github.com/tdewolff/canvas` for drawing each wheel frame into a raster image.

Why:

- It is a Go vector graphics library that outputs raster formats, SVG, PDF, EPS, and more. Its README describes it as a Cairo/node-canvas-style target with path manipulation and text formatting support.
- It supports text drawing through `Context.DrawText`, view transforms through `Context.Rotate`, and raster writers including PNG/GIF via `renderers`.
- The package is current enough for this repo: `go list -m -json github.com/tdewolff/canvas@latest` returned `v0.0.0-20260617131110-529326a1955e` with Go 1.25.
- It avoids GUI/display server concerns.

Trade-offs:

- More complex API than `gg`.
- Not v1/stable-tagged on pkg.go.dev.
- Full package has many imports; use only the subpackages needed.
- It may still use Go image-related packages internally. The value is removing our direct low-level glyph/pixel renderer, not purging every image dependency.

Fit for wheel:

```text
for each frame:
  c := canvas.New(widthMM, heightMM)
  ctx := canvas.NewContext(c)
  draw slices as paths/arcs
  draw labels with text object + rotated view
  draw pointer, rim, status band
  rasterize to RGBA/paletted image
append frame to GIF
```

### 2. Keep `image/gif` Initially

Keep `image/gif.EncodeAll` for first migration. It is already working, tested, and simple.

Change only the frame source first:

```text
old: renderWheelBetaFrame(...) *image.Paletted
new: renderWheelBetaFrameRGBA(...) *image.RGBA
     palettize frame
     gif.EncodeAll(...)
```

If anti-aliasing is added, the fixed 14-color palette will limit quality. Move to a larger 128/256-color palette before introducing external encoders.

### 3. Optional Encoder Upgrade: `gifski`

Use `gifski` only if we specifically need better GIF color quality after renderer migration.

Why:

- `gifski` is designed for high-quality GIF conversion. Its docs highlight pngquant palettes, temporal dithering, lossy LZW, temporal smoothing, and denoising.
- It accepts PNG frames through CLI, and can also be compiled as a C library.

Why not first:

- Adds external binary or C/Rust/FFI complexity.
- More moving parts in Coolify/self-host deploy.
- Current animation is only 320x320 and simple; the standard GIF pipeline may be good enough with a better renderer and palette.

## Comparison

| Option | Role | Pros | Cons | Verdict |
|---|---|---|---|---|
| `tdewolff/canvas` | Vector renderer | Good transforms, paths, text, gradients, raster targets, current Go metadata | Larger API/import surface, no v1 tag | Best primary renderer |
| `fogleman/gg` | Simple 2D RGBA renderer | Very easy API, anchored text, transforms, gradients | Last tagged release 2019; still uses font/image stack; less advanced text/layout | Good fallback for quick prototype |
| `llgcode/draw2d` | Canvas-style vector renderer | Pure Go, affine transforms, text, image/PDF/SVG backends | Older API style, less compelling than `canvas` for text/layout | Acceptable but not first choice |
| `gifski` | GIF encoder | Best output quality, temporal dithering, PNG-frame input | External binary/C library operational cost | Optional phase 2 encoder |
| `raylib-go` | Game/graphics runtime | Mature 2D/game APIs and animation examples | OpenGL/window/runtime requirements, first build can be heavy, poor fit for headless bot | Avoid |
| Cairo/Skia bindings | Native renderer | Excellent rendering quality | C/C++ deps, cgo/build complexity | Avoid unless Go-native options fail |

## Implementation Path

1. Add a renderer seam:

```go
type wheelBetaFrameRenderer interface {
    Render(options []string, winner int, rotation float64, reveal bool, status string, celebrateStep int) image.Image
}
```

2. Keep current renderer as baseline.

3. Add `canvas` renderer behind a small internal function, not public command behavior.

4. Preserve current animation contract:

- same frame count
- same delay array
- same final winner alignment at 3h pointer
- same Telegram `sendAnimation` behavior
- same Vietnamese glyph coverage

5. Add visual-focused tests:

- GIF decodes and has expected frame count/delay.
- Result frame differs from last spin frame.
- Text appears near expected slice label centers.
- Current/winner index still tracks pointer.
- Optional golden-ish image metrics: count non-background pixels, pointer bounds, label bounds.

6. Only after renderer looks better, decide whether encoder upgrade is needed:

- If color banding/text edges remain bad: evaluate 256-color palette and `gifski`.
- If file size too large: tune frame count, palette, or consider MP4 upload if acceptable for Telegram.

## Code Sketch

```go
func renderWheelOfNamesBetaGIF(options []string, winner int) ([]byte, error) {
    frames := make([]*image.Paletted, 0, wheelBetaSpinFrames+wheelBetaHoldFrames)
    delays := make([]int, 0, cap(frames))

    renderer := newWheelBetaCanvasRenderer()
    profile := newWheelBetaSpinProfile(len(options), winner, nil)

    for i := 0; i < wheelBetaSpinFrames; i++ {
        t := float64(i) / float64(wheelBetaSpinFrames-1)
        rgba := renderer.Render(options, winner, profile.rotationAt(t), false, profile.statusAt(t), -1)
        frames = append(frames, palettizeWheelFrame(rgba))
        delays = append(delays, wheelBetaSpinDelay)
    }

    // hold frames unchanged
    return encodeWheelGIF(frames, delays)
}
```

Keep this as an internal refactor; no Telegram command contract change.

## Security And Operations

- Prefer pure Go. Avoid spawning shell commands from message handlers unless strongly justified.
- If using `gifski` CLI later, never pass user-controlled paths or shell strings; write temp frames in a controlled temp dir and execute with `exec.Command` args.
- Watch memory: 50 frames at 320x320 RGBA is about 20 MB before GIF/palette overhead. Fine, but avoid retaining extra PNG buffers if using `gifski`.
- Avoid runtime font discovery. Continue embedding font bytes in the Go package.

## Source Notes

- `tdewolff/canvas` README: vector graphics target with raster output and text formatting; can output PNG/JPG/GIF and other formats. Source: https://github.com/tdewolff/canvas
- `tdewolff/canvas/renderers`: GIF writer accepts `canvas.Resolution`, `canvas.Colorspace`, and `image/gif.*Options`; `Write` supports `.gif`, `.png`, `.webp`, `.svg`, `.pdf`, and more. Source: https://pkg.go.dev/github.com/tdewolff/canvas/renderers
- `tdewolff/canvas` pkg metadata: latest pseudo-version published 2026-06-17, MIT, Go 1.25. Source: https://pkg.go.dev/github.com/tdewolff/canvas
- `fogleman/gg`: creates `image.RGBA` contexts, supports text drawing, anchored text, gradients, transforms, and rotated drawing helpers. Source: https://pkg.go.dev/github.com/fogleman/gg and https://github.com/fogleman/gg
- `draw2d`: supports image/PDF/OpenGL/SVG outputs, text rendering with TrueType fonts, and affine transforms. Source: https://pkg.go.dev/github.com/llgcode/draw2d and https://github.com/llgcode/draw2d
- `gifski`: uses pngquant palettes, temporal dithering, lossy LZW compression, smoothing/denoising; supports CLI and C library integration. Source: https://gif.ski/ and https://github.com/ImageOptim/gifski
- `raylib-go`: game-oriented Go bindings; includes raylib C source and has Linux graphics dependencies or embedded native libraries depending mode. Source: https://github.com/gen2brain/raylib-go
- Cairo: strong renderer but C library/bindings; supports affine transforms and antialiased text, but increases native dependency surface. Source: https://www.cairographics.org/

## Next Steps

1. Prototype `tdewolff/canvas` in a small internal spike rendering one static wheel frame to PNG/GIF.
2. Compare output against current renderer at 320x320 and maybe 640x640 downsampled.
3. If quality is clearly better and build/test cost acceptable, migrate `/wheelofnamesbeta` frame rendering.
4. Only then evaluate `gifski` or broader GIF quantization.

## Unresolved Questions

None.
