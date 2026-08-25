package sticker

import (
	"context"
	"errors"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
)

// userError carries text meant to be shown to the user verbatim.
//
// The module has two kinds of failure and must never confuse them: a refusal
// the user can act on ("that pack name is too long"), and an internal failure
// that must not reach a reply at all — plan rule 5 exists because a transport
// error's text can embed the bot token. Wrapping the first kind in a distinct
// type makes "is this safe to echo?" a type question instead of a judgement
// call at each call site.
type userError struct{ msg string }

func (e userError) Error() string { return e.msg }

// refuse builds a userError. Its text is replied verbatim, so write it as a
// sentence addressed to the user.
func refuse(msg string) error { return userError{msg: msg} }

// errNoUsername means the bot's own username is unavailable, so no new set name
// can be built. Internal, not user-facing: nothing the caller does fixes it.
var errNoUsername = errors.New("sticker: bot has no username")

// isStickerSetMissing reports whether err positively says the set does not
// exist on Telegram's side.
//
// Classification here is positive-only, and deliberately so: this is the one
// signal that authorises deleting a user's pack record. A network blip, a 429,
// or a context cancelled by SIGTERM must never be read as "the pack is gone" —
// under one pack per user that would destroy the only record of a live pack and
// block /newpack until the phantom cleared.
func isStickerSetMissing(err error) bool {
	return errors.Is(err, bot.ErrorBadRequest) &&
		strings.Contains(err.Error(), "STICKERSET_INVALID")
}

// apiRefusal maps a Telegram API error to user-facing text, or returns ok=false
// when the error has no specific meaning and should be treated as a failure.
//
// Matching is on MTProto code substrings rather than prose. The Bot API server
// rewrites only three of these into English (PACK_SHORT_NAME_OCCUPIED,
// PACK_SHORT_NAME_INVALID, STICKER_EMOJI_INVALID); the rest arrive as
// "Bad Request: <CODE>", and the prose for the three could change without
// notice. Both forms are matched where they differ.
func apiRefusal(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	text := err.Error()
	switch {
	case contains(text, "PACK_SHORT_NAME_OCCUPIED", "already occupied"):
		return "That pack name is taken. Pick a different one.", true
	case contains(text, "PACK_SHORT_NAME_INVALID", "invalid sticker set name"):
		return "Telegram rejected that pack name. Use lowercase letters, digits and single underscores.", true
	case contains(text, "PACK_TITLE_INVALID"):
		return "Telegram rejected that title. Try a shorter, simpler one.", true
	case contains(text, "STICKERSET_INVALID"):
		return "Your pack no longer exists on Telegram. Use /newpack to create a new one.", true
	case contains(text, "STICKERS_TOO_MUCH"):
		return "Your pack is full (120 stickers).", true
	case contains(text, "STICKER_EMOJI_INVALID", "invalid sticker emojis"):
		return "Telegram rejected those emoji. Try different ones.", true
	case contains(text, "too many emoji specified"):
		return "At most 20 emoji per sticker.", true
	case contains(text, "STICKER_PNG_DIMENSIONS", "STICKER_DIMENSIONS_INVALID"):
		return "Telegram rejected that image's dimensions.", true
	}
	return "", false
}

// replyAPIError converts a Telegram API error into a reply. Errors with no
// specific mapping are logged and answered generically — the raw error never
// reaches the user.
func replyAPIError(ctx context.Context, b *bot.Bot, msg *models.Message, op string, err error) error {
	if text, ok := apiRefusal(err); ok {
		return reply(ctx, b, msg, text)
	}
	log.Error(op, "err", err)
	return reply(ctx, b, msg, genericFailure)
}

func contains(text string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

// createRefused reports whether err proves CreateNewStickerSet created nothing.
//
// Deliberately separate from apiRefusal even though today their code lists
// overlap. apiRefusal's job is "map an error to user-facing text"; this one's is
// "prove no set exists", which is what authorises releasing a name reservation
// and dropping a write-ahead intent. Reusing apiRefusal for both would mean the
// next person adding a code there for wording reasons silently converts it into
// a strand-the-slug bug.
//
// Every code here is a request-validation refusal: Telegram rejected the call
// before creating anything.
func createRefused(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(),
		"PACK_SHORT_NAME_OCCUPIED", "already occupied",
		"PACK_SHORT_NAME_INVALID", "invalid sticker set name",
		"PACK_TITLE_INVALID",
		"STICKER_EMOJI_INVALID", "invalid sticker emojis",
		"too many emoji specified",
	)
}
