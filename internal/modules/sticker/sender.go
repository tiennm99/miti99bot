package sticker

import (
	"errors"

	"github.com/go-telegram/bot/models"
)

// errNoPersonalSender is returned when a message carries no usable personal
// identity. Handlers turn it into senderRefusal.
var errNoPersonalSender = errors.New("sticker: no personal sender")

// senderRefusal explains the fix rather than only denying. Anonymous posting is
// a per-message toggle, so the user can act on this immediately.
const senderRefusal = "Sticker packs need a personal account. Turn off anonymous posting for this message and try again."

// senderID returns the personal Telegram user behind msg.
//
// Every pack is keyed by this value, so it must identify one human. Telegram
// substitutes a single global GroupAnonymousBot user for *every* anonymous
// group-admin message and puts the real origin in SenderChat: without the
// SenderChat check, all anonymous admins across all groups would share one
// pack. Under one-pack-per-user that is worse than a leak — the first anonymous
// admin to run /newpack would own the result and block every other one.
//
// Other modules check only From != nil && From.ID != 0, which is safe for
// paper-trading state but not for durable Telegram-side objects.
func senderID(msg *models.Message) (int64, error) {
	if msg == nil || msg.From == nil || msg.From.ID == 0 {
		return 0, errNoPersonalSender
	}
	if msg.From.IsBot {
		return 0, errNoPersonalSender
	}
	if msg.SenderChat != nil {
		return 0, errNoPersonalSender
	}
	return msg.From.ID, nil
}
