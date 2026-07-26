---
phase: 2
title: "Cargo Client"
status: pending
effort: "M"
priority: P2
dependencies: [1]
---

# Phase 2: Cargo Client

## Overview

A minimal read-only client for Leaguepedia's Cargo API returning finished-match scores in a UTC time window. Mirrors the existing `Client` shape in `api_client.go` so tests can inject an `httptest.Server`.

## Requirements

Functional:
- Fetch `MatchSchedule` rows for `[from, to)` UTC
- Return team names, series scores, Bo count, scheduled time
- Skip rows without a usable score (unplayed / in-progress future matches)

Non-functional:
- Disabled by default; single env var enables
- Bounded timeout, no retries (a miss is not worth a second request against a throttling host)
- Descriptive User-Agent with contact URL — required by wiki API etiquette
- Never returns an error the caller must surface to the user; caller treats all failures as "no data"

## Architecture

New file `internal/modules/lol/leaguepedia_client.go`.

```go
// leaguepediaEnabledEnv gates the whole feature. Absent or not "1"/"true" → no
// requests are ever made.
const leaguepediaEnabledEnv = "LOL_LEAGUEPEDIA_ENABLED"

const (
    leaguepediaURL = "https://lol.fandom.com/api.php"
    // leaguepediaUserAgent identifies the bot per wiki API etiquette; Fandom
    // throttles anonymous traffic aggressively and an honest UA with contact
    // info is the minimum courtesy for a free source.
    leaguepediaUserAgent = "miti99bot/0.1 (+https://github.com/tiennm99/miti99bot)"
    leaguepediaTimeout   = 6 * time.Second
    // leaguepediaRowLimit caps a day/week window. A dense week across all
    // leagues (incl. amateur tiers we filter out) can exceed 100 rows.
    leaguepediaRowLimit = 500
)

// SeriesResult is one finished series as Leaguepedia records it. Team names are
// full names ("Movistar KOI"), matching Riot's Team.Name — see Phase 3 join.
type SeriesResult struct {
    StartTime time.Time
    Team1     string
    Team2     string
    Team1Wins int
    Team2Wins int
}

type leaguepediaClient struct {
    HTTP *http.Client
    URL  string // test override; empty → leaguepediaURL
}

// leaguepediaEnabled reports whether the operator opted in.
func leaguepediaEnabled() bool

// FetchResults returns finished series in [from, to). Returns (nil, nil) when
// disabled — callers treat empty and error identically, so there is no
// "disabled" error to special-case.
func (c *leaguepediaClient) FetchResults(ctx context.Context, from, to time.Time) ([]SeriesResult, error)
```

### Cargo response shape

Rows arrive wrapped, with **spaces** in field names (Cargo replaces `_`):

```json
{"cargoquery":[{"title":{
  "DateTime UTC":"2026-07-25 17:30:00",
  "Team1":"Movistar KOI","Team2":"Karmine Corp",
  "Team1Score":"2","Team2Score":"0",
  "Winner":"1","BestOf":"3"}}]}
```

Two decoding traps, both must be handled:
- `DateTime UTC` and `Team1Score` are **strings**, not numbers → decode into `string` and `strconv.Atoi`
- Errors come back HTTP 200 with `{"error":{"info":"You've exceeded your rate limit..."}}` → must check for an `error` key, not just the status code

Timestamp format is `2006-01-02 15:04:05`, no zone suffix, always UTC.

## Related Code Files

- Create: `internal/modules/lol/leaguepedia_client.go`
- Create: `internal/modules/lol/leaguepedia_client_test.go`
- Reference (patterns to copy, do not modify): `internal/modules/lol/api_client.go` (Client/HTTP/URL shape, `truncate` for log bounding), `internal/modules/misc/wheelofnames_api_client.go:62` (env const + `os.Getenv`)

## Implementation Steps

1. Add env gate `leaguepediaEnabled()` — `strings.TrimSpace(os.Getenv(...))` matched against `"1"`/`"true"`, so an empty or garbage value is safely off
2. Add `httpClient()`/`baseURL()` helpers mirroring `api_client.go:133-145`
3. Build the query with `url.Values` — `action=cargoquery`, `format=json`, `tables=MatchSchedule=MS`, the field list, `where` bounded by `from`/`to`, `order_by`, `limit`
4. Set `User-Agent` and `Accept`; issue GET with the request context
5. Decode into a struct with an `Error *struct{ Info string }` field; if present, log at warn with `truncate(...)` and return an error
6. Map rows → `[]SeriesResult`, skipping any row where either score fails to parse or `Team1Wins == 0 && Team2Wins == 0` (unplayed — same "absent vs zero" trap as the original bug; do not import a fabricated 0-0 from a *second* source)
7. Parse `DateTime UTC` with `time.ParseInLocation(..., time.UTC)`; skip unparseable rows

## Tests

All via `httptest.Server`, no network:

- [ ] Happy path: 3 rows → 3 `SeriesResult` with correct scores and UTC times
- [ ] Disabled env → returns `(nil, nil)` and **makes zero HTTP requests** (assert with a request counter on the test server)
- [ ] Rate-limit body (`HTTP 200` + `{"error":{"info":"..."}}`) → error, no partial results
- [ ] Non-2xx status → error
- [ ] Malformed JSON → error
- [ ] Row with `Team1Score:""` → skipped, siblings still returned
- [ ] Row with `Team1Score:"0", Team2Score:"0"` → skipped (unplayed guard)
- [ ] Row with unparseable `DateTime UTC` → skipped, siblings returned
- [ ] Request assertions: `where` contains both bounds, UA is set, `limit` present
- [ ] Context cancellation propagates

## Success Criteria

- [ ] `go test ./internal/modules/lol/ -run Leaguepedia` green
- [ ] Zero network access in tests
- [ ] Zero HTTP requests when disabled
- [ ] No change to any existing file — this phase is purely additive
- [ ] `go vet ./...` clean

## Risk Assessment

**Silent schema drift.** Cargo field names with spaces are easy to typo into always-empty strings, which would look like "Leaguepedia has no data" rather than a bug. Mitigate by asserting exact parsed values in the happy-path test using a fixture captured verbatim in Phase 1, not hand-written JSON.

**Importing another fabricated 0-0.** Step 6's guard is the whole point — this plan exists because a 0-0 was trusted once already.

**Throttling in production.** Out of scope here (client makes one request when asked); Phase 4 owns call-site frequency.
