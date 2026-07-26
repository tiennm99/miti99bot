# Research: Alternative LoL Result Source (restore real scores)

Conducted 2026-07-26 09:41 ICT. Trigger: lolesports marks matches `completed` without publishing `result.gameWins`/`outcome`, so the digest now shows `☑️ … score pending` (commit `570ee94`) instead of a score.

Goal: source that has real series scores for finished matches, so `✅ MKOI 2–0 KC` returns.

## Executive Summary

**Any wrapper of lolesports inherits the bug.** That eliminates most "alternative LoL API" search hits — they proxy the same upstream store. Verified empirically: `getCompletedEvents` returns the identical `MKOI 0-0 KC`, so the gap is store-wide at Riot, not endpoint-specific.

Only genuinely independent ingestion helps: community wikis (Leaguepedia, Liquipedia) or commercial providers (PandaScore, Abios, GRID).

**Recommendation: don't build it yet.** The schedule half of lolesports is reliable; only results are broken, and they self-heal. A second provider means a new HTTP dep + team-name→code mapping + cache + failure handling — real complexity for a cosmetic win on an intermittent upstream failure. Current behavior is honest. Wait, measure how often/long Riot stalls, and build the enrichment only if chronic.

If it does prove chronic → **Leaguepedia Cargo, as opportunistic enrichment, not migration.**

## Empirical Findings (this session)

| Probe | Result |
|---|---|
| `getSchedule` | 6 events `completed` w/ `{"outcome":null,"gameWins":0}`; clean cutoff — all ≤`2026-07-25T11:30Z` scored, all ≥`14:30Z` not |
| `getEventDetails` (match `115548681803406271`) | games 1&2 `state:unstarted` but **7 VODs each**; game 3 `unstarted`/0 VODs → played + broadcast, results never ingested |
| `getEventDetails` (scored control) | games `completed`/8 VODs, game 3 `unneeded` |
| `getCompletedEvents?tournamentId=115548681802226458` | **same staleness** — `MKOI 0-0 KC`. Rules out a free same-provider fix |
| `getStandings` | active stage returns `Playoffs` only; no useful Week-1 series data |
| Leaguepedia Cargo `MatchSchedule` | API works, returns populated `Team1Score`/`Team2Score`/`Winner` for finished matches. **Heavily throttled anonymously — 1 success in ~15 attempts** |

Notable: the one successful Cargo query returned scored results for matches at `2026-07-25 12:00–14:00Z` — inside the window Riot has failed on (cutoff `11:30Z`). Suggestive that Leaguepedia is ahead of Riot there, but those were amateur/university leagues with different editors, so **not** proof for LEC.

## Provider Comparison

| Provider | Cost | Auth | Independent ingestion? | Verdict |
|---|---|---|---|---|
| **Leaguepedia** (Fandom Cargo) | free | none | ✅ community-edited | **Best fit.** `MatchSchedule.Team1Score/Team2Score/Winner/BestOf`. Harsh anon rate limit; uses team *names* not codes; CC-BY-SA attribution required |
| **Liquipedia** LPDB | free tier | API key | ✅ | 60 req/hr. Free tier needs non-commercial + **open-sourced project** + custom UA + attribution. Repo is public → likely qualifies. Must apply for key |
| **PandaScore** | free tier 1k req/hr (schedules+results); paid from €400/mo/game | key | ✅ | Free tier may suffice for a daily digest. Verify results are in free scope — pricing implies historical is paid. Betting-adjacent ToS restrictions on stats plans |
| **Abios / GRID / Sportradar** | $2k–10k/mo | key | ✅ | Absurd for a Telegram bot. **Rule out** |
| lolesportsapi.com | free 500/mo | key | ❌ wraps lolesports | **Useless** — inherits the bug |
| Pupix/lol-esports-api | free | — | ❌ wrapper | **Useless** |
| Apify LoL scraper | paid compute | token | ❌ scrapes lolesports | **Useless** |

## Recommended Design (if/when built)

Enrichment layer, not a migration. lolesports stays authoritative for schedule, times, Bo count, block names — all correct today.

```
GetEventsWithFallback (lolesports)  →  events
   └─ for events where state==completed && !scoreIsPublished(t1,t2):
        └─ 1 batched Leaguepedia Cargo query for the day window
             ├─ hit  → fill gameWins + outcome → renders ✅ with real score
             └─ miss → unchanged ☑️ … score pending
```

Properties:
- `scoreIsPublished` (already shipped) is the exact trigger — no new detection logic
- 1 extra request per digest, only when a gap exists; cache alongside existing `cacheRecord`
- Degrades to current behavior on any failure — strictly additive, no regression path
- Never overwrites a Riot-published score; fills gaps only

Work required:
1. Team identity mapping — Leaguepedia returns `Movistar KOI`, we display `MKOI`. Cargo `Teams` table has a `Short` field; needs a cached lookup or a static map for the ~12 allowlisted leagues
2. `OverviewPage` naming per league/split (e.g. `LEC/2026 Season/Split 3`) — brittle, changes each split. Alternatively match on team names + UTC timestamp, which avoids page naming entirely
3. Rate-limit discipline — cache aggressively, custom UA w/ contact, single daily query
4. CC-BY-SA attribution somewhere user-visible

## Cost/Benefit

Against: new external dep on a community wiki (editorial lag, schema drift, throttling); name→code mapping is ongoing maintenance; ~150–250 LOC + tests; benefit is cosmetic — current output is already truthful.

For: restores the informative digest; useful independently if Riot's stalls become routine.

**Verdict: defer.** Cheapest next step is observation, not code — the current fix already fails safe.

## Unresolved Questions

1. **Could not verify** Leaguepedia holds the specific missing LEC/LCS scores (`MKOI vs KC`, `FLY vs LYON`, `DIG vs SEN`) — rate-limited on every targeted attempt; WebFetch of the wiki page returned HTTP 402. **This is the single blocking unknown**; retry the Cargo query later before committing to Leaguepedia.
2. Is Riot's stall a one-off or recurring? No history collected. Determines whether any of this is worth building.
3. Does PandaScore's free tier actually include completed-match results, or schedules only? Pricing page ambiguous.
4. Does Liquipedia grant LPDB keys to a public hobby Telegram bot? Terms say non-commercial + open-source, which fits, but approval is manual.
5. Leaguepedia editorial latency for major leagues specifically — unmeasured. If editors lag Riot, the enrichment adds nothing.
