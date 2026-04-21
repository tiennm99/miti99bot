# lolschedule

LoL esports match schedule via the **Leaguepedia** MediaWiki/Cargo API.

## Commands

| Command | Description |
|---|---|
| `/lol_today` | Today's matches (ICT). Scores if played / live. Times if scheduled. |
| `/lol_week`  | Next 7 days, grouped by day. |

## Data source

- Endpoint: `https://lol.fandom.com/api.php?action=cargoquery`
- Table: `MatchSchedule`
- Fields used: `DateTime_UTC`, `Team1`, `Team2`, `Team1Score`, `Team2Score`, `Winner`, `Tournament`, `BestOf`, `OverviewPage`
- Auth: none. UA header identifies the bot.

See `plans/reports/researcher-260421-0845-leaguepedia-api-verification.md` and `plans/reports/researcher-260421-0909-leaguepedia-auth-token.md` for the feasibility verdict + rate-limit strategy.

## Caching

Two layers:

1. **Worker edge cache** — `fetch(url, { cf: { cacheTtl: 30, cacheEverything: true }})` dedupes near-simultaneous calls across requests to the same edge POP.
2. **KV cache** (per module) — `matches:{fromIso}:{toIso}` with `ts`+`rows` payload. TTL: 60 s for `/lol_today`, 300 s for `/lol_week`. On fetch failure, stale cache (up to 4× TTL) is returned as a fallback.

## Time zone

All rendering is in **ICT (UTC+7)**. `DateTime_UTC` strings from the wiki are parsed as UTC, then shifted for display. Change `TZ_OFFSET_MS` in `format.js` if you need a different zone.

## Files

- `index.js` — module contract, registers `/lol_today` and `/lol_week`.
- `api-client.js` — cargoquery POST, range fetch, cache-first wrapper.
- `format.js` — pure renderers (`classifyMatch`, `formatMatchLine`, `renderToday`, `renderWeek`).
- `handlers.js` — grammY command handlers + ICT day-boundary helpers.

## Known caveat

The `where=` clause with `>=`/`<` operators hits `MWException` from shared egress IPs during verification (see auth-token report). POST + a proper UA appears to work from CF Worker egress, but confirm on first deploy. If it fails in production, fall back to `HOLDS`-based filters or a client-side filter over a broader fetch.
