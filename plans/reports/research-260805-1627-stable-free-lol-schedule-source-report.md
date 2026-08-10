# Research: stable free LoL esports schedule source (replace/back up gql client)

Date: 2026-08-05 16:27 ICT | Sources: 5 web searches + live probes from bot's server | Context: commit b76d0ca (gql persisted-query client) works but breaks when Riot rotates the persisted-query ID.

## Executive summary

No free source beats the current gql client on data quality (official, live state, scores, league slugs). The stability gain is available two ways: **Leaguepedia** (no signup, proven decade-old MediaWiki API, works anon from this server — verified live today) or **PandaScore free tier** (most stable API contract, 1000 req/h, but adds account+token dependency). Recommendation: keep gql client primary, add Leaguepedia as the fallback source; skip PandaScore unless the user prefers a commercial-grade contract over zero-dependency.

## Candidates

### 1. Leaguepedia Cargo API — recommended fallback
- Endpoint: `https://lol.fandom.com/api.php?action=cargoquery`, table `MatchSchedule` (Team1, Team2, DateTime_UTC, BestOf, Winner, scores, OverviewPage, Stream…).
- **Verified live from the bot's server (2026-08-05):** proper descriptive UA + single paced request → HTTP 200 with correct data. Earlier `ratelimited` was the anon throttle (~1 req/min); burst of 2 rapid requests trips it.
- Cross-validated against lolesports gateway: same match, same start time (Estral vs Ei Nerd 00:00 UTC) — data agrees.
- Auth: Fandom account + bot password raises limits (exact numbers undocumented; anon ~1/min per mediawiki-api list thread). Anon is enough for this bot: 1 daily push + user commands behind 60-min cache, if requests are serialized.
- License: CC BY-SA — attribution required (one line in /help or bot bio suffices).
- Cons: different data model — full team names not codes, league via `OverviewPage` string not slug (needs mapping for FilterMajor), no live `inProgress` state (has Winner/score for finished). Community-entered data (major leagues near-instant, minor leagues can lag).
- Stability: MediaWiki+Cargo API unchanged for ~a decade; immune to Riot frontend deploys.

### 2. PandaScore free tier — most stable contract
- 1000 req/h free, no credit card (pricing page). Versioned REST (`/lol/matches/upcoming`, `/lol/matches/past`), token auth, real docs.
- Cons: signup + API token to manage (new secret in env); team/league naming differs; free-tier terms can change unilaterally; third-party (not official Riot data, though sourced professionally).
- Best choice if a long-lived stable contract matters more than zero-dependency.

### 3. Liquipedia LPDB API — viable but stricter
- Free tier 60 req/h; **requires open-source project + non-commercial**; CC-BY-SA attribution; custom UA with contact mandatory; MediaWiki api.php capped 1 req/2s.
- No advantage over Leaguepedia for LoL specifically; stricter terms. Skip.

### 4. Community iCal feeds (zlypher/lol-events, olol/lolesp-cal)
- Free, zero auth, but single-maintainer projects (staleness risk), ICS carries no scores/state. Emergency backup grade only. zlypher feed last updated 2026-07 and itself sources Liquipedia.

### 5. lolesportsapi.com
- Commercial wrapper, free 500 calls/month; unknown operator that itself scrapes Riot. Adds a middleman with less stability than our own gql client. Skip.

## Comparative summary

| Source | Signup | Quota (free) | Live state | Scores | Deploy-proof | Data model fit |
|---|---|---|---|---|---|---|
| Current gql client | none | ample | ✅ | ✅ | ❌ ID rotates | ✅ native |
| Leaguepedia | none | ~1/min anon | ❌ | ✅ | ✅ | ⚠️ mapping needed |
| PandaScore | account+token | 1000/h | ✅ | ✅ | ✅ | ⚠️ mapping needed |
| Liquipedia | none | 60/h LPDB | ❌ | ✅ | ✅ | ⚠️ + strict terms |
| iCal feeds | none | n/a | ❌ | ❌ | ⚠️ maintainer | ❌ |

## Recommendation

1. Keep gql client primary (richest data, official).
2. Implement Leaguepedia as fallback in `GetEventsWithFallback`'s failure path (after stale cache): serialize requests, ≥60s spacing, descriptive UA, map OverviewPage→league slug for the major-league allowlist, render without live-state features. Zero new secrets or accounts.
3. Reconsider PandaScore only if Leaguepedia's anon limit proves painful in practice.
4. Regardless: alert on `lol_persisted_query_rotated` so ID refresh happens fast (fallback covers the gap).

## Sources

- [Leaguepedia rate limit thread (mediawiki-api list)](https://lists.wikimedia.org/hyperkitty/list/mediawiki-api@lists.wikimedia.org/thread/A6VWUYRHLGGJWZ3USGEBQJSDMX6A4YCM/)
- [PandaScore pricing](https://www.pandascore.co/pricing)
- [Liquipedia API terms](https://liquipedia.net/api-terms-of-use), [usage guidelines](https://liquipedia.net/commons/Liquipedia:API_Usage_Guidelines)
- [zlypher/lol-events iCal](https://github.com/zlypher/lol-events), [olol/lolesp-cal](https://olol.github.io/lolesp-cal/)
- [river-mwclient (Leaguepedia wrapper by its dev)](https://pypi.org/project/river-mwclient)
- Live probes: Leaguepedia cargoquery 200 from bot host (2026-08-05), cross-checked vs lolesports gateway.

## Unresolved questions

- Leaguepedia authenticated (bot-password) rate limit exact number — undocumented; ask on their Discord if anon 1/min ever binds.
- PandaScore free-tier ToS details (commercial-use clause) — not verified beyond "no credit card"; check at signup if pursued.
- Whether the bot repo being public matters to the user (required only for Liquipedia, not Leaguepedia/PandaScore).
