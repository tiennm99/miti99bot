package wc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

const dailyPushCronName = "wc_daily_push"

// 17:00 UTC is 00:00 UTC+7 (ICT).
const dailyPushSchedule = "0 17 * * *"

const lastPushDateKey = "daily_push:last_date"

const telegramRateLimitThreshold = 30

const telegramRateLimitDelay = 50 * time.Millisecond

type messageSender interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
}

type lastPushDoc struct {
	Date string `json:"date" bson:"date"`
}

// PushDateStore is the typed store for last-push date documents.
type PushDateStore = storage.DocStore[lastPushDoc]

func (s *state) dailyPushCron() modules.Cron {
	return modules.Cron{
		Name:     dailyPushCronName,
		Schedule: dailyPushSchedule,
		Handler:  s.dailyPushHandler,
	}
}

func (s *state) dailyPushHandler(ctx context.Context, deps modules.Deps) error {
	if deps.Bot == nil {
		return errNilBot
	}
	return runDailyPush(ctx, s, deps.Bot)
}

func claimDailyPush(ctx context.Context, store PushDateStore, today string) (bool, error) {
	current, version, err := store.Get(ctx, lastPushDateKey)
	switch {
	case err == nil:
		if current.Date == today {
			return false, nil
		}
	case errors.Is(err, storage.ErrNotFound):
		version = 0
	default:
		return false, err
	}

	if err := store.PutVersioned(ctx, lastPushDateKey, version, lastPushDoc{Date: today}); err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func runDailyPush(ctx context.Context, s *state, sender messageSender) error {
	subs, err := listSubscribers(ctx, s.subscribers)
	if err != nil {
		return fmt.Errorf("wc daily push: list subscribers: %w", err)
	}
	if len(subs) == 0 {
		log.Info("wc daily push: no subscribers, skipping")
		return nil
	}

	from := ictDayStartOf(s.now())
	to := addDays(from, 1)
	matches, err := s.client.GetMatchesCached(ctx, s.cache, from, to)
	if err != nil {
		return fmt.Errorf("wc daily push: fetch matches: %w", err)
	}
	text := RenderToday(matches, from)

	today := s.now().UTC().Format("2006-01-02")
	won, err := claimDailyPush(ctx, s.pushDate, today)
	if err != nil {
		return fmt.Errorf("wc daily push: claim date: %w", err)
	}
	if !won {
		log.Info("wc daily push: already pushed today, skipping", "date", today)
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
			DisableNotification: true,
		}); err != nil {
			log.Warn("wc daily push send failed", "chat", sub.ChatID, "thread", sub.ThreadID, "err", err)
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

	pruned := pruneDeadSubscribers(ctx, s, deadChats, deadTopics)
	log.Info("wc daily push complete",
		"subscribers", len(subs),
		"sent", sent,
		"failed", failed,
		"pruned", pruned,
		"throttled", throttle)
	return nil
}

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
			log.Warn("wc prune dead chat failed", "chat", chatID, "err", err)
			continue
		}
		removed += n
	}
	for _, sub := range topicOnly {
		if _, ok := chatWide[sub.ChatID]; ok {
			continue
		}
		ok, err := removeSubscriber(ctx, s.subscribers, sub.ChatID, sub.ThreadID)
		if err != nil {
			log.Warn("wc prune dead topic failed", "chat", sub.ChatID, "thread", sub.ThreadID, "err", err)
			continue
		}
		if ok {
			removed++
		}
	}
	return removed
}
