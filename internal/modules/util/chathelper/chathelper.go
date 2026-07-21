// Package chathelper consolidates per-module Telegram helpers (SubjectFor,
// ArgAfterCommand, NowMillis, Reply, ReplyHTML, WinRate) that would
// otherwise be duplicated across every module. Single source here; modules
// import.
package chathelper

import (
	"context"
	"html"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// MonospaceTable renders an HTML-safe, left-aligned table for Telegram's
// <pre> mode. Callers should send the result with ReplyHTML.
func MonospaceTable(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for index, header := range headers {
		widths[index] = len([]rune(header))
	}
	for _, row := range rows {
		for index := range headers {
			if index < len(row) && len([]rune(row[index])) > widths[index] {
				widths[index] = len([]rune(row[index]))
			}
		}
	}
	var lines []string
	appendRow := func(row []string) {
		cells := make([]string, len(headers))
		for index := range headers {
			if index < len(row) {
				cells[index] = row[index]
			}
			if index < len(headers)-1 {
				cells[index] += strings.Repeat(" ", widths[index]-len([]rune(cells[index])))
			}
		}
		lines = append(lines, strings.Join(cells, "  "))
	}
	appendRow(headers)
	separator := make([]string, len(headers))
	for index := range headers {
		separator[index] = strings.Repeat("-", widths[index])
	}
	appendRow(separator)
	for _, row := range rows {
		appendRow(row)
	}
	return "<pre>" + html.EscapeString(strings.Join(lines, "\n")) + "</pre>"
}

// SubjectFor returns the identity key per-module state should be scoped by:
// group/supergroup → chat ID (shared game state), otherwise → user ID.
// Returns "" when no usable id is present (caller should reply with a
// "cannot identify chat" error). Channels and unknown chat types fall
// through to From.ID.
func SubjectFor(msg *models.Message) string {
	if msg == nil {
		return ""
	}
	switch msg.Chat.Type {
	case models.ChatTypeGroup, models.ChatTypeSupergroup:
		return strconv.FormatInt(msg.Chat.ID, 10)
	default:
		if msg.From != nil {
			return strconv.FormatInt(msg.From.ID, 10)
		}
	}
	return ""
}

// ArgAfterCommand returns everything after the first space in text, trimmed.
// Works for `/cmd arg`, `/cmd@bot arg`, etc.
func ArgAfterCommand(text string) string {
	if text == "" {
		return ""
	}
	idx := strings.IndexByte(text, ' ')
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(text[idx+1:])
}

// NowMillis returns current UTC ms-since-epoch.
func NowMillis() int64 { return time.Now().UTC().UnixMilli() }

// replyReserve is the slice of the handler's deadline kept aside for delivering
// the Telegram reply. The whole update handler runs under one bounded context
// (see internal/telegram/webhook.go); if upstream price fetches consume all of
// it, the final SendMessage fails with "context deadline exceeded" and the user
// sees no response. Reserving a fixed tail guarantees delivery headroom.
const replyReserve = 3 * time.Second

// FetchContext derives a child of ctx for upstream data fetches, leaving
// replyReserve of the parent's deadline for the subsequent Reply (which must be
// called with the original ctx, not this child). If ctx has no deadline, or
// less than replyReserve remains, the child gets a small positive floor so a
// fetch still attempts rather than failing instantly. Callers must call the
// returned cancel.
func FetchContext(ctx context.Context) (context.Context, context.CancelFunc) {
	dl, ok := ctx.Deadline()
	if !ok {
		return context.WithCancel(ctx)
	}
	budget := time.Until(dl) - replyReserve
	if budget < time.Second {
		budget = time.Second
	}
	return context.WithTimeout(ctx, budget)
}

// Reply sends a plain-text response to the chat the inbound message came from.
//
// Forwards MessageThreadID so replies in a forum-supergroup topic stay in the
// same topic. Telegram routes outgoing messages with an absent/zero
// message_thread_id to the General topic — that mis-routing is the precise
// reason this helper takes the whole message instead of just a chat ID.
func Reply(ctx context.Context, b *bot.Bot, msg *models.Message, text string) error {
	if msg == nil {
		return nil
	}
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		MessageThreadID: msg.MessageThreadID,
		Text:            text,
	})
	return err
}

// ReplyHTML sends a Telegram HTML-formatted response to the chat the inbound
// message came from. Forwards MessageThreadID — see Reply for rationale.
func ReplyHTML(ctx context.Context, b *bot.Bot, msg *models.Message, text string) error {
	if msg == nil {
		return nil
	}
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		MessageThreadID: msg.MessageThreadID,
		Text:            text,
		ParseMode:       models.ParseModeHTML,
	})
	return err
}

// WinRate computes wins/played as a percentage rounded to nearest int.
// Uses math.Round (round half away from zero for positive inputs) so 2/3
// renders as 67%, not 66% as plain int(...) truncation would give.
// Returns 0 when played == 0 (avoids NaN).
func WinRate(wins, played int) int {
	if played <= 0 {
		return 0
	}
	return int(math.Round(float64(wins) / float64(played) * 100))
}
