package sticker

import (
	"bytes"
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleSetPackIcon sets the pack thumbnail from a sticker already in the pack.
//
// The sticker's own file_id cannot simply be handed to Telegram: a pack
// thumbnail must be exactly 100x100, which a 512px sticker is not. So the image
// is fetched, resized, and uploaded as a new file.
func (s *state) handleSetPackIcon(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	msg := update.Message
	ownerID, err := senderID(msg)
	if err != nil {
		return reply(ctx, b, msg, senderRefusal)
	}

	owned, err := s.resolveOwned(ctx, msg, ownerID)
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_setpackicon_resolve", err)
	}

	// Reserve the reply tail before the slow leg. See mediaContext.
	mediaCtx, cancelMedia := mediaContext(ctx)
	defer cancelMedia()

	raw, err := downloadFile(mediaCtx, b, owned.fileID)
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_setpackicon_download", err)
	}
	thumb, err := toThumbnailPNG(raw)
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_setpackicon_resize", err)
	}

	if _, err := b.SetStickerSetThumbnail(ctx, &bot.SetStickerSetThumbnailParams{
		Name:      owned.pack.Name,
		UserID:    ownerID,
		Thumbnail: &models.InputFileUpload{Filename: "thumb.png", Data: bytes.NewReader(thumb)},
		Format:    stickerFormatStatic,
	}); err != nil {
		if isStickerSetMissing(err) {
			s.dropPackRecord(ctx, ownerID)
		}
		return replyAPIError(ctx, b, msg, "sticker_setpackicon", err)
	}
	return reply(ctx, b, msg, "Pack icon updated.")
}
