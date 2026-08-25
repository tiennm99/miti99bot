package sticker

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// stickerFormatStatic is InputSticker.Format for this module. The whole module
// is static-only; animated and video packs are out of scope.
const stickerFormatStatic = "static"

// notOwnedRefusal answers every "that sticker is not yours to manage" case.
//
// It is deliberately the same sentence whether the caller has no pack at all,
// or replied to a sticker from someone else's pack, or from a set this bot did
// not create. Distinct wording would answer "does this set belong to another
// user of this bot?" for any set the caller can find — a question they have no
// standing to ask. The uniformity is the feature; do not "improve" this into
// three specific messages.
//
// /newpack's slug-occupancy answer is a separate, accepted disclosure: a share
// link is publicly probeable without the bot, so it reveals nothing new.
const notOwnedRefusal = "That sticker is not in your pack. Reply to a sticker from your own pack — /mypack shows it."

// usageReplyToSticker is the shared "you must reply to something" line.
const usageReplyToSticker = "Reply to a sticker with this command."

// stickerSource is a resolved sticker ready to be added to a set: whatever the
// replied message carried, reduced to a file_id.
type stickerSource struct {
	fileID string   // usable directly as InputSticker.Sticker
	emoji  []string // inherited from a replied sticker; at most one element
}

// ownedSticker is an existing sticker in the caller's own pack.
type ownedSticker struct {
	fileID string
	pack   Pack
}

// resolveSource turns the replied message into a sticker source.
//
// It takes ctx, b, and ownerID even though the sticker branch uses none of
// them: the photo branch (which resolves by downloading the image and calling
// UploadStickerFile) lives in this same function, and declaring the full
// signature up front keeps that from churning every call site.
func (s *state) resolveSource(ctx context.Context, b *bot.Bot, ownerID int64, msg *models.Message) (stickerSource, error) {
	replied := msg.ReplyToMessage
	if replied == nil {
		return stickerSource{}, refuse(usageReplyToSticker)
	}

	if st := replied.Sticker; st != nil {
		if err := requireStaticSticker(st); err != nil {
			return stickerSource{}, err
		}
		src := stickerSource{fileID: st.FileID}
		if st.Emoji != "" {
			// models.Sticker.Emoji is a single string, so a replied sticker
			// contributes at most one emoji.
			src.emoji = []string{st.Emoji}
		}
		return src, nil
	}

	return s.resolvePhotoSource(ctx, b, ownerID, replied)
}

// requireStaticSticker enforces the module's static-only scope on a sticker the
// bot is about to copy into a set.
//
// IsAnimated and IsVideo are the obvious half. Type is the half that is easy to
// miss: a mask sticker and a custom-emoji sticker are both static, and both are
// invalid in a regular sticker set, so the boolean pair alone would let them
// through to fail at the API with an opaque error.
func requireStaticSticker(st *models.Sticker) error {
	if st.IsAnimated || st.IsVideo {
		return refuse("This module handles static stickers only — that one is animated or video.")
	}
	if st.Type != "" && st.Type != "regular" {
		return refuse("That is a mask or custom-emoji sticker, which cannot go in a regular pack.")
	}
	return nil
}

// resolveOwned resolves a replied sticker that must already be in the caller's
// own pack. It is the single ownership gate for /delsticker, /editsticker,
// /ordersticker, and /setpackicon.
//
// Costs exactly one store Get and a string comparison — no List, no API call.
func (s *state) resolveOwned(ctx context.Context, msg *models.Message, ownerID int64) (ownedSticker, error) {
	replied := msg.ReplyToMessage
	if replied == nil || replied.Sticker == nil {
		return ownedSticker{}, refuse(usageReplyToSticker)
	}
	st := replied.Sticker
	if st.SetName == "" {
		return ownedSticker{}, refuse("That sticker does not belong to any pack.")
	}
	// Before the store read: a malformed reply costs nothing, and this keeps
	// "rejected before any API call" true by construction.
	if err := requireStaticSticker(st); err != nil {
		return ownedSticker{}, err
	}

	pack, found, err := getPack(ctx, s.store, ownerID)
	if err != nil {
		return ownedSticker{}, err
	}
	// Both branches answer identically — see notOwnedRefusal.
	if !found || pack.Pending {
		return ownedSticker{}, refuse(notOwnedRefusal)
	}
	if !ownsSet(pack, st.SetName) {
		return ownedSticker{}, refuse(notOwnedRefusal)
	}
	return ownedSticker{fileID: st.FileID, pack: pack}, nil
}
