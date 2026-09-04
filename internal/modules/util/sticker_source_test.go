package util

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

// hasMovingSource picks which of the two conversion paths a reply takes, from
// the message alone. Getting it wrong sends a video at the image resampler, or
// an image at ffmpeg.
func TestHasMovingSource(t *testing.T) {
	cases := []struct {
		name    string
		replied *models.Message
		want    bool
	}{
		{name: "nil reply", replied: nil, want: false},
		{name: "animation", replied: &models.Message{Animation: &models.Animation{FileID: "a"}}, want: true},
		{name: "video", replied: &models.Message{Video: &models.Video{FileID: "v"}}, want: true},
		{name: "video note", replied: &models.Message{VideoNote: &models.VideoNote{FileID: "n"}}, want: true},
		{name: "gif document", replied: &models.Message{Document: &models.Document{MimeType: "image/gif"}}, want: true},
		{name: "mp4 document", replied: &models.Message{Document: &models.Document{MimeType: "video/mp4"}}, want: true},
		{name: "webm document", replied: &models.Message{Document: &models.Document{MimeType: "video/webm"}}, want: true},
		{name: "mov document", replied: &models.Message{Document: &models.Document{MimeType: "video/quicktime"}}, want: true},
		{name: "png document", replied: &models.Message{Document: &models.Document{MimeType: "image/png"}}, want: false},
		{name: "photo", replied: &models.Message{Photo: []models.PhotoSize{{FileID: "p"}}}, want: false},
		{name: "sticker", replied: &models.Message{Sticker: &models.Sticker{FileID: "s"}}, want: false},
		{name: "pdf document", replied: &models.Message{Document: &models.Document{MimeType: "application/pdf"}}, want: false},
		{name: "plain text", replied: &models.Message{Text: "hello"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasMovingSource(tc.replied); got != tc.want {
				t.Errorf("hasMovingSource = %v, want %v", got, tc.want)
			}
		})
	}
}

// Source selection happens before any download, so an oversized clip costs
// nothing.
func TestVideoFileID(t *testing.T) {
	cases := []struct {
		name    string
		replied *models.Message
		want    string
		ok      bool
	}{
		{
			name:    "animation",
			replied: &models.Message{Animation: &models.Animation{FileID: "anim", FileSize: 1 << 20}},
			want:    "anim", ok: true,
		},
		{
			name:    "video",
			replied: &models.Message{Video: &models.Video{FileID: "vid", FileSize: 1 << 20}},
			want:    "vid", ok: true,
		},
		{
			name:    "video note",
			replied: &models.Message{VideoNote: &models.VideoNote{FileID: "note", FileSize: 1 << 20}},
			want:    "note", ok: true,
		},
		{
			name:    "document",
			replied: &models.Message{Document: &models.Document{FileID: "doc", MimeType: "video/mp4"}},
			want:    "doc", ok: true,
		},
		{
			// Animation is checked before Video: Telegram attaches both to a
			// GIF-style message, and the animation is the one to convert.
			name: "animation wins over video",
			replied: &models.Message{
				Animation: &models.Animation{FileID: "anim"},
				Video:     &models.Video{FileID: "vid"},
			},
			want: "anim", ok: true,
		},
		{
			name:    "oversized animation refused",
			replied: &models.Message{Animation: &models.Animation{FileID: "anim", FileSize: maxVideoSourceBytes + 1}},
			ok:      false,
		},
		{
			name:    "oversized video refused",
			replied: &models.Message{Video: &models.Video{FileID: "vid", FileSize: maxVideoSourceBytes + 1}},
			ok:      false,
		},
		{
			name:    "oversized document refused",
			replied: &models.Message{Document: &models.Document{FileID: "doc", MimeType: "video/mp4", FileSize: maxVideoSourceBytes + 1}},
			ok:      false,
		},
		{
			name:    "nothing usable",
			replied: &models.Message{Text: "hello"},
			ok:      false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := videoFileID(tc.replied)
			if tc.ok {
				if err != nil {
					t.Fatalf("videoFileID: %v", err)
				}
				if got != tc.want {
					t.Errorf("file_id = %q, want %q", got, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("videoFileID = %q, want a refusal", got)
			}
		})
	}
}

// The video cap is the one on the source, and it is an order of magnitude above
// the image cap on purpose — the 256 KB sticker limit applies to the transcode
// output, not the input.
func TestVideoSourceCapExceedsImageCap(t *testing.T) {
	if maxVideoSourceBytes <= maxSourceBytes {
		t.Errorf("maxVideoSourceBytes (%d) must exceed maxSourceBytes (%d)", maxVideoSourceBytes, maxSourceBytes)
	}
}
