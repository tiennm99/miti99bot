package sticker

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
)

// maxStickersPerPack is Telegram's documented ceiling for a regular set. It is
// not enforced locally — the server is the authority and a local copy would go
// stale — but it is quoted back to the user when the server refuses.
const maxStickersPerPack = 120

// handleAddSticker adds the replied sticker (or, from Phase 5, photo) to the
// caller's pack.
func (s *state) handleAddSticker(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	msg := update.Message
	ownerID, err := senderID(msg)
	if err != nil {
		return reply(ctx, b, msg, senderRefusal)
	}

	// Every argument is an emoji: with no pack token to disambiguate, a stray
	// word fails here rather than being silently read as something else.
	emoji, err := parseEmoji(commandArgs(msg))
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_addsticker_emoji", err)
	}

	pack, found, err := getPack(ctx, s.store, ownerID)
	if err != nil {
		log.Error("sticker_addsticker_load", "err", err)
		return reply(ctx, b, msg, genericFailure)
	}
	// Unlike resolveOwned's deliberately uniform refusal, this one is specific:
	// it answers only "do *you* have a pack", about the caller's own state, and
	// so discloses nothing about anyone else.
	if !found {
		return reply(ctx, b, msg, noPackYet)
	}
	if pack.Pending {
		return reply(ctx, b, msg, noPackYet+pendingMarker)
	}

	source, err := s.resolveSource(ctx, b, ownerID, msg)
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_addsticker_source", err)
	}

	// Precedence: explicit args, then the replied sticker's own emoji, then the
	// default. Telegram requires at least one.
	if len(emoji) == 0 {
		emoji = source.emoji
	}
	if len(emoji) == 0 {
		emoji = []string{defaultEmoji}
	}

	defer s.lockUser(ownerID)()

	_, err = b.AddStickerToSet(ctx, &bot.AddStickerToSetParams{
		UserID: ownerID, // always the caller: a non-owner never reaches this call
		Name:   pack.Name,
		Sticker: models.InputSticker{
			Sticker:   source.fileID,
			Format:    stickerFormatStatic,
			EmojiList: emoji,
		},
	})
	if err != nil {
		if isStickerSetMissing(err) {
			s.dropPackRecord(ctx, ownerID)
		}
		return replyAPIError(ctx, b, msg, "sticker_addsticker", err)
	}

	updated, err := s.adjustCount(ctx, ownerID, +1)
	if err != nil {
		// The sticker is already in the set; only our count is stale.
		log.Error("sticker_addsticker_commit", "err", err)
		updated = pack
		updated.Count++
	}
	return reply(ctx, b, msg, fmt.Sprintf("Added to %s (%d stickers).\n%s",
		updated.Title, updated.Count, shareLink(updated.Name)))
}

// handleDelSticker removes the replied sticker from the caller's pack.
//
// It deliberately does *not* probe afterwards to see whether the set survived.
// Whether removing the last sticker also destroys the set is undocumented, and
// an earlier design that probed would have deleted the pack record whenever the
// probe merely failed — so a 429, a DNS blip, or a SIGTERM during a routine
// delete would erase the only record of a live pack. Deleting the record needs
// a positive signal, and the next command's STICKERSET_INVALID is one.
func (s *state) handleDelSticker(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	msg := update.Message
	ownerID, err := senderID(msg)
	if err != nil {
		return reply(ctx, b, msg, senderRefusal)
	}

	owned, err := s.resolveOwned(ctx, msg, ownerID)
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_delsticker_resolve", err)
	}

	// Same read-modify-write on Count as /addsticker, so the same lock.
	defer s.lockUser(ownerID)()

	if _, err := b.DeleteStickerFromSet(ctx, &bot.DeleteStickerFromSetParams{Sticker: owned.fileID}); err != nil {
		if isStickerSetMissing(err) {
			s.dropPackRecord(ctx, ownerID)
		}
		return replyAPIError(ctx, b, msg, "sticker_delsticker", err)
	}

	pack, err := s.adjustCount(ctx, ownerID, -1)
	if err != nil {
		// The sticker is already gone from the set; only our count is stale.
		log.Error("sticker_delsticker_commit", "err", err)
		pack = owned.pack
		if pack.Count > 0 {
			pack.Count--
		}
	}

	if pack.Count == 0 {
		// Telegram may have removed the now-empty set. /mypack makes no API
		// calls so it cannot notice, and /newpack stays blocked while a record
		// exists — so name the command that clears it.
		return reply(ctx, b, msg, "Removed. Your pack is now empty, and Telegram may have deleted it. "+
			"If /addsticker says the pack is gone, use /delpack to clear it and /newpack to start again.")
	}
	return reply(ctx, b, msg, fmt.Sprintf("Removed. %s now has %d sticker(s).", pack.Title, pack.Count))
}

// handleEditSticker replaces the emoji of a sticker in the caller's pack.
func (s *state) handleEditSticker(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	msg := update.Message
	ownerID, err := senderID(msg)
	if err != nil {
		return reply(ctx, b, msg, senderRefusal)
	}

	emoji, err := parseEmoji(commandArgs(msg))
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_editsticker_emoji", err)
	}
	// An empty emoji_list is invalid, so this cannot fall back to a default the
	// way /addsticker does: the user has to say what they want.
	if len(emoji) == 0 {
		return reply(ctx, b, msg, "Usage: /editsticker <emoji...>\nReply to a sticker in your pack with at least one emoji.")
	}

	owned, err := s.resolveOwned(ctx, msg, ownerID)
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_editsticker_resolve", err)
	}

	if _, err := b.SetStickerEmojiList(ctx, &bot.SetStickerEmojiListParams{
		Sticker:   owned.fileID,
		EmojiList: emoji,
	}); err != nil {
		if isStickerSetMissing(err) {
			s.dropPackRecord(ctx, ownerID)
		}
		return replyAPIError(ctx, b, msg, "sticker_editsticker", err)
	}
	return reply(ctx, b, msg, "Emoji updated.")
}

// handleOrderSticker moves a sticker to a new position in the caller's pack.
func (s *state) handleOrderSticker(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	msg := update.Message
	ownerID, err := senderID(msg)
	if err != nil {
		return reply(ctx, b, msg, senderRefusal)
	}

	args := commandArgs(msg)
	if len(args) != 1 {
		return reply(ctx, b, msg, "Usage: /ordersticker <position>\nReply to a sticker in your pack. Positions start at 0.")
	}
	pos, err := strconv.Atoi(args[0])
	if err != nil || pos < 0 {
		// Only the lower bound is checked locally. The upper bound is the set's
		// current size, which Telegram knows and a local copy would not.
		return reply(ctx, b, msg, "Position must be a whole number, 0 or greater.")
	}

	owned, err := s.resolveOwned(ctx, msg, ownerID)
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_ordersticker_resolve", err)
	}

	if _, err := b.SetStickerPositionInSet(ctx, &bot.SetStickerPositionInSetParams{
		Sticker:  owned.fileID,
		Position: pos,
	}); err != nil {
		if isStickerSetMissing(err) {
			s.dropPackRecord(ctx, ownerID)
		}
		return replyAPIError(ctx, b, msg, "sticker_ordersticker", err)
	}
	return reply(ctx, b, msg, fmt.Sprintf("Moved to position %d.", pos))
}
