package stock

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

type fakeDividendProvider struct {
	events []DividendEvent
	err    error
	calls  int
}

type blockingDividendProvider struct {
	started chan struct{}
	release chan struct{}
	events  []DividendEvent
}

func (p *blockingDividendProvider) FetchDividendEvents(_ context.Context, _ string, _, _ time.Time) ([]DividendEvent, error) {
	close(p.started)
	<-p.release
	return append([]DividendEvent(nil), p.events...), nil
}

func (f *fakeDividendProvider) FetchDividendEvents(_ context.Context, _ string, _, _ time.Time) ([]DividendEvent, error) {
	f.calls++
	return append([]DividendEvent(nil), f.events...), f.err
}

func newDividendFlowState(t *testing.T, events []DividendEvent) (*state, Store, PendingDividendStore, *testutil.RecordingBot, time.Time) {
	t.Helper()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, saigonLocation)
	priceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"stockSymbol":"TCB","matchedPrice":30000}]}`))
	}))
	t.Cleanup(priceServer.Close)

	provider := storage.NewMemoryProvider()
	collection := provider.Collection(CollectionName)
	store := storage.Typed[Portfolio](collection)
	pending := storage.Typed[PendingDividendAction](collection)
	s := &state{
		store:     store,
		pending:   pending,
		prices:    &PriceClient{HTTP: priceServer.Client(), URL: priceServer.URL},
		dividends: &fakeDividendProvider{events: events},
		nowFn:     func() time.Time { return now },
		newDividendToken: func() (string, error) {
			return "abcdefghijklmnopqrstuv", nil
		},
	}
	rb := testutil.NewRecordingBot(t)
	return s, store, pending, rb, now
}

func seedDividendFlowPortfolio(t *testing.T, store Store, now time.Time, quantity int64) {
	t.Helper()
	p := NewPortfolio(now.Add(-48 * time.Hour).UnixMilli())
	p.VND = 100_000
	p.Meta.Invested = 3_000_000
	if err := p.BuyTicker("TCB", quantity, float64(quantity)*30_000, now.Add(-48*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := SavePortfolio(context.Background(), store, 7, p); err != nil {
		t.Fatal(err)
	}
}

func TestStockPortfolio_NoDividendEventsSendsOnlyPortfolio(t *testing.T) {
	s, store, _, rb, now := newDividendFlowState(t, nil)
	seedDividendFlowPortfolio(t, store, now, 100)

	if err := s.handleStats(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/stock_portfolio")); err != nil {
		t.Fatal(err)
	}
	calls := rb.Sent()
	if len(calls) != 1 || !strings.Contains(calls[0].Text(), "Stock Portfolio") {
		t.Fatalf("calls = %#v, want only portfolio message", calls)
	}
	p, err := LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Assets["TCB"].DividendCheckedAt; got != now.UnixMilli() {
		t.Fatalf("dividend cursor = %d, want %d", got, now.UnixMilli())
	}
}

func TestStockPortfolio_DividendEventFollowsPortfolioWithOpaqueButton(t *testing.T) {
	event := DividendEvent{
		ProviderID: "2612974", Symbol: "TCB", Kind: DividendKindCash,
		PublishedAt: time.Date(2026, 6, 24, 8, 0, 0, 0, saigonLocation),
		ExDate:      time.Date(2026, 6, 27, 0, 0, 0, 0, saigonLocation),
		VNDPerShare: 1500, Title: "Cash dividend",
	}
	s, store, pending, rb, now := newDividendFlowState(t, []DividendEvent{event})
	seedDividendFlowPortfolio(t, store, now, 100)

	if err := s.handleStats(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/stock_portfolio")); err != nil {
		t.Fatal(err)
	}
	calls := rb.Sent()
	if len(calls) != 2 || !strings.Contains(calls[0].Text(), "Stock Portfolio") || !strings.Contains(calls[1].Text(), "Recent dividend event") {
		t.Fatalf("unexpected message order: %#v", calls)
	}
	markup := calls[1].Form["reply_markup"]
	if !strings.Contains(markup, dividendCallbackPrefix+"abcdefghijklmnopqrstuv") || strings.Contains(markup, "1500") || strings.Contains(markup, "2612974") {
		t.Fatalf("callback markup is not opaque: %q", markup)
	}
	keys, err := pending.List(context.Background(), pendingDividendPrefix)
	if err != nil || len(keys) != 1 {
		t.Fatalf("pending keys = %v, err=%v", keys, err)
	}
	action, _, err := pending.Get(context.Background(), keys[0])
	if err != nil || action.OwnerUserID != 7 || action.ChatID != 7 || action.MessageID != 1 {
		t.Fatalf("pending action = %+v, err=%v", action, err)
	}
}

func TestDividendCallback_OnlyOwnerCanApplyAndUsesCurrentHolding(t *testing.T) {
	s, store, pending, rb, now := newDividendFlowState(t, nil)
	seedDividendFlowPortfolio(t, store, now, 100)
	action := PendingDividendAction{
		OwnerUserID: 7, ChatID: -100, MessageID: 99, ProviderEventID: "2612974",
		Symbol: "TCB", Kind: DividendKindCash, VNDPerShare: 1500,
		ObservedHolding: 50, PositionOpenedAt: now.Add(-48 * time.Hour).UnixMilli(), CheckThrough: now.UnixMilli(), CreatedAt: now.UnixMilli(), ExpiresAt: now.Add(time.Hour).UnixMilli(),
	}
	key := pendingDividendKey("abcdefghijklmnopqrstuv")
	if err := pending.Put(context.Background(), key, action); err != nil {
		t.Fatal(err)
	}

	callback := func(userID int64) *models.Update {
		return &models.Update{CallbackQuery: &models.CallbackQuery{
			ID: "query", From: models.User{ID: userID}, Data: dividendCallbackPrefix + "abcdefghijklmnopqrstuv",
			Message: models.MaybeInaccessibleMessage{Type: models.MaybeInaccessibleMessageTypeMessage, Message: &models.Message{
				ID: 99, Chat: models.Chat{ID: -100, Type: models.ChatTypeGroup}, MessageThreadID: 3,
			}},
		}}
	}
	if err := s.handleDividendCallback(context.Background(), rb.Bot, callback(8)); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if err != nil || p.VND != 100_000 {
		t.Fatalf("unauthorized click changed portfolio: %+v, err=%v", p, err)
	}
	if _, _, err := pending.Get(context.Background(), key); err != nil {
		t.Fatalf("unauthorized click consumed action: %v", err)
	}

	s.nowFn = func() time.Time { return now.Add(30 * time.Minute) }
	if err := s.handleDividendCallback(context.Background(), rb.Bot, callback(7)); err != nil {
		t.Fatal(err)
	}
	p, err = LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if p.VND != 250_000 || p.Assets["TCB"].Quantity != 100 || p.Assets["TCB"].DividendCheckedAt != now.UnixMilli() || len(p.AppliedDividendEvents) != 1 {
		t.Fatalf("applied portfolio = %+v", p)
	}
	if err := s.handleDividendCallback(context.Background(), rb.Bot, callback(7)); err != nil {
		t.Fatal(err)
	}
	p, _ = LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if p.VND != 250_000 {
		t.Fatalf("repeated click credited twice: %v", p.VND)
	}
}

func TestDividendCallback_AppliesShareDividendOnce(t *testing.T) {
	s, store, pending, rb, now := newDividendFlowState(t, nil)
	seedDividendFlowPortfolio(t, store, now, 139)
	action := PendingDividendAction{
		OwnerUserID: 7, ChatID: 7, MessageID: 1, ProviderEventID: "2612975",
		Symbol: "TCB", Kind: DividendKindShares, OwnedShares: 100, NewShares: 10,
		ObservedHolding: 139, PositionOpenedAt: now.Add(-48 * time.Hour).UnixMilli(), CheckThrough: now.UnixMilli(), CreatedAt: now.UnixMilli(), ExpiresAt: now.Add(time.Hour).UnixMilli(),
	}
	if err := pending.Put(context.Background(), pendingDividendKey("abcdefghijklmnopqrstuv"), action); err != nil {
		t.Fatal(err)
	}
	update := &models.Update{CallbackQuery: &models.CallbackQuery{
		ID: "query", From: models.User{ID: 7}, Data: dividendCallbackPrefix + "abcdefghijklmnopqrstuv",
		Message: models.MaybeInaccessibleMessage{Type: models.MaybeInaccessibleMessageTypeMessage, Message: &models.Message{
			ID: 1, Chat: models.Chat{ID: 7, Type: models.ChatTypePrivate},
		}},
	}}
	if err := s.handleDividendCallback(context.Background(), rb.Bot, update); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if p.Assets["TCB"].Quantity != 152 || p.Assets["TCB"].Base != 139*30_000 {
		t.Fatalf("share dividend result = %+v", p.Assets["TCB"])
	}
}

func TestDividendCallback_RejectsSoldAndReopenedPosition(t *testing.T) {
	s, store, pending, rb, now := newDividendFlowState(t, nil)
	seedDividendFlowPortfolio(t, store, now, 100)
	action := PendingDividendAction{
		OwnerUserID: 7, ChatID: 7, MessageID: 1, ProviderEventID: "2612974",
		Symbol: "TCB", Kind: DividendKindCash, VNDPerShare: 1500,
		PositionOpenedAt: now.Add(-48 * time.Hour).UnixMilli(), CheckThrough: now.UnixMilli(), CreatedAt: now.UnixMilli(), ExpiresAt: now.Add(time.Hour).UnixMilli(),
	}
	key := pendingDividendKey("abcdefghijklmnopqrstuv")
	if err := pending.Put(context.Background(), key, action); err != nil {
		t.Fatal(err)
	}
	p, _ := LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if _, _, ok, err := p.SellTicker("TCB", 100); err != nil || !ok {
		t.Fatalf("sell before reopen: ok=%v err=%v", ok, err)
	}
	if err := p.BuyTicker("TCB", 50, 2_000_000, now.Add(time.Minute).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := SavePortfolio(context.Background(), store, 7, p); err != nil {
		t.Fatal(err)
	}
	update := dividendCallbackUpdate(7, 7, 1)
	if err := s.handleDividendCallback(context.Background(), rb.Bot, update); err != nil {
		t.Fatal(err)
	}
	p, _ = LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if p.VND != 100_000 || p.Assets["TCB"].Quantity != 50 {
		t.Fatalf("stale action changed reopened position: %+v", p)
	}
	if _, _, err := pending.Get(context.Background(), key); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("stale action was not invalidated: %v", err)
	}
}

func TestDividendCallback_ExpiredActionDoesNotApply(t *testing.T) {
	s, store, pending, rb, now := newDividendFlowState(t, nil)
	seedDividendFlowPortfolio(t, store, now, 100)
	action := PendingDividendAction{
		OwnerUserID: 7, ChatID: 7, MessageID: 1, ProviderEventID: "2612974",
		Symbol: "TCB", Kind: DividendKindCash, VNDPerShare: 1500,
		PositionOpenedAt: now.Add(-48 * time.Hour).UnixMilli(), CheckThrough: now.UnixMilli(), CreatedAt: now.Add(-2 * time.Hour).UnixMilli(), ExpiresAt: now.Add(-time.Hour).UnixMilli(),
	}
	key := pendingDividendKey("abcdefghijklmnopqrstuv")
	if err := pending.Put(context.Background(), key, action); err != nil {
		t.Fatal(err)
	}
	if err := s.handleDividendCallback(context.Background(), rb.Bot, dividendCallbackUpdate(7, 7, 1)); err != nil {
		t.Fatal(err)
	}
	p, _ := LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if p.VND != 100_000 {
		t.Fatalf("expired action changed balance: %v", p.VND)
	}
}

func TestDividendSuggestionFailureLeavesCursorUnchanged(t *testing.T) {
	event := DividendEvent{
		ProviderID: "2612974", Symbol: "TCB", Kind: DividendKindCash,
		PublishedAt: time.Date(2026, 6, 24, 8, 0, 0, 0, saigonLocation), VNDPerShare: 1500,
	}
	s, store, _, rb, now := newDividendFlowState(t, []DividendEvent{event})
	seedDividendFlowPortfolio(t, store, now, 100)
	p, _ := LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	before := p.Assets["TCB"].DividendCheckedAt
	rb.FailMethod("sendMessage", http.StatusInternalServerError, "")
	_ = s.notifyDividendEvents(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/stock_portfolio").Message, 7, p, now)
	p, _ = LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if got := p.Assets["TCB"].DividendCheckedAt; got != before {
		t.Fatalf("failed delivery advanced cursor: got %d want %d", got, before)
	}
}

func TestAdvanceDividendCursorPreservesConcurrentTrade(t *testing.T) {
	s, store, _, _, now := newDividendFlowState(t, nil)
	seedDividendFlowPortfolio(t, store, now, 100)
	p, _ := LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if err := p.BuyTicker("TCB", 25, 1_000_000, now.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := SavePortfolio(context.Background(), store, 7, p); err != nil {
		t.Fatal(err)
	}
	if err := s.advanceDividendCursors(context.Background(), 7, []string{"TCB"}, now.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	p, _ = LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if p.Assets["TCB"].Quantity != 125 || p.Assets["TCB"].Base != 4_000_000 {
		t.Fatalf("cursor merge lost trade: %+v", p.Assets["TCB"])
	}
}

func TestConcurrentDividendSuggestionsCreateOneButton(t *testing.T) {
	s, store, pending, rb, now := newDividendFlowState(t, nil)
	seedDividendFlowPortfolio(t, store, now, 100)
	event := DividendEvent{ProviderID: "2612974", Symbol: "TCB", Kind: DividendKindCash, VNDPerShare: 1500}
	msg := testutil.NewPrivateMessage(7, "/stock_portfolio").Message
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.sendDividendSuggestion(context.Background(), rb.Bot, msg, 7, now.Add(-48*time.Hour).UnixMilli(), now, event)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls := rb.Sent(); len(calls) != 1 {
		t.Fatalf("sent %d duplicate suggestions: %#v", len(calls), calls)
	}
	keys, err := pending.List(context.Background(), pendingDividendPrefix)
	if err != nil || len(keys) != 1 {
		t.Fatalf("pending actions = %v, err=%v", keys, err)
	}
}

func TestDividendFetchDoesNotBindOldEventToReopenedPosition(t *testing.T) {
	s, store, pending, rb, now := newDividendFlowState(t, nil)
	seedDividendFlowPortfolio(t, store, now, 100)
	blocking := &blockingDividendProvider{
		started: make(chan struct{}), release: make(chan struct{}),
		events: []DividendEvent{{ProviderID: "2612974", Symbol: "TCB", Kind: DividendKindCash, VNDPerShare: 1500}},
	}
	s.dividends = blocking
	done := make(chan error, 1)
	go func() {
		done <- s.handleStats(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/stock_portfolio"))
	}()
	<-blocking.started
	p, _ := LoadPortfolio(context.Background(), store, 7, now.UnixMilli())
	if _, _, ok, err := p.SellTicker("TCB", 100); err != nil || !ok {
		t.Fatalf("sell during fetch: ok=%v err=%v", ok, err)
	}
	if err := p.BuyTicker("TCB", 50, 2_000_000, now.Add(time.Minute).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := SavePortfolio(context.Background(), store, 7, p); err != nil {
		t.Fatal(err)
	}
	close(blocking.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if calls := rb.Sent(); len(calls) != 1 || !strings.Contains(calls[0].Text(), "Stock Portfolio") {
		t.Fatalf("old lifecycle event was sent: %#v", calls)
	}
	keys, err := pending.List(context.Background(), pendingDividendPrefix)
	if err != nil || len(keys) != 0 {
		t.Fatalf("old lifecycle created pending action: %v, err=%v", keys, err)
	}
}

func dividendCallbackUpdate(userID, chatID int64, messageID int) *models.Update {
	return &models.Update{CallbackQuery: &models.CallbackQuery{
		ID: "query", From: models.User{ID: userID}, Data: dividendCallbackPrefix + "abcdefghijklmnopqrstuv",
		Message: models.MaybeInaccessibleMessage{Type: models.MaybeInaccessibleMessageTypeMessage, Message: &models.Message{
			ID: messageID, Chat: models.Chat{ID: chatID, Type: models.ChatTypePrivate},
		}},
	}}
}
