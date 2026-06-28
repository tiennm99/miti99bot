package modules

import (
	"context"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/metrics"
)

// Auth gates Protected/Private commands by sender Telegram user ID. Public
// commands are always allowed. A zero BotOwnerID + empty AdminUserIDs means
// every Protected/Private command is denied — the safe default for an
// unconfigured deployment.
type Auth struct {
	BotOwnerID   int64          // owner is implicitly an admin; receives Private + Protected
	AdminUserIDs map[int64]bool // additional users allowed to run Protected commands
}

// Permits reports whether the sender of update may run a command of visibility v.
// Denies are silent — callers must NOT reply to denied requests, otherwise the
// existence of a Protected/Private command is leaked to unprivileged users.
func (a Auth) Permits(v Visibility, update *models.Update) bool {
	if v == VisibilityPublic {
		return true
	}
	if update == nil || update.Message == nil || update.Message.From == nil {
		return false
	}
	senderID := update.Message.From.ID
	switch v {
	case VisibilityPrivate:
		return a.BotOwnerID != 0 && senderID == a.BotOwnerID
	case VisibilityProtected:
		if a.BotOwnerID != 0 && senderID == a.BotOwnerID {
			return true
		}
		return a.AdminUserIDs[senderID]
	}
	return false
}

// Install registers every command in the registry with the Telegram bot.
//
// Uses RegisterHandlerMatchFunc with a local matcher rather than the library's
// bot.MatchTypeCommand because the library compares the full bot_command
// entity bytes for equality. In groups, Telegram clients send /cmd@botname,
// so the entity bytes are "cmd@botname" — never equal to the registered
// command name "cmd". The matcher below strips the @suffix before comparing.
//
// auth gates Protected/Private commands; pass a zero-value Auth to deny all
// Protected/Private commands (the right answer for a misconfigured deploy).
func Install(b *bot.Bot, reg *Registry, auth Auth) {
	for name, cmd := range reg.AllCommands {
		cmdCopy := cmd // capture by value for the closure
		nameCopy := name
		b.RegisterHandlerMatchFunc(
			func(update *models.Update) bool {
				return matchCommand(nameCopy, update)
			},
			func(ctx context.Context, b *bot.Bot, update *models.Update) {
				if !auth.Permits(cmdCopy.Visibility, update) {
					return // silent — do not leak existence of gated commands
				}
				metrics.IncCommand(cmdCopy.Name)
				// context.Background is intentional: the hook must outlive the request
				// context so stats writes complete even after the handler returns.
				go func() { //nolint:gosec // G118: goroutine intentionally detached from request context
					hookCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					reg.RunCommandHooks(hookCtx, cmdCopy.Name, update)
				}()
				err := cmdCopy.Handler(ctx, b, update)
				if err != nil {
					metrics.IncError("handler-error")
				}
				logCommand(cmdCopy.Name, update, err)
			},
		)
	}
}

// logCommand emits one structured line per authorized command invocation: what
// was typed (input), who sent it (user id + @username), where (DM vs group, with
// chat id and — for groups — the title), and the outcome. The result is kept
// simple — "ok" via INFO, or the handler error via ERROR — because handler
// return values can be arbitrarily complex; the error plus context is the useful
// part. msg is non-nil here: matchCommand only matches updates with a Message.
func logCommand(name string, update *models.Update, err error) {
	msg := update.Message
	fields := []any{
		"command", name,
		"input", msg.Text,
		"chat_type", string(msg.Chat.Type),
		"chat_id", msg.Chat.ID,
	}
	if msg.Chat.Title != "" {
		fields = append(fields, "chat_title", msg.Chat.Title)
	}
	if from := msg.From; from != nil {
		fields = append(fields, "user_id", from.ID)
		if from.Username != "" {
			fields = append(fields, "username", from.Username)
		}
	}
	if err != nil {
		log.Error("command", append(fields, "err", err)...)
		return
	}
	log.Info("command", fields...)
}

// matchCommand reports whether update is a text message whose bot_command
// entity (after stripping any @botname suffix) equals name. Mirrors the
// library's HandlerTypeMessageText + MatchTypeCommand semantics but tolerates
// the group-form /cmd@botname that the library rejects.
//
// Telegram routes /cmd@otherbot only to otherbot, so an @suffix present in
// the entity addresses *this* bot — no need to verify against our username.
func matchCommand(name string, update *models.Update) bool {
	if update == nil || update.Message == nil {
		return false
	}
	text := update.Message.Text
	for _, e := range update.Message.Entities {
		if e.Type != models.MessageEntityTypeBotCommand {
			continue
		}
		// Bounds check: defensive against malformed entities from a future
		// API revision; the library's match func omits this so a bad entity
		// would panic the goroutine before our recover() in webhook.go.
		end := e.Offset + e.Length
		if e.Offset < 0 || end > len(text) || e.Length < 1 {
			continue
		}
		tok := text[e.Offset+1 : end] // drop leading '/'
		if i := strings.IndexByte(tok, '@'); i >= 0 {
			tok = tok[:i]
		}
		if tok == name {
			return true
		}
	}
	return false
}
