package wc

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

type state struct {
	subscribers SubscriberStore
	pushDate    PushDateStore
	cache       CacheStore
	client      *Client
	nowFn       func() time.Time

	subscribersMu sync.Mutex
}

func (s *state) now() time.Time {
	if s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now()
}

// handleSchedule is /wc [date], defaulting to today in ICT.
func (s *state) handleSchedule(ctx context.Context, b *bot.Bot, update *models.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}
	arg := chathelper.ArgAfterCommand(msg.Text)
	parsed := ParseScheduleDate(arg, s.now())
	if !parsed.OK {
		return chathelper.Reply(ctx, b, msg, parsed.Error)
	}
	return s.replyForRange(ctx, b, msg, parsed.Date, addDays(parsed.Date, 1), false)
}

func (s *state) handleToday(ctx context.Context, b *bot.Bot, update *models.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}
	from := ictDayStartOf(s.now())
	return s.replyForRange(ctx, b, msg, from, addDays(from, 1), false)
}

func (s *state) handleWeek(ctx context.Context, b *bot.Bot, update *models.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}
	from := ictWeekStartOf(s.now())
	return s.replyForRange(ctx, b, msg, from, addDays(from, 7), true)
}

func (s *state) replyForRange(ctx context.Context, b *bot.Bot, msg *models.Message, from, to time.Time, week bool) error {
	matches, err := s.client.GetMatchesCached(ctx, s.cache, from, to)
	if err != nil {
		log.Error("wc_fetch_fail", "err", err, "from", from, "to", to)
		if errors.Is(err, ErrNotConfigured) {
			return chathelper.Reply(ctx, b, msg, "World Cup schedule is not configured (missing WC_FOOTBALL_DATA_TOKEN).")
		}
		hint := "Could not fetch World Cup matches. Try again later."
		if week {
			hint = "Could not fetch this week's World Cup matches. Try again later."
		}
		return chathelper.Reply(ctx, b, msg, hint)
	}
	var text string
	if week {
		text = RenderWeek(matches, from, to)
	} else {
		text = RenderToday(matches, from)
	}
	return chathelper.ReplyHTML(ctx, b, msg, text)
}

func (s *state) handleSubscribe(ctx context.Context, b *bot.Bot, update *models.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	added, err := addSubscriber(ctx, s.subscribers, msg.Chat.ID, msg.MessageThreadID)
	if err != nil {
		return err
	}
	if added {
		return chathelper.Reply(ctx, b, msg,
			"Subscribed. You'll get today's World Cup schedule at 08:00 ICT.\n"+
				"If you block the bot, you'll be auto-unsubscribed on the next push.")
	}
	return chathelper.Reply(ctx, b, msg, "Already subscribed.")
}

func (s *state) handleUnsubscribe(ctx context.Context, b *bot.Bot, update *models.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	removed, err := removeSubscriber(ctx, s.subscribers, msg.Chat.ID, msg.MessageThreadID)
	if err != nil {
		return err
	}
	if removed {
		return chathelper.Reply(ctx, b, msg, "Unsubscribed.")
	}
	return chathelper.Reply(ctx, b, msg, "You weren't subscribed.")
}
