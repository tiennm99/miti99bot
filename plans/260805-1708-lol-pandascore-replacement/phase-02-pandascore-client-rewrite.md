---
phase: 2
title: PandaScore client rewrite
status: completed
effort: 'M — transport swap + fixtures, same blast radius as b76d0ca'
dependencies:
  - 1
---

# Phase 2: PandaScore client rewrite

## Overview

Swap the transport in `internal/modules/lol/api_client.go` from the gql
persisted-query client to PandaScore REST, keeping `ScheduleEvent` and every
consumer (`format.go`, `handlers.go`, `cron.go`, cache) unchanged. Mirrors
the b76d0ca rewrite: transport + mapping only.

## Requirements

- Functional: same rendered output for day/week windows; scores on finished
  series; major-league filtering intact via slug mapping from Phase 1.
- Non-functional: no live calls in tests; bounded timeouts (keep 8s); token
  never logged; stale-cache fallback (60 min) preserved.

## Architecture

```
GetEventsWithFallback / GetEventsLive          (unchanged)
        │
        ▼
fetchEventsInRange(ctx, from, to, maxPages)    (page-walk, [from,to) filter — unchanged shape)
        │
        ▼
fetchEventsPage(ctx, from, to, page)           (NEW: GET api.pandascore.co/lol/matches
        │                                        range[begin_at]=from,to & sort=begin_at
        ▼                                        & per_page=100 & page=N
psMatch.toScheduleEvent()                        Authorization: Bearer $LOL_PANDASCORE_TOKEN)
        │
        ▼
ScheduleEvent (existing struct, bson cache shape untouched)
```

Mapping (finalize against Phase 1 findings):

| ScheduleEvent | PandaScore |
|---|---|
| StartTime | `begin_at` (RFC3339) |
| State | `not_started→unstarted`, `running→inProgress`, `finished→completed`; drop canceled/postponed per Phase 1 |
| BlockName | source decided in Phase 1 (candidate `tournament.name`) |
| League.Slug/Name/Image | PandaScore league mapped → canonical slug via table; name/image passthrough |
| Match.Strategy.Count | `number_of_games` (Type: "bestOf") |
| Match.Teams[].Code/Name/Image | `opponents[].opponent.acronym/name/image_url` |
| Match.Teams[].Result | `results[]` matched to opponent by `team_id`; GameWins=score; Outcome win/loss from `winner_id` when `finished` |

## Related Code Files

- Modify: `internal/modules/lol/api_client.go` (transport, psMatch structs,
  mapping, league slug table, token from env)
- Modify: `internal/modules/lol/api_client_test.go` (fixtures → PandaScore
  shape; keep test matrix: cache-first, live-first, stale fallback, hard
  failure, page walk, exact-window filter, non-JSON error; add: token-missing
  error, canceled-match dropped, results/winner mapping)
- Modify: `internal/modules/lol/handlers_test.go` (`todayBody`/`futureBody`
  fixtures → PandaScore shape)
- No changes: `format.go`, `handlers.go`, `cron.go`, `subscribers.go`,
  `lol.go` wiring (Client zero-value still works; token read at request time)

## Implementation Steps

1. Define `psMatch`/`psOpponent`/`psResult`/`psLeague` structs + 
   `toScheduleEvent()` with the mapping table above.
2. Replace `fetchEventsPage`: build URL with `range[begin_at]`, `sort`,
   `per_page`, `page`; Bearer header from `LOL_PANDASCORE_TOKEN` (read via
   `os.Getenv` at call time, `strings.TrimSpace`d); keep `truncate`d body
   logging on non-2xx; distinct log key `lol_token_missing` when env empty
   (return error without calling upstream).
3. Rework `fetchEventsInRange` pagination: increment `page` while a full page
   (`len==per_page`) returned, capped by maxPages; keep exact `[from,to)`
   client-side filter (PandaScore range bounds semantics per Phase 1).
4. League slug mapping table (PandaScore slug → canonical) as package-level
   map from Phase 1 findings; unmapped leagues pass through their PandaScore
   slug (FilterMajor drops them naturally).
5. Update package doc comment: endpoint, auth, quota, mapping rationale,
   token env var.
6. Rewrite test fixtures in PandaScore shape (raw JSON arrays — note:
   `/lol/matches` returns a top-level array, not an envelope, per docs;
   confirm in Phase 1). Keep httptest pattern; assert Bearer header + query
   params in a request-inspection test.
7. Handle 401/403 (bad token) and 429 (quota) with clear log keys
   (`lol_fetch` with status suffices; verify stale cache covers).
8. `gofmt`, `go test ./internal/modules/lol/`, then full gates.

## Success Criteria

- [ ] All existing test scenarios pass with PandaScore fixtures
- [ ] New tests: token-missing, canceled dropped, score/winner mapping, Bearer
      header + range params asserted
- [ ] `rg lolesports internal/modules/lol` → only doc-comment history note or
      nothing
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` green

## Risk Assessment

- Top-level array vs envelope mismatch → caught by Phase 1 recipe.
- Opponent order vs results order mismatch → always join `results` by
  `team_id`, never by index; test covers reversed order.
- Token leakage → never log request URL if token-in-URL auth is used; use
  Bearer header only.
