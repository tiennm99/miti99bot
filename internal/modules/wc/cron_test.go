package wc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

type fakeSender struct {
	mu          sync.Mutex
	calls       []bot.SendMessageParams
	terminalOn  map[int64]bool
	topicOnlyOn map[int64]bool
	transientOn map[int64]bool
}

func (f *fakeSender) SendMessage(_ context.Context, p *bot.SendMessageParams) (*models.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, *p)
	id, _ := p.ChatID.(int64)
	if f.terminalOn[id] {
		return nil, errors.New("Forbidden: bot was blocked by the user")
	}
	if f.topicOnlyOn[id] {
		return nil, errors.New("Bad Request: have no rights to send a message")
	}
	if f.transientOn[id] {
		return nil, errors.New("connection reset by peer")
	}
	return &models.Message{}, nil
}

func newTestStores() (SubscriberStore, PushDateStore, CacheStore) {
	col := storage.NewMemoryProvider().Collection("wc")
	return storage.Typed[subscribersDoc](col),
		storage.Typed[lastPushDoc](col),
		storage.Typed[cacheRecord](col)
}

func newTestState() *state {
	subs, pd, cache := newTestStores()
	return &state{
		subscribers: subs,
		pushDate:    pd,
		cache:       cache,
		client:      &Client{},
		nowFn:       func() time.Time { return fakeNow },
	}
}

func seedFreshCache(t *testing.T, cache CacheStore, matches []Match) {
	t.Helper()
	rec := cacheRecord{Ts: time.Now().UTC().UnixMilli(), Matches: matches}
	if err := cache.Put(context.Background(), cacheKey(), rec); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
}

func TestRunDailyPush_SendsAndIsIdempotent(t *testing.T) {
	s := newTestState()
	seedFreshCache(t, s.cache, []Match{mkMatch("TIMED", "MEX", "RSA", "2026-06-12T13:00:00Z")})
	if _, err := addSubscriber(context.Background(), s.subscribers, 100, 7); err != nil {
		t.Fatal(err)
	}
	sender := &fakeSender{}
	for i := 0; i < 2; i++ {
		if err := runDailyPush(context.Background(), s, sender); err != nil {
			t.Fatalf("runDailyPush %d: %v", i, err)
		}
	}
	if len(sender.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(sender.calls))
	}
	call := sender.calls[0]
	if call.MessageThreadID != 7 || call.ParseMode != models.ParseModeHTML || !call.DisableNotification {
		t.Fatalf("call = %+v, want thread 7 HTML silent notification", call)
	}
}

func TestRunDailyPush_ClaimsICTDayAtMidnight(t *testing.T) {
	s := newTestState()
	s.nowFn = func() time.Time {
		return time.Date(2026, 6, 12, 17, 0, 0, 0, time.UTC) // 2026-06-13 00:00 ICT
	}
	seedFreshCache(t, s.cache, []Match{mkMatch("TIMED", "MEX", "RSA", "2026-06-12T18:00:00Z")})
	if _, err := addSubscriber(context.Background(), s.subscribers, 100, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.pushDate.Put(context.Background(), lastPushDateKey, lastPushDoc{Date: "2026-06-12"}); err != nil {
		t.Fatal(err)
	}

	sender := &fakeSender{}
	if err := runDailyPush(context.Background(), s, sender); err != nil {
		t.Fatal(err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("calls = %d, want 1 midnight ICT push", len(sender.calls))
	}
	doc, _, err := s.pushDate.Get(context.Background(), lastPushDateKey)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Date != "2026-06-13" {
		t.Fatalf("last push date = %q, want ICT day 2026-06-13", doc.Date)
	}
}

func TestRunDailyPush_PrunesDeadChat(t *testing.T) {
	s := newTestState()
	seedFreshCache(t, s.cache, nil)
	for _, sub := range []Subscriber{{ChatID: 100}, {ChatID: 200}, {ChatID: 200, ThreadID: 9}} {
		if _, err := addSubscriber(context.Background(), s.subscribers, sub.ChatID, sub.ThreadID); err != nil {
			t.Fatal(err)
		}
	}
	sender := &fakeSender{terminalOn: map[int64]bool{200: true}}
	if err := runDailyPush(context.Background(), s, sender); err != nil {
		t.Fatal(err)
	}
	remaining, _ := listSubscribers(context.Background(), s.subscribers)
	if len(remaining) != 1 || remaining[0].ChatID != 100 {
		t.Fatalf("remaining = %v, want only chat 100", remaining)
	}
}

func TestDailyPushCronRegistrationAndNilBot(t *testing.T) {
	s := newTestState()
	c := s.dailyPushCron()
	if c.Name != dailyPushCronName || c.Schedule != "0 17 * * *" || c.Handler == nil {
		t.Fatalf("cron = %+v", c)
	}
	err := s.dailyPushHandler(context.Background(), modules.Deps{Store: storage.NewMemoryProvider().Collection("wc")})
	if !errors.Is(err, errNilBot) {
		t.Fatalf("err = %v, want errNilBot", err)
	}
}
