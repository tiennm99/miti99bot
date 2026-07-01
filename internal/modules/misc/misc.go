// Package misc is a small stub module that proves the framework end-to-end:
// /ping (public, exercises KV write), /ping_stats (protected, exercises KV
// read), /the_answer (private easter egg).
package misc

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// lastPingKey is the per-module store key /ping writes and /ping_stats reads.
const lastPingKey = "last_ping"

// defaultTarget is the substituted "investigator" name when /trongtruonghop is
// invoked without an argument.
const defaultTarget = "VNG"

// trongTruongHopTemplate is the disclaimer rendered by /trongtruonghop. Three
// %s slots: target (escaped), sender mention, sender mention.
const trongTruongHopTemplate = "Trong trường hợp nhóm này bị điều tra bởi %s, %s khẳng định không liên quan tới nhóm hoặc những cá nhân khác trong nhóm này. %s không rõ tại sao lại có mặt ở đây vào thời điểm này, có lẽ tài khoản đã được thêm bởi một bên thứ ba."

// lastPing is the value stored at the `last_ping` key: { at: <ms-since-epoch> }.
// int64 ms-epoch (not time.Time → RFC3339) keeps the on-disk shape compact
// and consistent with every other timestamp field in the bot's KV.
type lastPing struct {
	At int64 `json:"at" bson:"at"`
}

// New is the module Factory. Builds a typed store from deps.Store and captures
// it via closure so each command handler has direct access.
func New(deps modules.Deps) modules.Module {
	store := storage.Typed[lastPing](deps.Store)
	return modules.Module{
		Commands: []modules.Command{
			pingCommand(store),
			pingStatsCommand(store),
			theAnswerCommand(),
			trongTruongHopCommand("trongtruonghop"),
			trongTruongHopCommand("tth"),
		},
	}
}

func pingCommand(store storage.DocStore[lastPing]) modules.Command {
	return modules.Command{
		Name:        "ping",
		Visibility:  modules.VisibilityPublic,
		Description: "Health check — replies pong and records last ping",
		Handler: func(ctx context.Context, b *bot.Bot, update *models.Update) error {
			if update.Message == nil {
				return nil
			}
			// Best-effort write — if store is unavailable, still reply.
			payload := lastPing{At: chathelper.NowMillis()}
			if err := store.Put(ctx, lastPingKey, payload); err != nil {
				log.Error("store put failed", "module", "misc", "command", "ping", "key", lastPingKey, "err", err)
			}
			return chathelper.Reply(ctx, b, update.Message, "pong")
		},
	}
}

func pingStatsCommand(store storage.DocStore[lastPing]) modules.Command {
	return modules.Command{
		Name:        "ping_stats",
		Visibility:  modules.VisibilityProtected,
		Description: "Show the timestamp of the last /ping",
		Handler: func(ctx context.Context, b *bot.Bot, update *models.Update) error {
			if update.Message == nil {
				return nil
			}
			text := "last ping: never"
			last, _, err := store.Get(ctx, lastPingKey)
			switch {
			case err == nil && last.At > 0:
				text = fmt.Sprintf("last ping: %s",
					time.UnixMilli(last.At).UTC().Format(time.RFC3339))
			case err != nil && !errors.Is(err, storage.ErrNotFound):
				// User-visible reply mirrors how stock/wordle/loldle handle
				// transient store failures — returning the error here would leave
				// the user with no reply at all.
				log.Error("store get failed", "module", "misc", "command", "ping_stats", "key", lastPingKey, "err", err)
				text = "Could not load stats. Try again later."
			}
			return chathelper.Reply(ctx, b, update.Message, text)
		},
	}
}

// senderMention renders the mention used inside the trongtruonghop template.
// Prefer @username (Telegram resolves it server-side and enforces a safe
// charset). Fall back to a tg://user?id link with the user's display name when
// the account has no username; escape the name because first/last names can
// legitimately contain '<' or '&'.
func senderMention(u *models.User) string {
	if u == nil {
		return "thành viên"
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = "thành viên"
	}
	return fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, u.ID, html.EscapeString(name))
}

func trongTruongHopCommand(name string) modules.Command {
	return modules.Command{
		Name:        name,
		Visibility:  modules.VisibilityPublic,
		Description: "Phát biểu disclaimer cho thành viên hiện tại",
		Handler: func(ctx context.Context, b *bot.Bot, update *models.Update) error {
			if update.Message == nil || update.Message.From == nil {
				return nil
			}
			arg := strings.TrimSpace(chathelper.ArgAfterCommand(update.Message.Text))
			if arg == "" {
				arg = defaultTarget
			}
			mention := senderMention(update.Message.From)
			text := fmt.Sprintf(trongTruongHopTemplate, html.EscapeString(arg), mention, mention)
			return chathelper.ReplyHTML(ctx, b, update.Message, text)
		},
	}
}

func theAnswerCommand() modules.Command {
	return modules.Command{
		Name:        "the_answer",
		Visibility:  modules.VisibilityPrivate,
		Description: "Easter egg — the answer",
		Handler: func(ctx context.Context, b *bot.Bot, update *models.Update) error {
			if update.Message == nil {
				return nil
			}
			return chathelper.Reply(ctx, b, update.Message, "The answer.")
		},
	}
}
