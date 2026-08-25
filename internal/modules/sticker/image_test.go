package sticker

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// makePNG generates a test image in-process, so no binary fixtures are
// committed and every case is readable from the test itself.
func makePNG(t *testing.T, w, h int, alpha uint8) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: alpha})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return buf.Bytes()
}

func decodeSize(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return cfg.Width, cfg.Height
}

// Telegram requires one side to be exactly 512 and the other at most 512.
func TestToStickerPNG_Geometry(t *testing.T) {
	cases := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"landscape", 1024, 512, 512, 256},
		{"portrait", 300, 900, 171, 512},
		{"square", 512, 512, 512, 512},
		{"upscales small input", 64, 32, 512, 256},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := toStickerPNG(makePNG(t, tc.w, tc.h, 255))
			if err != nil {
				t.Fatalf("toStickerPNG: %v", err)
			}
			gotW, gotH := decodeSize(t, out)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("%dx%d -> %dx%d, want %dx%d", tc.w, tc.h, gotW, gotH, tc.wantW, tc.wantH)
			}
			if gotW != stickerEdge && gotH != stickerEdge {
				t.Errorf("neither side is exactly %d: %dx%d", stickerEdge, gotW, gotH)
			}
		})
	}
}

// An extreme aspect ratio must not round the short edge down to zero, which
// would produce an invalid image rather than an error.
//
// 1x4000 rather than the plan's 1x5000: 5000 is past maxDecodeDimension, so
// that case never reaches the scaler at all — it is rejected by the guard
// below. The clamp still needs exercising, and this is the most extreme ratio
// that actually gets there (512/4000 rounds to 0 before clamping).
func TestToStickerPNG_ExtremeAspectRatio(t *testing.T) {
	out, err := toStickerPNG(makePNG(t, 1, 4000, 255))
	if err != nil {
		t.Fatalf("toStickerPNG: %v", err)
	}
	w, h := decodeSize(t, out)
	if w < 1 || h < 1 {
		t.Fatalf("got a zero-dimension image: %dx%d", w, h)
	}
	if h != stickerEdge {
		t.Errorf("long edge = %d, want %d", h, stickerEdge)
	}
}

// The other half: past the cap, the guard refuses rather than the scaler
// coping. The bound exists to cap allocation, so it must win over any
// aspect-ratio handling.
func TestToStickerPNG_ExtremeAspectPastCapIsRejected(t *testing.T) {
	if _, err := toStickerPNG(makePNG(t, 1, maxDecodeDimension+904, 255)); err == nil {
		t.Fatal("an image past the dimension cap was accepted")
	}
}

// Stickers are cut-outs; losing alpha in the resize would put a black box
// behind every one of them.
func TestToStickerPNG_PreservesAlpha(t *testing.T) {
	out, err := toStickerPNG(makePNG(t, 256, 256, 0))
	if err != nil {
		t.Fatalf("toStickerPNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, _, _, a := img.At(128, 128).RGBA(); a != 0 {
		t.Errorf("alpha = %d at a fully transparent pixel, want 0", a)
	}
}

func TestToThumbnailPNG_IsExactly100Square(t *testing.T) {
	// A non-square input still has to come out exactly 100x100, padded.
	out, err := toThumbnailPNG(makePNG(t, 800, 400, 255))
	if err != nil {
		t.Fatalf("toThumbnailPNG: %v", err)
	}
	w, h := decodeSize(t, out)
	if w != thumbnailEdge || h != thumbnailEdge {
		t.Errorf("thumbnail = %dx%d, want %dx%d", w, h, thumbnailEdge, thumbnailEdge)
	}
}

// The dimension guard runs on the header, before any pixel buffer exists — it
// is what bounds peak allocation for attacker-supplied images.
func TestDecodeBounded_RejectsOversizedDimensions(t *testing.T) {
	oversized := makePNG(t, maxDecodeDimension+1, 4, 255)
	if _, err := toStickerPNG(oversized); err == nil {
		t.Fatal("toStickerPNG accepted an image past the dimension cap")
	}
}

func TestDecodeBounded_RejectsNonImage(t *testing.T) {
	if _, err := toStickerPNG([]byte("this is not an image")); err == nil {
		t.Fatal("toStickerPNG accepted non-image bytes")
	}
}

func TestScaleToLongEdge(t *testing.T) {
	cases := []struct {
		w, h, edge   int
		wantW, wantH int
	}{
		{1000, 500, 512, 512, 256},
		{500, 1000, 512, 256, 512},
		{100, 100, 512, 512, 512},
		{5000, 1, 512, 512, 1}, // clamped, never 0
	}
	for _, tc := range cases {
		gotW, gotH := scaleToLongEdge(tc.w, tc.h, tc.edge)
		if gotW != tc.wantW || gotH != tc.wantH {
			t.Errorf("scaleToLongEdge(%d, %d, %d) = (%d, %d), want (%d, %d)",
				tc.w, tc.h, tc.edge, gotW, gotH, tc.wantW, tc.wantH)
		}
	}
}
