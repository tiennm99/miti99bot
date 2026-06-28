package wc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tiennm99/miti99bot/internal/log"
)

const (
	apiURL       = "https://api.football-data.org/v4/competitions/WC/matches"
	userAgent    = "miti99bot/0.1 (https://t.me/miti99bot)"
	worldCupYear = "2026"

	staleMaxAge = 24 * time.Hour
	httpTimeout = 8 * time.Second
)

// ErrNotConfigured is returned when no football-data.org token is available.
var ErrNotConfigured = errors.New("wc: WC_FOOTBALL_DATA_TOKEN not set")

// Client talks to football-data.org. Tests inject URL/HTTP/Token.
type Client struct {
	HTTP  *http.Client
	URL   string
	Token string
}

// NewClientFromEnv builds the production World Cup API client.
func NewClientFromEnv() *Client {
	return &Client{Token: strings.TrimSpace(os.Getenv("WC_FOOTBALL_DATA_TOKEN"))}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: httpTimeout}
}

func (c *Client) baseURL() string {
	if c.URL != "" {
		return c.URL
	}
	return apiURL
}

func (c *Client) token() string {
	return strings.TrimSpace(c.Token)
}

func (c *Client) fetchAllMatches(ctx context.Context) ([]Match, error) {
	if c.token() == "" {
		return nil, ErrNotConfigured
	}
	u, err := url.Parse(c.baseURL())
	if err != nil {
		return nil, fmt.Errorf("wc parse url: %w", err)
	}
	q := u.Query()
	q.Set("season", worldCupYear)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("wc build request: %w", err)
	}
	req.Header.Set("X-Auth-Token", c.token())
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("wc do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("wc read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn("wc_fetch", "status", resp.StatusCode, "body", truncateLog(string(body), 500))
		return nil, fmt.Errorf("wc API HTTP %d", resp.StatusCode)
	}

	var out matchesResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("wc decode: %w", err)
	}
	sortMatches(out.Matches)
	return out.Matches, nil
}

func cacheKey() string {
	return "matches:" + worldCupYear
}

// GetMatchesCached returns matches in [from, to). It is live-first: every call
// attempts football-data.org so TBD fixtures and live scores update quickly.
// The stored full-tournament payload is only a stale fallback when upstream
// fails.
func (c *Client) GetMatchesCached(ctx context.Context, cache CacheStore, from, to time.Time) ([]Match, error) {
	now := time.Now().UTC().UnixMilli()
	cached, _, cacheErr := cache.Get(ctx, cacheKey())
	hasCached := cacheErr == nil

	matches, fetchErr := c.fetchAllMatches(ctx)
	if fetchErr == nil {
		rec := cacheRecord{Ts: now, Matches: matches}
		if err := cache.Put(ctx, cacheKey(), rec); err != nil {
			log.Warn("wc_cache_put_fail", "err", err)
		}
		return filterMatches(matches, from, to), nil
	}

	if hasCached && now-cached.Ts < staleMaxAge.Milliseconds() {
		log.Warn("wc_stale_fallback", "err", fetchErr)
		return filterMatches(cached.Matches, from, to), nil
	}
	return nil, fetchErr
}

func filterMatches(matches []Match, from, to time.Time) []Match {
	out := make([]Match, 0, len(matches))
	for _, m := range matches {
		t, err := time.Parse(time.RFC3339, m.UTCDate)
		if err != nil {
			continue
		}
		if !t.Before(from) && t.Before(to) {
			out = append(out, m)
		}
	}
	sortMatches(out)
	return out
}

func sortMatches(matches []Match) {
	sort.SliceStable(matches, func(i, j int) bool {
		ti, errI := time.Parse(time.RFC3339, matches[i].UTCDate)
		tj, errJ := time.Parse(time.RFC3339, matches[j].UTCDate)
		if errI != nil || errJ != nil {
			return matches[i].ID < matches[j].ID
		}
		if ti.Equal(tj) {
			return matches[i].ID < matches[j].ID
		}
		return ti.Before(tj)
	})
}

func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := maxLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}
