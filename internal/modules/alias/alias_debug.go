package alias

import (
	"strings"

	"github.com/go-telegram/bot/models"
)

// replyShape describes what /alias was handed, for the debug log.
//
// Enable with LOG_LEVEL=debug; silent otherwise. It exists because the failures
// worth debugging here are all about what Telegram *did not* deliver: a reply
// stripped of its content (another bot's message), a caption where text was
// expected, or a kind that falls through capture's switch. Those are invisible
// from the user-facing refusal, which only says the message was unsupported.
//
// Deliberately reports shape, never content. Lengths and field names are
// enough to explain a capture failure, and message text in a log line would
// mean every aliased message ending up in stdout and whatever ships it.
func replyShape(replied *models.Message, entry Alias, ok bool) []any {
	if replied == nil {
		// The case that motivated this: Telegram can deliver /alias with no
		// reply attached at all, which is indistinguishable from the caller
		// forgetting to reply.
		return []any{"reply", "absent"}
	}

	attrs := []any{
		"reply", "present",
		"reply_id", replied.ID,
		"fields", strings.Join(populatedFields(replied), ","),
		"text_len", len(replied.Text),
		"caption_len", len(replied.Caption),
		"entities", len(replied.Entities) + len(replied.CaptionEntities),
		"captured", ok,
	}
	if ok {
		attrs = append(attrs, "kind", entry.Kind)
	}
	if replied.From != nil {
		attrs = append(attrs, "from_id", replied.From.ID, "from_bot", replied.From.IsBot)
	} else {
		// No sender at all — an anonymous channel post or a stripped reply.
		attrs = append(attrs, "from", "absent")
	}
	if replied.SenderChat != nil {
		attrs = append(attrs, "sender_chat", replied.SenderChat.ID)
	}
	return attrs
}

// populatedFields names the content fields the replied message actually has.
//
// Reads the same contentFields table hasContent tests, so the line always
// explains the refusal the caller was given. The two provenance markers are
// appended separately: they say where a message came from, not what it holds,
// and a reply carrying only those is still empty.
func populatedFields(m *models.Message) []string {
	var out []string
	for _, f := range contentFields {
		if f.present(m) {
			out = append(out, f.name)
		}
	}
	if len(out) == 0 {
		// The signature of a reply Telegram delivered but emptied.
		out = append(out, "none")
	}
	if m.ViaBot != nil {
		out = append(out, "via_bot")
	}
	if m.ForwardOrigin != nil {
		out = append(out, "forward_origin")
	}
	return out
}
