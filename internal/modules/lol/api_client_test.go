package lol

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiennm99/miti99bot/internal/storage"
)

// newCacheStore returns a fresh typed cache store backed by an in-memory
// collection — the per-test equivalent of what the factory wires in production.
func newCacheStore() CacheStore {
	return storage.Typed[cacheRecord](storage.NewMemoryProvider().Collection("lol"))
}

// mkServer spins an httptest.Server returning the supplied JSON body for
// every page request. callCount counts upstream hits so cache tests can
// assert "1 fetch, then no more".
func mkServer(t *testing.T, body string) (*httptest.Server, *int32) {
	t.Helper()
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

const sampleBody = `{
  "data": {
    "esports": {
      "events": [
        {
          "startTime": "2026-05-09T05:00:00Z",
          "state": "unstarted",
          "type": "match",
          "league": {"slug": "lck", "name": "LCK"},
          "match": {"strategy":{"count":3}},
          "matchTeams": [{"code":"T1"},{"code":"GEN"}]
        },
        {
          "startTime": "2026-05-09T08:00:00Z",
          "state": "unstarted",
          "type": "show",
          "league": {"slug": "lck", "name": "LCK"},
          "match": {"strategy":{}},
          "matchTeams": []
        }
      ],
      "pages": {"newer": null}
    }
  }
}`

func TestGetEventsWithFallback_FirstHitFetchesUpstreamAndCaches(t *testing.T) {
	srv, count := mkServer(t, sampleBody)
	c := &Client{HTTP: srv.Client(), URL: srv.URL}
	cache := newCacheStore()
	from := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	events, err := c.GetEventsWithFallback(context.Background(), cache, from, to)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("events = %d, want 1 (show filtered out)", len(events))
	}
	if events[0].League.Slug != "lck" {
		t.Errorf("event slug = %q, want lck", events[0].League.Slug)
	}
	if got := events[0].Match.Teams; len(got) != 2 || got[0].Code != "T1" {
		t.Errorf("teams not mapped from matchTeams: %+v", got)
	}
	if atomic.LoadInt32(count) != 1 {
		t.Errorf("upstream calls = %d, want 1", *count)
	}
	cached, _, err := cache.Get(context.Background(), cacheKey(from, to))
	if err != nil {
		t.Fatalf("load cached record: %v", err)
	}
	if cached.Ts <= 0 {
		t.Fatalf("cached ts = %d, want positive", cached.Ts)
	}
}

func TestGetEventsWithFallback_AlwaysFetchesWhenUpstreamAvailable(t *testing.T) {
	srv, count := mkServer(t, sampleBody)
	c := &Client{HTTP: srv.Client(), URL: srv.URL}
	cache := newCacheStore()
	from := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	if _, err := c.GetEventsWithFallback(context.Background(), cache, from, to); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetEventsWithFallback(context.Background(), cache, from, to); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(count); got != 2 {
		t.Errorf("upstream calls = %d, want 2 (live-first should refetch)", got)
	}
}

func TestGetEventsWithFallback_StaleFallback(t *testing.T) {
	// Prime the typed cache store with a stale-but-still-fresh-enough record.
	cache := newCacheStore()
	from := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	staleEvents := []ScheduleEvent{
		{StartTime: "2026-05-09T05:00:00Z", League: League{Slug: "lck", Name: "LCK"}},
	}
	// 10 minutes ago — past the 120s fresh window but well inside 60-min stale.
	staleTs := time.Now().UTC().Add(-10 * time.Minute).UnixMilli()
	if err := cache.Put(context.Background(), cacheKey(from, to), cacheRecord{Ts: staleTs, Events: staleEvents}); err != nil {
		t.Fatal(err)
	}

	// Upstream errors — server returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), URL: srv.URL}

	got, err := c.GetEventsWithFallback(context.Background(), cache, from, to)
	if err != nil {
		t.Fatalf("stale fallback should succeed: %v", err)
	}
	if len(got) != 1 || got[0].League.Slug != "lck" {
		t.Errorf("stale fallback returned wrong events: %+v", got)
	}
}

func TestGetEventsWithFallback_HardFailureWhenNoCache(t *testing.T) {
	cache := newCacheStore()
	from := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), URL: srv.URL}

	_, err := c.GetEventsWithFallback(context.Background(), cache, from, to)
	if err == nil {
		t.Errorf("expected error when upstream fails AND no cache")
	}
}

func TestGetEventsLive_DoesNotWriteCache(t *testing.T) {
	srv, _ := mkServer(t, sampleBody)
	c := &Client{HTTP: srv.Client(), URL: srv.URL}
	cache := newCacheStore()
	from := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	if _, err := c.GetEventsLive(context.Background(), from, to); err != nil {
		t.Fatalf("live fetch: %v", err)
	}
	if _, _, err := cache.Get(context.Background(), cacheKey(from, to)); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("live fetch cache entry err = %v, want ErrNotFound", err)
	}
}

func TestFetchEventsPage_DropsShowEvents(t *testing.T) {
	srv, _ := mkServer(t, sampleBody)
	c := &Client{HTTP: srv.Client(), URL: srv.URL}

	from := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	events, _, err := c.fetchEventsPage(context.Background(), from, from.Add(24*time.Hour), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Type == "show" {
			t.Errorf("show event leaked: %+v", e)
		}
	}
}

func TestFetchEventsPage_NonJSONErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), URL: srv.URL}
	from := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	_, _, err := c.fetchEventsPage(context.Background(), from, from.Add(24*time.Hour), "")
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("non-JSON should produce decode error; got %v", err)
	}
}

// TestFetchEventsPage_GraphQLErrorIsFetchError guards the in-band error path:
// the gateway answers HTTP 200 with an `errors` array (e.g. when Riot rotates
// the persisted-query ID), and that must surface as a fetch failure — not be
// cached as an empty schedule.
func TestFetchEventsPage_GraphQLErrorIsFetchError(t *testing.T) {
	body := `{"errors":[{"message":"Persisted query 'x' not found in the persisted query list","extensions":{"code":"PERSISTED_QUERY_NOT_IN_LIST"}}]}`
	srv, _ := mkServer(t, body)
	c := &Client{HTTP: srv.Client(), URL: srv.URL}
	from := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	_, _, err := c.fetchEventsPage(context.Background(), from, from.Add(24*time.Hour), "")
	if err == nil || !strings.Contains(err.Error(), "PERSISTED_QUERY_NOT_IN_LIST") {
		t.Errorf("gql error should surface as fetch error with code; got %v", err)
	}
}

// requestVars decodes the persisted-operation POST body a test server
// received and returns its variables map.
func requestVars(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var req struct {
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Errorf("decode request body: %v", err)
	}
	return req.Variables
}

// TestFetchEventsInRange_WalksNewerPages guards multi-page windows: when a
// date window holds more events than one page, the fetcher must follow
// `pages.newer` cursors until the gateway reports no further page.
//
// Server simulates 2 pages:
//   - no pageToken       → Mon May 18 + Tue May 19 (returns newer=page2)
//   - pageToken=page2    → Wed May 20 (no newer)
func TestFetchEventsInRange_WalksNewerPages(t *testing.T) {
	page1Body := `{"data":{"esports":{"events":[
		{"startTime":"2026-05-18T05:00:00Z","state":"completed","type":"match","league":{"slug":"lck","name":"LCK"},"match":{"strategy":{"count":3}},"matchTeams":[{"code":"KT"},{"code":"DK"}]},
		{"startTime":"2026-05-19T05:00:00Z","state":"completed","type":"match","league":{"slug":"lck","name":"LCK"},"match":{"strategy":{"count":3}},"matchTeams":[{"code":"NS"},{"code":"BRO"}]}
	],"pages":{"newer":"page2","older":null}}}}`
	page2Body := `{"data":{"esports":{"events":[
		{"startTime":"2026-05-20T05:00:00Z","state":"completed","type":"match","league":{"slug":"lck","name":"LCK"},"match":{"strategy":{"count":3}},"matchTeams":[{"code":"GEN"},{"code":"T1"}]}
	],"pages":{"newer":null,"older":"page1"}}}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := requestVars(t, r)
		token, _ := vars["pageToken"].(string)
		switch token {
		case "":
			_, _ = w.Write([]byte(page1Body))
		case "page2":
			_, _ = w.Write([]byte(page2Body))
		default:
			t.Errorf("unexpected pageToken %q", token)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), URL: srv.URL}
	from := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	to := from.Add(7 * 24 * time.Hour)

	events, err := c.fetchEventsInRange(context.Background(), from, to, 0)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("events = %d, want 3 (Mon, Tue, Wed); got days=%v", len(events), eventDays(events))
	}
}

// TestFetchEventsInRange_FiltersToExactWindow guards the client-side [from, to)
// filter: the request pads the date window a day each side (the gateway's date
// params have fuzzy timezone semantics), so events outside the exact instants
// must be dropped locally.
func TestFetchEventsInRange_FiltersToExactWindow(t *testing.T) {
	body := `{"data":{"esports":{"events":[
		{"startTime":"2026-05-17T23:00:00Z","state":"completed","type":"match","league":{"slug":"lck","name":"LCK"},"match":{"strategy":{"count":3}},"matchTeams":[{"code":"KT"},{"code":"DK"}]},
		{"startTime":"2026-05-18T05:00:00Z","state":"unstarted","type":"match","league":{"slug":"lck","name":"LCK"},"match":{"strategy":{"count":3}},"matchTeams":[{"code":"GEN"},{"code":"T1"}]},
		{"startTime":"2026-05-19T00:00:00Z","state":"unstarted","type":"match","league":{"slug":"lck","name":"LCK"},"match":{"strategy":{"count":3}},"matchTeams":[{"code":"NS"},{"code":"BRO"}]}
	],"pages":{"newer":null}}}}`
	srv, _ := mkServer(t, body)
	c := &Client{HTTP: srv.Client(), URL: srv.URL}

	from := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	events, err := c.fetchEventsInRange(context.Background(), from, to, 0)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(events) != 1 || events[0].Match.Teams[0].Code != "GEN" {
		t.Errorf("window filter wrong, got %v", eventDays(events))
	}
}

func eventDays(events []ScheduleEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.StartTime)
	}
	return out
}

// truncate is internal but worth a smoke test — log payloads use it.
func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q, want unchanged", got)
	}
	got := truncate("a long enough string", 5)
	if got != "a lon..." {
		t.Errorf("truncate = %q, want 'a lon...'", got)
	}
}

// Smoke: ErrEmptyResult is exported and distinct from generic errors.
func TestErrEmptyResult_Identity(t *testing.T) {
	if errors.Is(ErrEmptyResult, errors.New("other")) {
		t.Error("ErrEmptyResult should not match arbitrary errors")
	}
}
