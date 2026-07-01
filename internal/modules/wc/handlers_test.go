package wc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

func installWC(t *testing.T, bodyJSON string, now time.Time) (*testutil.RecordingBot, SubscriberStore) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bodyJSON))
	}))
	t.Cleanup(upstream.Close)

	rb := testutil.NewRecordingBot(t)
	col := storage.NewMemoryProvider().Collection("wc")
	s := &state{
		subscribers: storage.Typed[subscribersDoc](col),
		pushDate:    storage.Typed[lastPushDoc](col),
		cache:       storage.Typed[cacheRecord](col),
		client:      &Client{HTTP: upstream.Client(), URL: upstream.URL, Token: "secret-token"},
		nowFn:       func() time.Time { return now },
	}
	mod := modules.Module{
		Name: "wc",
		Commands: []modules.Command{
			{Name: "wc", Visibility: modules.VisibilityPublic, Description: "x", Handler: s.handleSchedule},
			{Name: "wc_this_week", Visibility: modules.VisibilityPublic, Description: "x", Handler: s.handleWeek},
			{Name: "wc_subscribe", Visibility: modules.VisibilityPublic, Description: "x", Handler: s.handleSubscribe},
			{Name: "wc_unsubscribe", Visibility: modules.VisibilityPublic, Description: "x", Handler: s.handleUnsubscribe},
		},
	}
	reg := &modules.Registry{Modules: []modules.Module{mod}, AllCommands: map[string]modules.Command{}}
	for _, c := range mod.Commands {
		reg.AllCommands[c.Name] = c
	}
	modules.Install(rb.Bot, reg, modules.Auth{})
	return rb, s.subscribers
}

var fakeNow = time.Date(2026, 6, 12, 5, 0, 0, 0, time.UTC)

func TestHandleSchedule_DefaultsToToday(t *testing.T) {
	rb, _ := installWC(t, sampleMatchesBody, fakeNow)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(1, "/wc"))

	got := rb.LastSent()
	if got.Method != "sendMessage" {
		t.Fatalf("method = %q, want sendMessage", got.Method)
	}
	if got.Form["parse_mode"] != "HTML" {
		t.Fatalf("parse_mode = %q, want HTML", got.Form["parse_mode"])
	}
	for _, want := range []string{"<b>World Cup -", "MEX vs RSA", "Estadio Azteca"} {
		if !strings.Contains(got.Text(), want) {
			t.Fatalf("missing %q in:\n%s", want, got.Text())
		}
	}
}

func TestHandleSchedule_BadDateInput(t *testing.T) {
	rb, _ := installWC(t, sampleMatchesBody, fakeNow)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(1, "/wc notadate"))
	if got := rb.LastSent().Text(); !strings.Contains(got, "Invalid date") {
		t.Fatalf("reply = %q, want invalid date", got)
	}
}

func TestHandleWeek_RendersThisWeek(t *testing.T) {
	rb, _ := installWC(t, sampleMatchesBody, fakeNow)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(1, "/wc_this_week"))

	got := rb.LastSent().Text()
	for _, want := range []string{"Mon Jun 8", "Sun Jun 14", "MEX vs RSA", "CAN vs SUI"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestHandleSubscribe_AddsAndIsIdempotent(t *testing.T) {
	rb, store := installWC(t, sampleMatchesBody, fakeNow)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wc_subscribe"))
	if got := rb.LastSent().Text(); !strings.Contains(got, "Subscribed") || !strings.Contains(got, "00:00 UTC+7") {
		t.Fatalf("first reply = %q, want subscribed with 00:00 UTC+7", got)
	}
	rb.Reset()
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wc_subscribe"))
	if got := rb.LastSent().Text(); !strings.Contains(got, "Already subscribed") {
		t.Fatalf("second reply = %q, want already subscribed", got)
	}
	subs, _ := listSubscribers(context.Background(), store)
	if len(subs) != 1 || subs[0] != (Subscriber{ChatID: 7}) {
		t.Fatalf("subs = %v, want [{7 0}]", subs)
	}
}

func TestHandleSubscribe_ForumTopic(t *testing.T) {
	rb, store := installWC(t, sampleMatchesBody, fakeNow)
	upd := testutil.NewSupergroupMessage(555, 999, "/wc_subscribe")
	upd.Message.MessageThreadID = 42
	rb.Bot.ProcessUpdate(context.Background(), upd)

	subs, _ := listSubscribers(context.Background(), store)
	if len(subs) != 1 || subs[0] != (Subscriber{ChatID: 555, ThreadID: 42}) {
		t.Fatalf("subs = %v, want topic subscription", subs)
	}
}

func TestHandleSchedule_MissingToken(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	col := storage.NewMemoryProvider().Collection("wc")
	s := &state{
		subscribers: storage.Typed[subscribersDoc](col),
		pushDate:    storage.Typed[lastPushDoc](col),
		cache:       storage.Typed[cacheRecord](col),
		client:      &Client{},
		nowFn:       func() time.Time { return fakeNow },
	}
	cmd := modules.Command{Name: "wc", Visibility: modules.VisibilityPublic, Description: "x", Handler: s.handleSchedule}
	reg := &modules.Registry{
		Modules:     []modules.Module{{Name: "wc", Commands: []modules.Command{cmd}}},
		AllCommands: map[string]modules.Command{cmd.Name: cmd},
	}
	modules.Install(rb.Bot, reg, modules.Auth{})

	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(1, "/wc"))
	if got := rb.LastSent().Text(); !strings.Contains(got, "WC_FOOTBALL_DATA_TOKEN") {
		t.Fatalf("reply = %q, want missing token hint", got)
	}
}
