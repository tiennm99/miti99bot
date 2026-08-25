package sticker

import (
	"bytes"
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// imageDocumentMimes is the allowlist for a replied document. Anything else is
// rejected before a single byte is downloaded.
var imageDocumentMimes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
}

// resolvePhotoSource turns a replied photo or image document into an uploaded
// sticker file.
//
// Raw bytes cannot ride along on AddStickerToSet: the form builder honours
// attach:// only for []models.InputSticker, and the single InputSticker in
// AddStickerToSetParams falls through to a default that drops the attachment
// silently. So the image is uploaded first and the returned file_id is used.
func (s *state) resolvePhotoSource(ctx context.Context, b *bot.Bot, ownerID int64, replied *models.Message) (stickerSource, error) {
	fileID, err := photoFileID(replied)
	if err != nil {
		return stickerSource{}, err
	}

	// Reserve the caller's reply tail: everything from here to the upload is
	// the slow leg. See mediaContext.
	mediaCtx, cancelMedia := mediaContext(ctx)
	defer cancelMedia()

	raw, err := downloadFile(mediaCtx, b, fileID)
	if err != nil {
		return stickerSource{}, err
	}
	png, err := toStickerPNG(raw)
	if err != nil {
		return stickerSource{}, err
	}

	uploaded, err := b.UploadStickerFile(mediaCtx, &bot.UploadStickerFileParams{
		UserID:        ownerID,
		Sticker:       &models.InputFileUpload{Filename: "sticker.png", Data: bytes.NewReader(png)},
		StickerFormat: stickerFormatStatic,
	})
	if err != nil {
		return stickerSource{}, err
	}
	// Consumed immediately, so the file_id's undocumented validity window never
	// matters. Do not restructure this into upload-now-use-later.
	return stickerSource{fileID: uploaded.FileID}, nil
}

// photoFileID picks the file to convert from a replied message.
func photoFileID(replied *models.Message) (string, error) {
	if len(replied.Photo) > 0 {
		// Pick the largest by size rather than trusting the array's order.
		best := replied.Photo[0]
		for _, size := range replied.Photo[1:] {
			if size.FileSize > best.FileSize {
				best = size
			}
		}
		if best.FileSize > maxSourceBytes {
			return "", refuse("That image is too large — keep it under 2 MB.")
		}
		return best.FileID, nil
	}

	if doc := replied.Document; doc != nil {
		if !imageDocumentMimes[doc.MimeType] {
			return "", refuse("That file is not a supported image. Send a PNG, JPEG or WEBP.")
		}
		if doc.FileSize > maxSourceBytes {
			return "", refuse("That image is too large — keep it under 2 MB.")
		}
		return doc.FileID, nil
	}

	return "", refuse("Reply to a sticker, photo, or image file with this command.")
}
