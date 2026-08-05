// Package lol serves LoL esports match schedules via lolesports.com's
// GraphQL gateway plus a daily push to subscribers.
//
// Endpoint: https://lolesports.com/api/gql — the same-origin proxy the
// lolesports.com web client uses. It accepts only persisted operations:
// the request carries no query text, just the operation name plus a
// pre-registered ID in the persistedQuery extension. The legacy
// esports-api.lolesports.com REST API and its public x-api-key were
// retired by Riot in 2026 (403 for everyone), so there is no key to ship.
//
// If Riot redeploys the frontend with a changed homeEvents operation, the
// registered ID rotates and the gateway answers PERSISTED_QUERY_NOT_IN_LIST.
// To refresh: load lolesports.com, find the webpack runtime's chunk-hash map
// for chunk 29 (the persisted-operations manifest, `loadManifest` in the
// bundle), download /_next/static/chunks/29.<hash>.js, and copy the `id`
// next to `"name":"homeEvents"`.
//
// Cache strategy: live-first fetches with a KV-backed 60-minute stale
// fallback for current schedule windows.
package lol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/storage"
)

const (
	apiURL = "https://lolesports.com/api/gql"
	// homeEventsID is the gateway-registered persisted-query ID for the web
	// client's homeEvents operation (schedule events by date window). Not a
	// hash we compute — the value comes from the frontend's operations
	// manifest and must be re-extracted if Riot rotates it (see package doc).
	homeEventsID = "7246add6f577cf30b304e651bf9e25fc6a41fe49aeafb0754c16b5778060fc0a"
	// The gateway rejects requests without Apollo client-awareness headers
	// ("No client headers set"), so we mirror the web client's name and send
	// our own version string.
	clientName    = "Esports Web"
	clientVersion = "miti99bot/0.1"
	userAgent     = "miti99bot/0.1 (https://t.me/miti99bot)"
	// pageSize is the events-per-page the gateway accepts; 100 covers a full
	// day in one page and a dense week in a handful.
	pageSize = 100
	// staleMaxAge: how long to fall back to a cached payload when the
	// upstream call fails outright.
	staleMaxAge = 60 * 60 * time.Second
	// httpTimeout: keep upstream calls bounded so a hung lolesports edge
	// can't hold a worker goroutine indefinitely.
	httpTimeout = 8 * time.Second
)

// Team is one side of a match. JSON shape matches the lolesports response.
// bson tags mirror the json names exactly: this tree is persisted inside
// cacheRecord, so the store must read those keys back verbatim — not the
// driver's lowercased default.
type Team struct {
	Name   string      `json:"name,omitempty" bson:"name,omitempty"`
	Code   string      `json:"code,omitempty" bson:"code,omitempty"`
	Image  string      `json:"image,omitempty" bson:"image,omitempty"`
	Result *TeamResult `json:"result,omitempty" bson:"result,omitempty"`
	Record *TeamRecord `json:"record,omitempty" bson:"record,omitempty"`
}

// TeamResult is a team's per-match outcome (named so tests and the bson decoder
// share one type).
type TeamResult struct {
	Outcome  string `json:"outcome,omitempty" bson:"outcome,omitempty"` // "win" or "loss"
	GameWins int    `json:"gameWins,omitempty" bson:"gameWins,omitempty"`
}

// TeamRecord is a team's running series record.
type TeamRecord struct {
	Wins   int `json:"wins,omitempty" bson:"wins,omitempty"`
	Losses int `json:"losses,omitempty" bson:"losses,omitempty"`
}

// League holds the league-section-header info on each event.
type League struct {
	Name  string `json:"name,omitempty" bson:"name,omitempty"`
	Slug  string `json:"slug,omitempty" bson:"slug,omitempty"`
	Image string `json:"image,omitempty" bson:"image,omitempty"`
}

// Strategy is the bestOf descriptor (Bo1, Bo3, Bo5).
type Strategy struct {
	Type  string `json:"type,omitempty" bson:"type,omitempty"`
	Count int    `json:"count,omitempty" bson:"count,omitempty"`
}

// Match is the inner match metadata.
type Match struct {
	ID       string   `json:"id,omitempty" bson:"id,omitempty"`
	Teams    []Team   `json:"teams,omitempty" bson:"teams,omitempty"`
	Strategy Strategy `json:"strategy,omitempty" bson:"strategy,omitempty"`
}

// ScheduleEvent is one upcoming or past match. State is "unstarted",
// "inProgress", or "completed". Type is set to "show" for pre/post-show
// segments which we filter out. This struct is the module's stable shape:
// formatters and the bson cache read it, so the GraphQL response is mapped
// into it rather than leaking the transport shape.
type ScheduleEvent struct {
	StartTime string `json:"startTime" bson:"startTime"`
	State     string `json:"state,omitempty" bson:"state,omitempty"`
	Type      string `json:"type,omitempty" bson:"type,omitempty"`
	BlockName string `json:"blockName,omitempty" bson:"blockName,omitempty"`
	League    League `json:"league,omitempty" bson:"league,omitempty"`
	Match     Match  `json:"match,omitempty" bson:"match,omitempty"`
}

// gqlEvent is one event as the gateway returns it. Teams live in matchTeams
// (not match.teams), so it converts into the stable ScheduleEvent shape.
type gqlEvent struct {
	StartTime  string `json:"startTime"`
	State      string `json:"state"`
	Type       string `json:"type"`
	BlockName  string `json:"blockName"`
	League     League `json:"league"`
	Match      Match  `json:"match"`
	MatchTeams []Team `json:"matchTeams"`
}

func (e gqlEvent) toScheduleEvent() ScheduleEvent {
	m := e.Match
	m.Teams = e.MatchTeams
	return ScheduleEvent{
		StartTime: e.StartTime,
		State:     e.State,
		Type:      e.Type,
		BlockName: e.BlockName,
		League:    e.League,
		Match:     m,
	}
}

// gqlResponse is the outer shape of a gateway response. GraphQL transports
// errors in-band on HTTP 200, so both branches must be checked.
type gqlResponse struct {
	Data struct {
		Esports struct {
			Events []gqlEvent `json:"events"`
			Pages  struct {
				Older string `json:"older,omitempty"`
				Newer string `json:"newer,omitempty"`
			} `json:"pages"`
		} `json:"esports"`
	} `json:"data"`
	Errors []struct {
		Message    string `json:"message"`
		Extensions struct {
			Code string `json:"code"`
		} `json:"extensions"`
	} `json:"errors"`
}

// cacheRecord is the store value: fetch timestamp + events. The Mongo store
// owns updatedAt separately; LoL uses ts for cache freshness in every backend.
type cacheRecord struct {
	Ts     int64           `json:"ts" bson:"ts"` // ms-since-epoch when fetched
	Events []ScheduleEvent `json:"events" bson:"events"`
}

// CacheStore is the typed store for schedule cache records.
type CacheStore = storage.DocStore[cacheRecord]

// Client is the lolesports API client. Default zero-value uses
// http.DefaultClient + http.DefaultTransport; tests inject a custom HTTP
// client (typically pointing at httptest.Server).
type Client struct {
	HTTP *http.Client
	URL  string // override for tests; empty falls back to apiURL
}

// httpClient returns the client to use, or a sensible default.
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

// gqlRequest is the persisted-operation request body: no query text, just
// the operation name, variables, and the registered ID.
type gqlRequest struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
	Extensions    struct {
		PersistedQuery struct {
			Version    int    `json:"version"`
			Sha256Hash string `json:"sha256Hash"`
		} `json:"persistedQuery"`
	} `json:"extensions"`
}

// fetchEventsPage retrieves one page of events for a date window. The
// gateway's eventDateStart/eventDateEnd are calendar dates with unspecified
// timezone semantics, so the window is padded a day on each side and callers
// filter to the exact [from, to) instants afterwards. pageToken continues a
// prior page's `pages.newer` cursor. Events come back ascending by startTime.
func (c *Client) fetchEventsPage(ctx context.Context, from, to time.Time, pageToken string) ([]ScheduleEvent, string, error) {
	vars := map[string]any{
		"hl":             "en-US",
		"sport":          []string{"lol"},
		"eventDateStart": from.UTC().AddDate(0, 0, -1).Format("2006-01-02"),
		"eventDateEnd":   to.UTC().AddDate(0, 0, 1).Format("2006-01-02"),
		"eventType":      "match",
		"pageSize":       pageSize,
	}
	if pageToken != "" {
		vars["pageToken"] = pageToken
	}
	reqBody := gqlRequest{OperationName: "homeEvents", Variables: vars}
	reqBody.Extensions.PersistedQuery.Version = 1
	reqBody.Extensions.PersistedQuery.Sha256Hash = homeEventsID
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", fmt.Errorf("lol encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL(), bytes.NewReader(payload))
	if err != nil {
		return nil, "", fmt.Errorf("lol build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apollographql-client-name", clientName)
	req.Header.Set("apollographql-client-version", clientVersion)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("lol do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("lol read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn("lol_fetch", "status", resp.StatusCode, "body", truncate(string(body), 500))
		return nil, "", fmt.Errorf("lol API HTTP %d", resp.StatusCode)
	}
	var page gqlResponse
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, "", fmt.Errorf("lol decode: %w", err)
	}
	if len(page.Errors) > 0 {
		first := page.Errors[0]
		// A rotated persisted-query ID is an operational event, not a blip:
		// every call fails until the constant is refreshed, so make the log
		// unmistakable (see package doc for the refresh procedure).
		if first.Extensions.Code == "PERSISTED_QUERY_NOT_IN_LIST" {
			log.Error("lol_persisted_query_rotated", "msg", first.Message)
		} else {
			log.Warn("lol_fetch", "gql_code", first.Extensions.Code, "gql_err", truncate(first.Message, 500))
		}
		return nil, "", fmt.Errorf("lol API gql error %s: %s", first.Extensions.Code, first.Message)
	}
	// Drop pre/post-show segments; they aren't matches.
	out := make([]ScheduleEvent, 0, len(page.Data.Esports.Events))
	for _, e := range page.Data.Esports.Events {
		if e.Type == "show" {
			continue
		}
		out = append(out, e.toScheduleEvent())
	}
	return out, page.Data.Esports.Pages.Newer, nil
}

// fetchEventsInRange covers [from, to) with a date-bounded query, following
// `pages.newer` cursors when a window holds more than one page. Page budget
// bounds upstream calls during dense weeks.
func (c *Client) fetchEventsInRange(ctx context.Context, from, to time.Time, maxPages int) ([]ScheduleEvent, error) {
	if maxPages <= 0 {
		maxPages = 8
	}
	var collected []ScheduleEvent
	pageToken := ""
	for page := 0; page < maxPages; page++ {
		events, newer, err := c.fetchEventsPage(ctx, from, to, pageToken)
		if err != nil {
			return nil, err
		}
		collected = append(collected, events...)
		if newer == "" {
			break
		}
		pageToken = newer
	}

	out := make([]ScheduleEvent, 0, len(collected))
	for _, e := range collected {
		t, err := time.Parse(time.RFC3339, e.StartTime)
		if err != nil {
			continue
		}
		if !t.Before(from) && t.Before(to) {
			out = append(out, e)
		}
	}
	return out, nil
}

// cacheKey is `matches:<from-iso>:<to-iso>` — a stable key for a date range.
func cacheKey(from, to time.Time) string {
	return "matches:" + from.UTC().Format(time.RFC3339) + ":" + to.UTC().Format(time.RFC3339)
}

// GetEventsLive fetches the requested range directly from upstream without
// consulting or writing the fallback cache.
func (c *Client) GetEventsLive(ctx context.Context, from, to time.Time) ([]ScheduleEvent, error) {
	return c.fetchEventsInRange(ctx, from, to, 3)
}

// GetEventsWithFallback is live-first. It always tries upstream, writes a
// successful response to cache, and uses a recent cached response only when the
// upstream call fails.
func (c *Client) GetEventsWithFallback(ctx context.Context, cache CacheStore, from, to time.Time) ([]ScheduleEvent, error) {
	key := cacheKey(from, to)
	now := time.Now().UTC().UnixMilli()

	cached, _, cacheErr := cache.Get(ctx, key)
	hasCached := cacheErr == nil

	events, fetchErr := c.GetEventsLive(ctx, from, to)
	if fetchErr == nil {
		rec := cacheRecord{Ts: now, Events: events}
		if err := cache.Put(ctx, key, rec); err != nil {
			log.Warn("lol_kv_put_fail", "err", err)
		}
		return events, nil
	}

	// Upstream failed — fall back to stale cache if recent enough.
	if hasCached && now-cached.Ts < staleMaxAge.Milliseconds() {
		log.Warn("lol_stale_fallback", "err", fetchErr)
		return cached.Events, nil
	}
	return nil, fetchErr
}

// truncate clips a string to a rune-boundary prefix whose byte length is
// <= maxLen, appending "..." if cut. Keeps log output bounded — lolesports
// occasionally returns multi-MB error pages, and team/player names mix in
// Korean/Chinese characters that a raw byte slice would split mid-codepoint
// (producing replacement glyphs in CloudWatch).
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := maxLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}

// ErrEmptyResult is reserved for explicit "no events" scenarios where the
// fetch succeeded but returned zero matches. Currently unused outside tests
// but kept exported so callers can distinguish from network errors.
var ErrEmptyResult = errors.New("lol: no events in range")
