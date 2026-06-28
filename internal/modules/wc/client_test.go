package wc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiennm99/miti99bot/internal/storage"
)

func newCacheStore() CacheStore {
	return storage.Typed[cacheRecord](storage.NewMemoryProvider().Collection("wc"))
}

func mkServer(t *testing.T, body string) (*httptest.Server, *int32) {
	t.Helper()
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		if got := r.Header.Get("X-Auth-Token"); got != "secret-token" {
			t.Errorf("X-Auth-Token = %q, want secret-token", got)
		}
		if got := r.URL.Query().Get("season"); got != worldCupYear {
			t.Errorf("season = %q, want %s", got, worldCupYear)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

const sampleMatchesBody = `{
  "matches": [
    {
      "id": 1,
      "utcDate": "2026-06-12T13:00:00Z",
      "status": "TIMED",
      "stage": "GROUP_STAGE",
      "group": "GROUP_A",
      "venue": "Estadio Azteca",
      "homeTeam": {"name": "Mexico", "shortName": "Mexico", "tla": "MEX"},
      "awayTeam": {"name": "South Africa", "shortName": "South Africa", "tla": "RSA"},
      "score": {"winner": null, "fullTime": {"home": null, "away": null}}
    },
    {
      "id": 2,
      "utcDate": "2026-06-13T13:00:00Z",
      "status": "TIMED",
      "stage": "GROUP_STAGE",
      "group": "GROUP_B",
      "homeTeam": {"tla": "CAN"},
      "awayTeam": {"tla": "SUI"},
      "score": {"winner": null, "fullTime": {"home": null, "away": null}}
    }
  ]
}`

func TestGetMatchesCached_FetchesAndFilters(t *testing.T) {
	srv, count := mkServer(t, sampleMatchesBody)
	c := &Client{HTTP: srv.Client(), URL: srv.URL, Token: "secret-token"}
	cache := newCacheStore()
	from := time.Date(2026, 6, 12, 0, 0, 0, 0, IctLocation).UTC()
	to := addDays(from, 1)

	matches, err := c.GetMatchesCached(context.Background(), cache, from, to)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(matches) != 1 || matches[0].HomeTeam.TLA != "MEX" {
		t.Fatalf("matches = %+v, want only MEX match", matches)
	}
	if got := atomic.LoadInt32(count); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
}

func TestGetMatchesCached_AlwaysRefetchesWhenProviderIsAvailable(t *testing.T) {
	var count int32
	firstBody := `{"matches":[{"id":1,"utcDate":"2026-06-12T13:00:00Z","status":"TIMED","homeTeam":{"tla":"MEX"},"awayTeam":{"tla":"RSA"}}]}`
	secondBody := `{"matches":[{"id":1,"utcDate":"2026-06-12T13:00:00Z","status":"TIMED","homeTeam":{"tla":"BRA"},"awayTeam":{"tla":"RSA"}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&count, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(firstBody))
			return
		}
		_, _ = w.Write([]byte(secondBody))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), URL: srv.URL, Token: "secret-token"}
	cache := newCacheStore()

	firstFrom := time.Date(2026, 6, 12, 0, 0, 0, 0, IctLocation).UTC()
	if _, err := c.GetMatchesCached(context.Background(), cache, firstFrom, addDays(firstFrom, 1)); err != nil {
		t.Fatal(err)
	}
	matches, err := c.GetMatchesCached(context.Background(), cache, firstFrom, addDays(firstFrom, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].HomeTeam.TLA != "BRA" {
		t.Fatalf("second fetch = %+v, want refreshed BRA match", matches)
	}
	if got := atomic.LoadInt32(&count); got != 2 {
		t.Fatalf("upstream calls = %d, want 2 live fetches", got)
	}
}

func TestGetMatchesCached_StaleFallback(t *testing.T) {
	cache := newCacheStore()
	from := time.Date(2026, 6, 12, 0, 0, 0, 0, IctLocation).UTC()
	stale := []Match{{ID: 9, UTCDate: "2026-06-12T13:00:00Z", HomeTeam: Team{TLA: "MEX"}}}
	rec := cacheRecord{Ts: time.Now().UTC().Add(-time.Hour).UnixMilli(), Matches: stale}
	if err := cache.Put(context.Background(), cacheKey(), rec); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), URL: srv.URL, Token: "secret-token"}

	matches, err := c.GetMatchesCached(context.Background(), cache, from, addDays(from, 1))
	if err != nil {
		t.Fatalf("stale fallback: %v", err)
	}
	if len(matches) != 1 || matches[0].ID != 9 {
		t.Fatalf("matches = %+v, want stale match", matches)
	}
}

func TestGetMatchesCached_MissingToken(t *testing.T) {
	c := &Client{Token: ""}
	from := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	_, err := c.GetMatchesCached(context.Background(), newCacheStore(), from, addDays(from, 1))
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestFetchAllMatches_NonJSONErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), URL: srv.URL, Token: "secret-token"}
	_, err := c.fetchAllMatches(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("err = %v, want decode error", err)
	}
}
