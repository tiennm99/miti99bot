package util

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

// Source selection happens before any download, so an unsupported or oversized
// file costs nothing.
func TestPhotoFileID(t *testing.T) {
	cases := []struct {
		name    string
		replied *models.Message
		want    string
		ok      bool
	}{
		{
			// Sizes are picked by FileSize rather than array order.
			name: "largest photo size wins",
			replied: &models.Message{Photo: []models.PhotoSize{
				{FileID: "small", FileSize: 100},
				{FileID: "large", FileSize: 5000},
				{FileID: "medium", FileSize: 900},
			}},
			want: "large", ok: true,
		},
		{
			name:    "png document",
			replied: &models.Message{Document: &models.Document{FileID: "doc", MimeType: "image/png"}},
			want:    "doc", ok: true,
		},
		{
			name:    "jpeg document",
			replied: &models.Message{Document: &models.Document{FileID: "doc", MimeType: "image/jpeg"}},
			want:    "doc", ok: true,
		},
		{
			name:    "webp document",
			replied: &models.Message{Document: &models.Document{FileID: "doc", MimeType: "image/webp"}},
			want:    "doc", ok: true,
		},
		{
			name:    "pdf rejected",
			replied: &models.Message{Document: &models.Document{FileID: "doc", MimeType: "application/pdf"}},
			ok:      false,
		},
		{
			name:    "gif document rejected",
			replied: &models.Message{Document: &models.Document{FileID: "doc", MimeType: "image/gif"}},
			ok:      false,
		},
		{
			name:    "oversized photo rejected",
			replied: &models.Message{Photo: []models.PhotoSize{{FileID: "huge", FileSize: maxSourceBytes + 1}}},
			ok:      false,
		},
		{
			name:    "oversized document rejected",
			replied: &models.Message{Document: &models.Document{FileID: "doc", MimeType: "image/png", FileSize: maxSourceBytes + 1}},
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
			got, err := photoFileID(tc.replied)
			if tc.ok {
				if err != nil {
					t.Fatalf("photoFileID: %v", err)
				}
				if got != tc.want {
					t.Errorf("file_id = %q, want %q", got, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("photoFileID = %q, want a refusal", got)
			}
		})
	}
}
