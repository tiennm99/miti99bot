package util

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// requireFFmpeg skips when the host has no ffmpeg. The runtime image installs
// it (see the Dockerfile), but a developer machine or a CI runner may not have
// it, and a skipped test is better than a red one for a missing tool.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(ffmpegBinary); err != nil {
		t.Skipf("%s not on PATH", ffmpegBinary)
	}
}

// makeAnimatedGIF builds a small multi-frame GIF in pure Go, so the *input* to
// the transcode needs no external tool. Deliberately not square and smaller
// than 512 on both sides, which is the case that exposes a scale filter that
// only ever shrinks.
func makeAnimatedGIF(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White, color.RGBA{R: 255, A: 255}}
	g := &gif.GIF{}
	for i := 0; i < frames; i++ {
		img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
		for x := 0; x < w; x++ {
			for y := 0; y < h; y++ {
				// Shift the pattern per frame so the encoder has real motion to
				// compress rather than identical frames it can drop.
				img.SetColorIndex(x, y, uint8((x+y+i*7)%len(palette)))
			}
		}
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	return buf.Bytes()
}

// probe reads stream properties back out of a produced file. Returns ok=false
// when ffprobe is unavailable, leaving the caller to skip the detail checks.
func probe(t *testing.T, data []byte, entries string) (string, bool) {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return "", false
	}
	path := filepath.Join(t.TempDir(), "probe.webm")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write probe input: %v", err)
	}
	out, err := exec.Command("ffprobe", "-v", "error",
		"-show_entries", entries, "-of", "default=nw=1", path).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	return string(out), true
}

func TestToStickerWEBM_MeetsTelegramsVideoRules(t *testing.T) {
	requireFFmpeg(t)

	// 100x50 and 12 frames: undersized on both axes, so the long edge must be
	// scaled *up* to exactly 512.
	src := makeAnimatedGIF(t, 100, 50, 12)

	out, err := toStickerWEBM(context.Background(), src)
	if err != nil {
		t.Fatalf("toStickerWEBM: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("toStickerWEBM returned no bytes")
	}
	if len(out) > maxStickerVideoBytes {
		t.Errorf("output is %d bytes, past Telegram's %d-byte limit", len(out), maxStickerVideoBytes)
	}
	// EBML magic — the container really is Matroska/WebM, not whatever the
	// muxer fell back to.
	if !bytes.HasPrefix(out, []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		t.Errorf("output does not start with EBML magic: % x", out[:min(4, len(out))])
	}

	info, ok := probe(t, out, "stream=codec_name,codec_type,width,height:format=duration")
	if !ok {
		t.Log("ffprobe unavailable; skipped stream assertions")
		return
	}

	// One side exactly 512, the other 512 or less.
	if !strings.Contains(info, "width=512") {
		t.Errorf("want the long edge scaled to exactly 512; got %s", info)
	}
	if !strings.Contains(info, "height=256") {
		t.Errorf("want the short edge to keep the 2:1 aspect; got %s", info)
	}
	if !strings.Contains(info, "codec_name=vp9") {
		t.Errorf("want VP9; got %s", info)
	}
	// No audio stream: Telegram refuses a video sticker that carries one.
	if strings.Contains(info, "codec_type=audio") {
		t.Errorf("output carries an audio stream: %s", info)
	}
}

func TestToStickerWEBM_CapsDuration(t *testing.T) {
	requireFFmpeg(t)

	// 10fps GIF with 80 frames is 8 seconds of source; the sticker must be cut
	// to 3.
	src := makeAnimatedGIF(t, 64, 64, 80)

	out, err := toStickerWEBM(context.Background(), src)
	if err != nil {
		t.Fatalf("toStickerWEBM: %v", err)
	}
	info, ok := probe(t, out, "format=duration")
	if !ok {
		t.Skip("ffprobe unavailable")
	}
	dur, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(info), "duration=")), 64)
	if err != nil {
		t.Fatalf("parse duration from %q: %v", info, err)
	}
	// A hair of slack for the final frame's presentation time.
	if dur > maxStickerVideoSeconds+0.2 {
		t.Errorf("duration = %.3fs, want at most %ds", dur, maxStickerVideoSeconds)
	}
}

// Garbage in must produce a refusal the user can read, never an internal error
// carrying ffmpeg's stderr.
func TestToStickerWEBM_RefusesNonVideo(t *testing.T) {
	requireFFmpeg(t)

	_, err := toStickerWEBM(context.Background(), []byte("this is not a video"))
	if err == nil {
		t.Fatal("toStickerWEBM accepted non-video bytes")
	}
	var ue userError
	if !errors.As(err, &ue) {
		t.Fatalf("err = %v (%T), want a userError safe to show the caller", err, err)
	}
	if strings.Contains(ue.msg, "ffmpeg") {
		t.Errorf("refusal mentions the tool: %q", ue.msg)
	}
}
