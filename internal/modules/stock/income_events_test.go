package stock

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

func newTestIncomeEventClient(t *testing.T, handler http.HandlerFunc) (*IncomeEventClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &IncomeEventClient{
		HTTP: srv.Client(),
		URL:  srv.URL,
	}, srv
}

func TestIncomeEventClient_FetchRecentUsesFireAntTimescaleMarks(t *testing.T) {
	now := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	c, _ := newTestIncomeEventClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/symbols/TCX/timescale-marks" {
			t.Errorf("path = %q, want /symbols/TCX/timescale-marks", r.URL.Path)
		}
		if got := r.URL.Query().Get("startDate"); got != "2026-05-06T00:00:00Z" {
			t.Errorf("startDate query = %q, want 2026-05-06T00:00:00Z", got)
		}
		if got := r.URL.Query().Get("endDate"); got != "2026-06-05T00:00:00Z" {
			t.Errorf("endDate query = %q, want 2026-06-05T00:00:00Z", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("authorization header = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"1","label":"Cổ tức","date":"2026-05-25T00:00:00Z","title":"TCX: dividend"},{"id":"2","label":"BCTC","date":"2026-05-24T00:00:00Z","title":"Financial report"},{"id":"3","label":"Cổ tức","date":"2025-06-12T00:00:00Z","title":"Old dividend"}]`))
	})
	c.Token = "test-token"

	got, err := c.FetchRecent(context.Background(), "tcx", now.Add(-incomeEventsLookback), now)
	if err != nil {
		t.Fatalf("FetchRecent: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events = %d, want 1: %+v", len(got), got)
	}
	if got[0].Symbol != "TCX" {
		t.Errorf("symbol = %q, want TCX", got[0].Symbol)
	}
	if got[0].Subtitle != "Cổ tức" {
		t.Errorf("subtitle = %q, want Cổ tức", got[0].Subtitle)
	}
}

func TestIncomeEventClient_FiltersNonIncomeMarks(t *testing.T) {
	now := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	c, _ := newTestIncomeEventClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"1","label":"BCTC","date":"2026-05-25T00:00:00Z","title":"Financial report"},{"id":"2","label":"GDKHQ","date":"2026-05-24T00:00:00Z","title":"Ngày đăng ký cuối cùng trả cổ tức"},{"id":"3","label":"GDKHQ","date":"2026-05-23T00:00:00Z","title":"Ngày đăng ký cuối cùng tham dự Đại hội đồng cổ đông"}]`))
	})

	got, err := c.FetchRecent(context.Background(), "TCX", now.Add(-incomeEventsLookback), now)
	if err != nil {
		t.Fatalf("FetchRecent: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Ngày đăng ký cuối cùng trả cổ tức" {
		t.Fatalf("events = %+v, want only income mark", got)
	}
}

func TestIncomeEventClient_RejectsNonHTTPSRemoteURL(t *testing.T) {
	c := &IncomeEventClient{URL: "http://official.example/events", Token: "secret"}
	_, err := c.FetchRecent(context.Background(), "TCX", time.Now().Add(-incomeEventsLookback), time.Now())
	if err == nil || !strings.Contains(err.Error(), "must be https") {
		t.Fatalf("FetchRecent error = %v, want https requirement", err)
	}
}

func TestRenderIncomeEvents(t *testing.T) {
	now := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	got := RenderIncomeEvents([]IncomeEvent{
		{
			Symbol:     "TCX",
			Title:      "TCX: dividend",
			Subtitle:   "stock dividend",
			DeployDate: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
			Link:       "",
		},
	}, now.Add(-incomeEventsLookback), now)
	for _, want := range []string{
		"Income events from FireAnt",
		"06/05/2026 - 05/06/2026",
		"TCX - 25/05/2026: TCX: dividend",
		"stock dividend",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func installTradingIncomeEvents(t *testing.T, eventBody string, now time.Time) (*testutil.RecordingBot, storage.KVStore) {
	t.Helper()
	eventsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(eventBody))
	}))
	t.Cleanup(eventsSrv.Close)

	priceSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"stockSymbol":"TCX","matchedPrice":24500}}`))
	}))
	t.Cleanup(priceSrv.Close)

	rb := testutil.NewRecordingBot(t)
	kv := storage.NewMemoryKVStore()
	s := &state{
		kv:           kv,
		prices:       &PriceClient{HTTP: priceSrv.Client(), URL: priceSrv.URL},
		incomeEvents: &IncomeEventClient{HTTP: eventsSrv.Client(), URL: eventsSrv.URL},
		nowFn:        func() time.Time { return now },
	}
	cmd := modules.Command{
		Name:        "stock_income_events",
		Visibility:  modules.VisibilityPublic,
		Description: "x",
		Handler:     s.handleIncomeEvents,
	}
	reg := &modules.Registry{
		Modules:     []modules.Module{{Name: "stock", Commands: []modules.Command{cmd}}},
		AllCommands: map[string]modules.Command{cmd.Name: cmd},
	}
	modules.Install(rb.Bot, reg, modules.Auth{})
	return rb, kv
}

func TestHandleIncomeEvents_WithTicker(t *testing.T) {
	now := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	body := `[{"id":"1","label":"GDKHQ","date":"2026-05-25T00:00:00Z","title":"TCX: 25.5.2026, ngày GDKHQ trả cổ tức bằng cổ phiếu năm 2024 (tỷ lệ 5:1)"}]`
	rb, _ := installTradingIncomeEvents(t, body, now)

	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(1, "/stock_income_events TCX"))
	got := rb.LastSent().Text()
	for _, want := range []string{"Income events from FireAnt", "TCX - 25/05/2026", "trả cổ tức"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestHandleIncomeEvents_UsesHoldingsWhenTickerMissing(t *testing.T) {
	now := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	body := `[{"id":"1","label":"Cổ tức","date":"2026-05-25T00:00:00Z","title":"Holding event"}]`
	rb, kv := installTradingIncomeEvents(t, body, now)
	p := NewPortfolio(now.UnixMilli())
	p.AddAsset("TCX", 100)
	if err := SavePortfolio(context.Background(), kv, 7, p); err != nil {
		t.Fatalf("SavePortfolio: %v", err)
	}

	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/stock_income_events"))
	got := rb.LastSent().Text()
	if !strings.Contains(got, "TCX - 25/05/2026: Holding event") {
		t.Errorf("expected holding event reply; got:\n%s", got)
	}
}
