# Debug: LoL schedule fetch 403 Forbidden — root cause

Date: 2026-08-05 | Module: `internal/modules/lol` | Status: root cause confirmed, no code changed

## Symptom

```
WARN lol_fetch status=403 body={"message":"Forbidden"}
ERROR lol_fetch_fail err="lol API HTTP 403"
```

Every call to `https://esports-api.lolesports.com/persisted/gw/getSchedule` fails; stale cache (60 min) expires, so `/lol` commands and daily push break.

## Root cause

Riot revoked/rotated the long-standing public web-client API key hardcoded at `internal/modules/lol/api_client.go:34` (`0TvQnueqKa5mxJntVWt0w4LpLfEkrV1Ta8rQBb9Z`). No replacement public key exists: the rebuilt lolesports.com no longer ships a key to the browser at all.

## Evidence

1. Reproduced from this host: `getSchedule` with the key → HTTP 403, body `{"message":"Forbidden"}`, headers `x-amzn-errortype: ForbiddenException` + `x-amz-apigw-id` present. Request reached AWS API Gateway and Gateway itself rejected the key — not a CloudFront/WAF/IP edge block (an edge block would not carry `x-amz-apigw-id`).
2. Same 403 with no key, and with browser UA + `Origin`/`Referer: lolesports.com` — rules out UA/referer/origin gating; the key itself is dead.
3. Downloaded all 49 JS chunks from current lolesports.com (Next.js on Netlify). No `esports-api.lolesports.com` reference, no embedded key. Bundle shows Apollo client with `uri: () => ESPORTS_EDGE_URL ?? "/api/gql"` and key injected server-side from `process.env.ESPORTS_API_KEY` — key moved behind their BFF, never sent to browsers.
4. Probed `POST https://lolesports.com/api/gql` anonymously → HTTP 400 `PERSISTED_QUERY_ID_REQUIRED`: proxy is publicly reachable but accepts only pre-registered persisted-query IDs, no freeform GraphQL.

## Why now

Riot migrated lolesports.com to a new Next.js/GraphQL stack; the legacy `persisted/gw` REST API's public key was revoked as part of that migration. The code comment at `api_client.go:5-7` ("if Riot ever rotates it, lift the new value from their public JS bundle") no longer works — there is nothing to lift.

## Fix options (not implemented)

1. **Call `lolesports.com/api/gql` with persisted-query IDs** — sniff the ID + variables the live site sends for its schedule query (browser devtools), replay with `extensions.persistedQuery.sha256Hash`. Anonymous access works. Fragile: IDs change on each frontend deploy; needs re-sniff on breakage. Response shape differs from current `schedulePage` → parser rewrite.
2. **Community mirror APIs** (e.g. Leaguepedia/Fandom cargo API, third-party esports APIs) — stable contracts, but different data model and licensing/ToS review needed.
3. **Official Riot esports data** — Riot has no public esports schedule API in the developer portal; production key does not cover lolesports data.
4. **Keep module, degrade gracefully** — if no replacement chosen, surface "schedule source unavailable" to users instead of daily error logs; disable daily push until fixed.

Recommendation: option 1 for continuity (accept fragility, keep 60-min stale-cache fallback and add alerting on repeated 403/400), else option 4 short-term.

## Follow-up: RGAPI key + LoL Esports Data Portal (checked 2026-08-05)

- User's `RGAPI-…` dev-portal key: valid (200 on `vn2.api.riotgames.com/lol/status/v4`), still 403 on `esports-api.lolesports.com` with both `x-api-key` and `X-Riot-Token`. Dev portal keys only cover `*.api.riotgames.com`; no esports schedule product there.
- Riot dev diary "Introducing the New LoL Esports Data Portal": LDP built with Bayes Esports, replaced LEGs/ACS; targets pro teams/partners; community access "planned".
- Bayes Esports acquired by GRID (bayesesports.com 301 → grid.gg). LoL data now behind GRID's paid portal (`grid.gg/get-league-of-legends/`). GRID Open Access (free tier) covers CS2/Dota2 only, and Series Events (schedules) is paid-tier even there.
- Net: the official replacement is commercially gated; not viable for the bot. Fix options unchanged (persisted-query proxy / Leaguepedia / graceful degrade).

## Solution options (validated 2026-08-05)

1. **lolesports.com `/api/gql` persisted queries** — recommended primary. Anonymous access verified; needs sniffed sha256Hash + variables from site frontend; parser rewrite; alert on `PERSISTED_QUERY_NOT_FOUND`; keep stale cache.
2. **PandaScore free tier** — `/lol/matches/upcoming`, token auth, ~1k req/h hobby tier, non-commercial. Stable contract; naming differs from Riot. Verify signup still self-serve.
3. **Leaguepedia Cargo API** (`MatchSchedule` table) — live test from this host: immediate `ratelimited` (anon datacenter IP throttled). Needs Fandom bot-password auth + CC BY-SA attribution.
4. **GRID** — official successor, paid for LoL; Open Access free tier = CS2/Dota2 only, schedules excluded. Not viable now; recheck quarterly.
5. **Scrape `lolesports.com/schedule` RSC payload** — fallback only; more fragile than option 1, no advantage.
6. **Graceful degrade** — regardless of source: user-facing "source unavailable" message + pause daily push on repeated failure.

Recommendation: 1 primary, 2 as designed fallback.

## Implemented (2026-08-05): option 1, gql persisted-query client

No browser available (headless ARM64 server) — persisted-query ID extracted statically instead of via devtools:

1. Bundle analysis: lolesports.com Apollo client uses `generatePersistedQueryIdsFromManifest` — IDs come from a build manifest, NOT computed sha256 (both `__meta__.hash` and sha256(print(doc)) are rejected by the gateway).
2. Manifest = webpack chunk 29 (`loadManifest:()=>o.e(29)…` in bundle); URL resolved from webpack runtime chunk-hash map → `/_next/static/chunks/29.<hash>.js`; contains `{name, id, body}` per operation.
3. Operation chosen: `homeEvents` — date-window filter (`eventDateStart/End`), pagination (`pages.newer` + `pageToken`), events carry startTime/state/type/blockName/league/match.strategy/matchTeams(result.outcome/gameWins). Same semantic contract as dead REST API. States verified identical: unstarted/inProgress/completed; pending result = `{outcome:null,gameWins:0}` — `scoreIsPublished` logic still correct.
4. Gateway requirements discovered: `apollographql-client-name` + `-version` headers mandatory ("No client headers set"); `sport` enum lowercase `["lol"]`; Date params `YYYY-MM-DD`.

Code changes (`internal/modules/lol/`):
- `api_client.go`: transport rewritten — POST /api/gql persisted op `homeEvents` (ID const + refresh procedure in package doc). Removed dead public API key. `gqlEvent→ScheduleEvent` mapping keeps module contract + bson cache shape unchanged. Date window padded ±1 day, exact [from,to) filtered client-side (gateway date tz semantics unspecified). In-band GraphQL errors (HTTP 200 + errors[]) surface as fetch errors; `PERSISTED_QUERY_NOT_IN_LIST` logged as `lol_persisted_query_rotated` ERROR with refresh hint. Older-page walk removed (window is query-bounded; newer-walk only).
- `api_client_test.go`, `handlers_test.go`: fixtures → gateway shape; new tests for gql-error-as-fetch-error and exact-window filtering.
- format.go/handlers.go/cron.go/subscribers.go untouched.

Verification: module tests green; live probe against real gateway returned 72 events for today+tomorrow incl. an in-progress LCK match, FilterMajor→10; `go test ./...`, `go vet`, `golangci-lint` all clean. Stale-cache fallback (60 min) retained.

## Unresolved questions

- Whether Riot will publish an official replacement API (watch RiotGames/developer-relations).
- Persisted-query ID churn rate on lolesports.com deploys (determines option 1 maintenance cost).
- PandaScore free-tier availability/terms as of now (not yet verified with a signup).
