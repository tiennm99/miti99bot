---
phase: 1
title: "Feasibility Gate"
status: pending
effort: "S"
priority: P1
dependencies: []
---

# Phase 1: Feasibility Gate

## Overview

Answer the questions research could not, because Fandom rate-limited then refused the connection. **This phase can cancel the plan.** No production code until every check below passes.

## Why This Exists

Research (`plans/reports/260726-0941-lol-score-source-alternatives.md`) established that Leaguepedia is the only viable free source with independent ingestion, and that its `MatchSchedule` table exposes `Team1Score`/`Team2Score`/`Winner`. It could **not** establish that Leaguepedia actually has the LEC/LCS scores lolesports is missing, nor how fast its editors are. Those are the load-bearing assumptions.

Leaguepedia certainly *covers* LEC/LCS/LPL/LCK — it is the reference wiki. The open risks are latency, join viability, and access.

## Checks

Run against the known-bad matches (lolesports has these as `completed` with `0-0`):

| startTime (UTC) | league | teams |
|---|---|---|
| 2026-07-25T14:30Z | lec | G2 Esports vs Team Vitality |
| 2026-07-25T17:30Z | lec | Movistar KOI vs Karmine Corp |
| 2026-07-25T19:00Z | cblol-brazil | LOUD vs paiN Gaming |
| 2026-07-25T20:00Z | lcs | FlyQuest vs LYON |
| 2026-07-25T23:00Z | lcs | Dignitas vs Sentinels |

Baseline query (proven to work during research):

```bash
curl -s -G "https://lol.fandom.com/api.php" \
  -A "miti99bot/0.1 (+https://github.com/tiennm99/miti99bot)" \
  --data-urlencode "action=cargoquery" \
  --data-urlencode "format=json" \
  --data-urlencode "tables=MatchSchedule=MS" \
  --data-urlencode "fields=MS.DateTime_UTC,MS.Team1,MS.Team2,MS.Team1Score,MS.Team2Score,MS.Winner,MS.BestOf,MS.OverviewPage" \
  --data-urlencode "where=MS.DateTime_UTC >= '2026-07-25 14:00:00' AND MS.DateTime_UTC < '2026-07-26 00:00:00'" \
  --data-urlencode "order_by=MS.DateTime_UTC" \
  --data-urlencode "limit=100"
```

### Gate 1 — Data exists (BLOCKING)

- [ ] At least 4 of the 5 matches above return a row with non-empty `Team1Score`/`Team2Score`
- [ ] Scores are plausible for the Bo3 (2-0/2-1, not 0-0)
- [ ] For `Movistar KOI vs Karmine Corp`, score is `2-0` or `0-2` — VOD evidence from the debug session showed exactly 2 games played, game 3 `unneeded`

**If fewer than 4 of 5 have scores → STOP. Cancel plan.** Leaguepedia is no faster than Riot and the enrichment buys nothing.

### Gate 2 — Name join works (BLOCKING)

Compare Leaguepedia `Team1`/`Team2` against Riot `Team.Name` for the same matches.

- [ ] Riot `Team.Name` values match Leaguepedia `Team1`/`Team2` exactly, OR differ only by case/punctuation/whitespace
- [ ] Record every mismatch verbatim — these define the normalization rules for Phase 3
- [ ] Confirm no two matches in a single day-window share the same normalized name pair (would make the join ambiguous)

Known naming risk to check specifically: `paiN Gaming` (Riot code `PAIN`) and `LYON` — irregular capitalization.

**If names diverge structurally (e.g. Leaguepedia uses `Movistar KOI (European Team)` disambiguators) → note it; Phase 3 gains a disambiguator-stripping rule. Not fatal, but scope grows.**

### Gate 3 — Editorial latency (INFORMATIONAL, shapes value)

- [ ] Note wall-clock lag between match end and Leaguepedia having the score, for at least 2 matches
- [ ] If Leaguepedia is consistently *slower* than Riot's backfill, the feature is near-worthless even if Gates 1-2 pass → report and ask before continuing

### Gate 4 — Access viability (BLOCKING)

- [ ] A single cold query succeeds without prior warmup (research only succeeded on retry 3 of 3 after bursts)
- [ ] Determine whether the throttle is per-IP burst-based or a hard quota — space 3 queries 60s apart and confirm all 3 succeed
- [ ] Confirm no API key or login is required for `MatchSchedule` reads

**If a lone spaced query cannot reliably succeed → STOP or pivot to Liquipedia LPDB** (free tier, needs key + manual approval; repo is public so it likely qualifies).

### Gate 5 — Licensing

- [ ] Confirm CC-BY-SA 3.0 attribution requirement and decide placement (Phase 4 handles it — likely a line in `/lol` help or the digest footer)

## Deliverable

Append findings to `plans/reports/260726-0941-lol-score-source-alternatives.md` (do not create a second report — the unresolved-questions section there is exactly what this phase closes). Record: raw JSON for one match, the exact name pairs observed, throttle behaviour, and a GO/NO-GO.

## Success Criteria

- [ ] Gates 1, 2, 4 pass → GO, proceed to Phase 2
- [ ] Gate 3 measured and acceptable
- [ ] Any gate fails → NO-GO recorded with evidence; plan marked `cancelled`; report back before writing code
- [ ] Normalization rules for Phase 3 written down from real observed data, not guessed

## Risk Assessment

**Biggest risk: doing Phases 2-4 first and discovering Leaguepedia is also stale.** That is the entire reason this phase is a hard gate rather than advisory.

Second risk: verifying against *this* stall only. One stall is a sample of one. If Gate 3 shows Leaguepedia leading Riot by hours here, that is suggestive, not proof of steady-state behaviour. Note the caveat rather than over-claiming.
