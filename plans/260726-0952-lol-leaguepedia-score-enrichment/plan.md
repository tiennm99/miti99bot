---
title: "LoL Leaguepedia score enrichment"
description: "Fill missing series scores from Leaguepedia when lolesports marks a match completed but publishes no result"
status: cancelled
priority: P2
branch: "main"
tags: [lol, external-api, enrichment]
blockedBy: [260805-1708-lol-pandascore-replacement]
blocks: []
created: "2026-07-26T02:54:48.516Z"
createdBy: "ck:plan"
source: skill
---

# LoL Leaguepedia score enrichment

> **Cancelled 2026-08-05.** Superseded by
> [260805-1708-lol-pandascore-replacement](../260805-1708-lol-pandascore-replacement/plan.md):
> the lol module's upstream moved to PandaScore, whose match `results` carry
> final series scores directly, absorbing this plan's enrichment job. The
> lolesports "completed without score" ingestion gap this plan targeted no
> longer applies to the new source.

## Overview

lolesports flips `event.state` to `completed` off the broadcast timeline but fills `result.outcome`/`result.gameWins` from a separate per-game ingestion path. When that path stalls, finished matches carry `{"outcome": null, "gameWins": 0}`. Commit `570ee94` made this render honestly as `☑️ … score pending`; this plan restores a real score by filling the gap from Leaguepedia.

**Additive only.** lolesports stays authoritative for schedule, times, Bo count, block names, and any score it *does* publish. Leaguepedia is consulted solely for events where `scoreIsPublished(t1, t2) == false`. Every failure path — disabled, throttled, network error, ambiguous match, no row — falls through to today's `☑️ … score pending`.

Evidence base: `plans/reports/260726-0941-lol-score-source-alternatives.md`.

## Key Design Decisions

**Join on team name + date, not team codes.** Riot's payload already carries `Team.Name` (`"Movistar KOI"`), and Leaguepedia's `MatchSchedule.Team1`/`Team2` are also full names. No code-mapping table, no `Teams` table query, no per-split `OverviewPage` string to maintain. This removes the largest maintenance risk identified in research.

**Query by UTC time window, not by tournament.** One Cargo request per render covers every league in the window. Avoids `OverviewPage` naming (`LEC/2026 Season/Split 3`), which drifts every split.

**No new persistence.** The existing TTL index (`startup.go:38-52`) has a partial filter scoped to `_id` in `[matches:, matches;)`, so any new cache prefix would never expire. Enrichment runs inline per render; the daily push is 1 request/day and `/lol*` commands are user-triggered and low-volume. Revisit only if rate limiting proves to bite.

**Off by default.** Anonymous Fandom Cargo throttled hard during research (1 success in ~15 burst attempts, then connection refused). Ship behind `LOL_LEAGUEPEDIA_ENABLED`, matching the env-const + `os.Getenv` pattern in `internal/modules/misc/wheelofnames_api_client.go:62`.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Feasibility Gate](./phase-01-feasibility-gate.md) | Pending |
| 2 | [Cargo Client](./phase-02-cargo-client.md) | Pending |
| 3 | [Score Join](./phase-03-score-join.md) | Pending |
| 4 | [Wire-in](./phase-04-wire-in.md) | Pending |

Phase 1 is a **kill switch**, not a formality. It is cheap (a handful of curl calls) and can invalidate Phases 2-4 entirely. Do not write production code before it passes.

## Architecture

```
replyForRange (handlers.go)  ─┐
                              ├─→ GetEventsWithFallback ─→ FilterMajor ─→ enrichScores ─→ render
runDailyPush (cron.go)       ─┘                                              │
                                                                             │ only if any event has
                                                                             │ state==completed && !scoreIsPublished
                                                                             ▼
                                                              leaguepediaClient.FetchResults(from, to)
                                                                             │
                                                              ┌──────────────┴──────────────┐
                                                         1 match found              0 or 2+ / error
                                                              │                            │
                                                    fill GameWins+Outcome            leave untouched
                                                              ▼                            ▼
                                                       ✅ MKOI 2–0 KC          ☑️ MKOI vs KC · score pending
```

Invariants (assert in tests):
- Never overwrites a score lolesports published
- Never mutates a non-`completed` event
- Never changes schedule fields (`StartTime`, `Strategy`, `BlockName`, `League`)
- Any error → input events returned unchanged, digest still sends

## Acceptance Criteria

- [ ] `✅ MKOI 2–0 KC · Bo3 (Week 1)` renders for a lolesports-unscored match that Leaguepedia has
- [ ] `☑️ … score pending` still renders when Leaguepedia lacks it, errors, is throttled, or is disabled
- [ ] Scores lolesports *does* publish are never altered
- [ ] Enrichment never blocks or fails a digest — no new error path reaches the user
- [ ] Zero extra HTTP requests when every completed event already has a score
- [ ] `go test ./...` green; no network access in tests (`httptest` only)
- [ ] Feature off by default; single env var flips it

## Dependencies

No cross-plan dependencies. All 8 existing plans are `completed` and scoped to the `stock` module.

External: `lol.fandom.com` Cargo API (CC-BY-SA 3.0 — attribution obligation, see Phase 4).

## Open Questions

Tracked in Phase 1; all must be answered before Phase 2 starts.
