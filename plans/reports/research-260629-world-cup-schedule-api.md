---
type: research
date: 2026-06-29
topic: world-cup-schedule-api-for-wc-module
status: complete
---

# Research Report: World Cup Schedule API For `wc` Module

## Executive Summary

Recommendation: use **football-data.org** as the primary provider for the first `wc`
module. It has FIFA World Cup coverage in the free tier, stable REST JSON, simple
match resources, and a practical free rate limit for a Telegram bot if we cache
like `lolschedule`. Use **API-Football / API-SPORTS** as the first paid/richer
fallback if football-data.org misses venue/live detail.

Do not scrape FIFA. No stable official public FIFA developer API surfaced. Do
not make community/single-maintainer APIs the primary source for production
notifications. Static JSON datasets are useful as fallback/seed data only.

## Research Methodology

- Conducted: 2026-06-29
- Sources consulted: 12
- Local context: `internal/modules/lolschedule/*`
- Key search terms: `World Cup 2026 schedule API`, `football-data.org WC API`,
  `API-Football World Cup 2026 fixtures`, `Sportmonks World Cup API`,
  `TheSportsDB World Cup API`, `openfootball worldcup.json`
- Recency need: high. World Cup 2026 is active/current in this environment.

## Existing `lolschedule` Pattern

`lolschedule` has the right shape to copy:

- client file owns upstream schema, HTTP, pagination, cache/stale fallback.
- handlers support day and week views.
- cron pushes a daily digest to subscribers.
- storage uses typed `DocStore`.
- renderer keeps upstream details out of handlers.

For `wc`, the data model is simpler than LoL esports:

- no pagination needed if provider returns full tournament/competition matches.
- date window filtering by ICT day/week still applies.
- render needs stage/group, teams, score/status, kickoff in ICT, venue.

## Provider Comparison

| Provider | Fit | Cost | Auth | Strengths | Risks |
|---|---:|---:|---|---|---|
| football-data.org | Best default | Free tier | `X-Auth-Token` | WC is listed in free tier; REST JSON; `WC` competition code; 10 req/min free | Register token; data richness lower than paid APIs |
| API-Football / API-SPORTS | Best rich fallback | Free 100/day; paid from $19/mo | `x-apisports-key` | Explicit World Cup 2026 guide; `fixtures?league=1&season=2026` returns all 104 matches with UTC date, venue, status | 100/day cap tight if low TTL; paid for comfortable usage |
| TheStatsAPI static JSON | Static fallback | Free static file; paid REST from $50/mo | none for static | Free JSON/CSV fixture file, all 104 matches, UTC kickoff, venues | Static file is not live status source; attribution likely required |
| OpenFootball `worldcup.json` | Seed/fallback | Free CC0 | none | Public domain, no key, includes 2026 fixtures/results JSON | Not live-updated; time format needs normalization |
| Sportmonks | Premium option | EUR 69/mo+ | token | Rich live scores/events/standings/bracket; strong docs | Overkill/cost for bot schedule digest |
| BALLDONTLIE FIFA | Niche rich option | trial/paid | `Authorization` | World Cup-specific endpoints for matches/standings/stats | Requires key; trial 5 req/min; betting-heavy |
| TheSportsDB | Not primary | Free/premium | free key/premium | Open sports DB, schedule endpoints | Crowd-sourced; free limits small; WC league mapping uncertain |
| `worldcup26.ir` GitHub API | Avoid primary | Free | none | Open-source, no auth, simple endpoints | Single-maintainer/SLA unknown; hosted domain dependency |

## Primary Recommendation: football-data.org

Use:

```http
GET https://api.football-data.org/v4/competitions/WC/matches?season=2026
X-Auth-Token: ${WC_FOOTBALL_DATA_TOKEN}
```

Why:

- Free-tier coverage page lists **Worldcup** under free competitions.
- API docs list competition code `WC` for World-Cup.
- Match resource includes scheduled date, status, teams, score shape.
- Free registered rate is 10 req/min, enough with cache.
- It is cleaner than one-off World Cup APIs and cheaper than Sportmonks.

Cache policy:

- Cache whole tournament match list.
- TTL 10-15 min while tournament active.
- Stale fallback 6-24h for schedule display if provider fails.
- Do not poll per chat. One provider call should serve all `/wc*` commands.

Expected env:

```sh
WC_PROVIDER=football-data
WC_FOOTBALL_DATA_TOKEN=...
```

## Fallback / Upgrade Recommendation: API-Football

Use if football-data.org lacks needed detail:

```http
GET https://v3.football.api-sports.io/fixtures?league=1&season=2026
x-apisports-key: ${WC_API_FOOTBALL_KEY}
```

Why:

- Their World Cup 2026 guide explicitly says this returns all 104 fixtures with
  fixture id, UTC date/time, venue, and status.
- Free plan is 100 requests/day and all endpoints are listed as available.
- Paid $19/mo plan gives 7,500/day; easy upgrade if the bot is popular.

Use as provider interface implementation, not hard-coded into handlers.

## Static Fallbacks

Use only when no API key is configured:

1. TheStatsAPI static JSON: `https://www.thestatsapi.com/world-cup/data/fixtures.json`
2. OpenFootball raw JSON: `https://raw.githubusercontent.com/openfootball/worldcup.json/master/2026/worldcup.json`

Behavior:

- Show schedule only.
- No live scores guarantee.
- Mark output as static/stale if needed.

This keeps `/wc` usable for local dev and avoids failing hard when env missing.

## Proposed `wc` Module Shape

Commands:

- `/wc [date]` — World Cup matches for one ICT day.
- `/wc_today` — today in ICT.
- `/wc_week` — current ICT week.
- `/wc_subscribe` — daily digest at 08:00 ICT.
- `/wc_unsubscribe`.

Internal files, mirroring `lolschedule`:

```text
internal/modules/wc/
  wc.go
  handlers.go
  api_client.go
  provider_football_data.go
  provider_api_football.go      # optional fallback/phase 2
  provider_static.go            # optional no-key fallback
  format.go
  parse_date.go
  subscribers.go
  cron.go
  *_test.go
```

Normalized model:

```go
type Match struct {
    ID        string
    StartTime time.Time // UTC
    Stage     string
    Group     string
    Home      Team
    Away      Team
    Status    string
    Score     Score
    Venue     string
    Source    string
}
```

Renderer should output:

```text
🏆 World Cup — Mon 29/06
20:00  Group A  Mexico 2-0 South Africa  FT
23:00  Group B  Canada vs Switzerland     BMO Field
```

## Security Considerations

- Treat provider tokens as secrets. Add to `.env.example`, never log.
- Use env names scoped to `wc`, e.g. `WC_FOOTBALL_DATA_TOKEN`.
- HTTP timeout 8s, same as `lolschedule`.
- Log status code and truncated body only; never log auth headers.
- Cache to Mongo to reduce provider quota and avoid chat-driven burst.

## Performance / Reliability

- Single full-list fetch is cheaper than date-by-date provider queries.
- Cache by provider + season (`wc:football-data:2026`).
- Filter in memory by ICT day/week.
- Stale-while-error is mandatory during tournament: a stale schedule is better
  than no schedule.
- During live matches, do not promise second-level live scores unless provider
  and quota support it. For Telegram, 10-15 min freshness is acceptable.

## Decision

Implement provider interface with **football-data.org first**. Keep API-Football
as the documented fallback/upgrade. Do not start with Sportmonks or custom
single-maintainer APIs.

## References

- football-data.org coverage: https://www.football-data.org/coverage
- football-data.org API docs: https://www.football-data.org/documentation/api
- football-data.org pricing: https://www.football-data.org/pricing
- API-Football World Cup 2026 guide: https://www.api-football.com/news/post/fifa-world-cup-2026-guide-to-using-data-with-api-sports
- API-Football pricing: https://www.api-football.com/pricing
- API-Sports football pricing/details: https://api-sports.io/sports/football
- Sportmonks World Cup API: https://www.sportmonks.com/football-api/world-cup-api/
- Sportmonks World Cup guide: https://www.sportmonks.com/blogs/world-cup-2026-api-guide-coverage-endpoints-data-types/
- TheStatsAPI static data: https://www.thestatsapi.com/world-cup/data
- OpenFootball worldcup.json: https://github.com/openfootball/worldcup.json
- TheSportsDB docs: https://www.thesportsdb.com/docs_api_guide
- BALLDONTLIE FIFA API: https://fifa.balldontlie.io/

## Next Steps

1. Add `WC_FOOTBALL_DATA_TOKEN` to `.env.example` and Coolify env.
2. Plan `wc` module as a copy-shaped sibling of `lolschedule`.
3. Implement provider interface + football-data.org client.
4. Add no-key static fallback only if user wants local zero-config behavior.
5. Register commands in `telegram-commands.json` and README.

## Unresolved Questions

- Do we want live score freshness or only schedule/daily digest?
- Is adding a free football-data.org token acceptable for production env?
- Should `/wc_subscribe` be separate from `/lolschedule_subscribe`, or should
  future schedule modules share a generic subscription store?
