package lolschedule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// terminalKind classifies a permanent send failure by blast radius.
type terminalKind int

const (
	// terminalNone is a transient failure (rate limit, timeout, 5xx). The
	// subscriber stays on the list and we retry on the next push.
	terminalNone terminalKind = iota
	// terminalChatWide means the chat itself is unreachable (bot blocked,
	// chat deactivated, kicked, deleted, group upgraded). Every subscriber
	// entry for that ChatID must be pruned — sister topics are dead too.
	terminalChatWide
	// terminalTopicOnly means the bot lost send rights in the specific topic
	// (e.g. a topic-level permissions change). Only the (ChatID, ThreadID)
	// entry that failed should be pruned; other topics in the same chat may
	// still be valid.
	terminalTopicOnly
)

// chatWideTerminalMarkers are substrings of Telegram API errors that mean
// the whole chat is gone, not just one topic. Detecting these lets the
// daily-push handler prune every subscription for that ChatID at once.
//
// String matching is fragile by nature, but the bot library surfaces these
// directly in err.Error() and Telegram has used the same wording for years.
// The false-negative path (we miss a new wording, dead chat lingers) is
// strictly safer than the false-positive path (we wrongly prune a live chat).
var chatWideTerminalMarkers = []string{
	"bot was blocked by the user",
	"user is deactivated",
	"bot is not a member",
	"chat not found",
	"group chat was upgraded",
	"chat was deleted",
}

// topicOnlyTerminalMarkers are errors that scope to a single forum topic
// (or to the bot's per-topic permissions). Pruning only the offending
// (ChatID, ThreadID) keeps the chat's other topic subscriptions alive.
var topicOnlyTerminalMarkers = []string{
	"have no rights to send",
}

// classifyTerminal reports whether err is a permanent send failure and, if
// so, whether it kills the whole chat or only the originating topic.
func classifyTerminal(err error) terminalKind {
	if err == nil {
		return terminalNone
	}
	msg := err.Error()
	for _, m := range chatWideTerminalMarkers {
		if strings.Contains(msg, m) {
			return terminalChatWide
		}
	}
	for _, m := range topicOnlyTerminalMarkers {
		if strings.Contains(msg, m) {
			return terminalTopicOnly
		}
	}
	return terminalNone
}

// dailyPushCronName is the cron route segment + in-process scheduler key.
// Must match the regex in internal/server/router.go (^[a-z0-9_]{1,32}$).
const dailyPushCronName = "lolschedule_daily_push"

// dailyPushSchedule drives the in-process scheduler (internal/cron). Cron
// expression is UTC; 01:00 UTC == 08:00 ICT.
const dailyPushSchedule = "0 1 * * *"

// lastPushDateKey records the UTC date (YYYY-MM-DD) of the most recent
// completed daily push. The handler claims this key before fanning out and
// no-ops if it is already today's date, making the push idempotent per UTC
// date. This defends against double-fire windows from rolling deploys that
// briefly run two containers or operator misconfiguration.
const lastPushDateKey = "daily_push:last_date"

// telegramRateLimitThreshold is the subscriber count above which we throttle
// sends to stay under Telegram's global 30 msg/sec cap. Below it we send hot.
const telegramRateLimitThreshold = 30

// telegramRateLimitDelay is the inter-send pause when above the threshold.
// 50ms = ~20 msg/sec, well clear of the 30/s ceiling with margin for jitter.
const telegramRateLimitDelay = 50 * time.Millisecond

// messageSender is the subset of *bot.Bot the cron handler uses. Defining it
// as an interface lets tests inject a mock without spinning up a fake
// Telegram API server.
type messageSender interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
}

// lastPushDoc wraps the last-push date string so it can be stored as a named
// root field in a Mongo document (a bare scalar cannot be a root doc).
type lastPushDoc struct {
	Date string `json:"date" bson:"date"`
}

// PushDateStore is the typed store for last-push date documents.
type PushDateStore = storage.DocStore[lastPushDoc]

// dailyPushCron returns the cron registration. Schedule is documentation only.
func (s *state) dailyPushCron() modules.Cron {
	return modules.Cron{
		Name:     dailyPushCronName,
		Schedule: dailyPushSchedule,
		Handler:  s.dailyPushHandler,
	}
}

// dailyPushHandler is invoked by the cron dispatcher. It pulls Bot from Deps
// (set in main.go via BuildOptions.Bot) and delegates to runDailyPush so the
// core logic is testable without an actual *bot.Bot.
func (s *state) dailyPushHandler(ctx context.Context, deps modules.Deps) error {
	if deps.Bot == nil {
		return errors.New("lolschedule daily push: deps.Bot is nil (BuildOptions.Bot not wired)")
	}
	return runDailyPush(ctx, s, deps.Bot)
}

// claimDailyPush atomically records that today's UTC push is happening and
// reports whether THIS caller won the claim. It is the idempotency primitive
// for the daily push: a winner proceeds to fan out; a loser (another trigger
// already claimed today) returns false and sends nothing.
//
// The claim uses version-based optimistic write (PutVersioned) on
// lastPushDateKey so two simultaneous triggers cannot both win.
func claimDailyPush(ctx context.Context, store PushDateStore, today string) (bool, error) {
	current, version, err := store.Get(ctx, lastPushDateKey)
	switch {
	case err == nil:
		if current.Date == today {
			return false, nil // already pushed today
		}
	case errors.Is(err, storage.ErrNotFound):
		version = 0 // never pushed
	default:
		return false, err
	}

	if err := store.PutVersioned(ctx, lastPushDateKey, version, lastPushDoc{Date: today}); err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return false, nil // another trigger claimed today first
		}
		return false, err
	}
	return true, nil
}

// runDailyPush is the testable core: fetch subscribers, fetch today's matches,
// fan out to every subscriber. Per-chat send failures are logged but do not
// abort the batch — one bad chat does not deny the rest.
//
// MessageThreadID is forwarded on every send so subscribers in a forum-topic
// receive the digest in that topic, not in General.
func runDailyPush(ctx context.Context, s *state, sender messageSender) error {
	subs, err := listSubscribers(ctx, s.subscribers)
	if err != nil {
		return fmt.Errorf("lolschedule daily push: list subscribers: %w", err)
	}
	if len(subs) == 0 {
		log.Info("lolschedule daily push: no subscribers, skipping")
		return nil
	}

	from := ictDayStartOf(s.now())
	to := addDays(from, 1)
	events, err := s.client.GetEventsCached(ctx, s.cache, from, to)
	if err != nil {
		return fmt.Errorf("lolschedule daily push: fetch matches: %w", err)
	}
	filtered := FilterMajor(events)
	text := RenderToday(filtered, from)
	disableNotification := len(filtered) == 0

	// Idempotency gate: claim today's push before sending. A lost claim means
	// another trigger already pushed (or is pushing) for this UTC date, so we
	// send nothing. Placed after the fetch so a transient fetch failure does
	// not consume the day's claim.
	today := s.now().UTC().Format("2006-01-02")
	won, err := claimDailyPush(ctx, s.pushDate, today)
	if err != nil {
		return fmt.Errorf("lolschedule daily push: claim date: %w", err)
	}
	if !won {
		log.Info("lolschedule daily push: already pushed today, skipping", "date", today)
		return nil
	}

	throttle := len(subs) > telegramRateLimitThreshold
	var sent, failed int
	deadChats := map[int64]struct{}{}
	var deadTopics []Subscriber
	for i, sub := range subs {
		if throttle && i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(telegramRateLimitDelay):
			}
		}
		if _, err := sender.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:              sub.ChatID,
			MessageThreadID:     sub.ThreadID,
			Text:                text,
			ParseMode:           models.ParseModeHTML,
			DisableNotification: disableNotification,
		}); err != nil {
			log.Warn("lolschedule daily push send failed",
				"chat", sub.ChatID, "thread", sub.ThreadID, "err", err)
			failed++
			switch classifyTerminal(err) {
			case terminalChatWide:
				deadChats[sub.ChatID] = struct{}{}
			case terminalTopicOnly:
				deadTopics = append(deadTopics, sub)
			}
			continue
		}
		sent++
	}

	// Best-effort prune. Failure here just leaves the dead chats in the list
	// for tomorrow's push — same behaviour as before this code existed, so
	// strictly an improvement even when the writes fail.
	pruned := pruneDeadSubscribers(ctx, s, deadChats, deadTopics)

	log.Info("lolschedule daily push complete",
		"subscribers", len(subs),
		"sent", sent,
		"failed", failed,
		"pruned", pruned,
		"throttled", throttle)
	return nil
}

// pruneDeadSubscribers removes entries flagged unreachable. Chat-wide failures
// drop every subscription for the chat; topic-only failures drop just the one
// (ChatID, ThreadID). Serializes through state.subscribersMu so a concurrent
// /subscribe handler doesn't lose its write. Returns total entries removed.
func pruneDeadSubscribers(ctx context.Context, s *state, chatWide map[int64]struct{}, topicOnly []Subscriber) int {
	if len(chatWide) == 0 && len(topicOnly) == 0 {
		return 0
	}
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	removed := 0
	for chatID := range chatWide {
		n, err := removeAllForChat(ctx, s.subscribers, chatID)
		if err != nil {
			log.Warn("lolschedule prune dead chat failed", "chat", chatID, "err", err)
			continue
		}
		removed += n
	}
	for _, sub := range topicOnly {
		// Skip if the whole chat was already pruned above — saves a redundant
		// Get→mutate→Put round trip.
		if _, ok := chatWide[sub.ChatID]; ok {
			continue
		}
		ok, err := removeSubscriber(ctx, s.subscribers, sub.ChatID, sub.ThreadID)
		if err != nil {
			log.Warn("lolschedule prune dead topic failed",
				"chat", sub.ChatID, "thread", sub.ThreadID, "err", err)
			continue
		}
		if ok {
			removed++
		}
	}
	return removed
}
