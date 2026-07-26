---
phase: 3
title: "Score Join"
status: pending
effort: "M"
priority: P2
dependencies: [2]
---

# Phase 3: Score Join

## Overview

Pure functions that decide which lolesports events need a score, and merge Leaguepedia rows into them. No I/O — this is the correctness core and must be exhaustively unit-tested.

## Requirements

- Identify events needing enrichment: `State == "completed" && !scoreIsPublished(t1, t2)`
- Match each to at most one `SeriesResult`
- Fill `GameWins` + `Outcome` so `formatEventLine` renders `✅` naturally, with the winner bolded
- Refuse ambiguous matches rather than guess
- Leave everything else byte-identical

## Architecture

New file `internal/modules/lol/score_enrich.go`. Reuses `scoreIsPublished` from `format.go` — that predicate already encodes exactly "has upstream published a usable score", so the trigger needs no new logic.

```go
// needsScore reports whether an event is a finished series with no published
// score — the only shape this enrichment may touch.
func needsScore(e ScheduleEvent) bool

// normalizeTeamName folds the cosmetic differences between Riot's Team.Name and
// Leaguepedia's Team1/Team2 into a comparable key. Rules are derived from the
// real pairs recorded in Phase 1, not invented here.
func normalizeTeamName(s string) string

// mergeScores returns events with missing scores filled from results. Input is
// never mutated; a copy is returned. Unmatched, ambiguous, and already-scored
// events pass through untouched. Reports how many were filled, for logging.
func mergeScores(events []ScheduleEvent, results []SeriesResult) ([]ScheduleEvent, int)
```

### Match key

`(normalizeTeamName(team1), normalizeTeamName(team2), UTC date of StartTime)`

Date rather than exact instant: Leaguepedia records *scheduled* time, which can drift from Riot's by minutes when a broadcast slips. Same-day plus an exact team pair is specific enough — two teams do not play the same opponent twice in a day in these leagues.

Orientation: try `(t1, t2)`; if no hit, try `(t2, t1)` and **swap the scores when applying**. Riot and Leaguepedia do not guarantee the same side ordering. Getting this backwards silently inverts every result, so it needs its own test.

### Ambiguity rule

Build the index as `map[key][]SeriesResult`. Apply only when `len(candidates) == 1`. Two or more → skip and log at warn. A wrong score is worse than `score pending`; that is the premise of the whole fix.

### Applying a result

```go
// Winner gets outcome "win", loser "loss", so formatEventLine bolds correctly.
// Outcome must be set: scoreIsPublished keys off it, so a fill that set only
// GameWins would still render as "score pending".
```

Edge case — a genuine draw or an unresolved series cannot occur here, because Phase 2 already dropped `0-0` rows. If both wins are equal and non-zero (a real Bo2 1-1), set both outcomes to `"loss"`... **decide during implementation**: LEC/LCS/LPL Bo2 splits exist. Safer: if `Team1Wins == Team2Wins`, fill `GameWins` on both and set both `Outcome` to `"tie"`. `formatEventLine` bolds only on `== "win"`, so a tie renders `✅ A 1–1 B` unbolded, which is correct. Add a test.

## Related Code Files

- Create: `internal/modules/lol/score_enrich.go`
- Create: `internal/modules/lol/score_enrich_test.go`
- Reference (do not modify): `internal/modules/lol/format.go` (`scoreIsPublished`, `declaredOutcome`, `seriesWins`)

## Implementation Steps

1. `needsScore` — thin wrapper over `State == "completed"` + `!scoreIsPublished(...)`; guard `len(Teams) >= 2` first (`format.go:126-131` tolerates short slices, so this must too)
2. `normalizeTeamName` — lowercase, trim, collapse internal whitespace, strip punctuation. Add a disambiguator strip (`" (…Team)"` suffix) **only if Phase 1 observed one**
3. Build the candidate index from `results`
4. Copy the events slice; deep-copy only the `Match.Teams` of events being modified (`Team.Result` is a pointer — mutating a shared `*TeamResult` would corrupt the caller's data and, worse, the cached payload)
5. For each event where `needsScore`, look up forward then reversed; apply on a unique hit
6. Return the copy plus a fill count

## Tests

- [ ] Fills a `{"outcome":null,"gameWins":0}` event → renders `✅ MKOI 2–0 KC` with `<b>MKOI</b>` via `formatEventLine`
- [ ] Reversed orientation: Leaguepedia has `KC vs MKOI 0-2` → still yields `MKOI 2–0 KC`, winner bolded correctly
- [ ] Already-scored event is **never** touched, even when a conflicting Leaguepedia row exists
- [ ] `unstarted` and `inProgress` events untouched
- [ ] No matching row → unchanged, still `☑️ … score pending`
- [ ] Two candidates for one key → unchanged + fill count 0
- [ ] Name normalization: `"paiN Gaming"` vs `"PaiN Gaming"`, extra whitespace, punctuation
- [ ] Different UTC date, same teams → no match
- [ ] Tie (`1-1` Bo2) → both filled, neither bolded
- [ ] Event with <2 teams → no panic
- [ ] **Aliasing**: input events unchanged after `mergeScores` (assert the original `*TeamResult` pointers still read `Outcome == ""`)
- [ ] Empty results / empty events → no-op
- [ ] Schedule fields (`StartTime`, `Strategy.Count`, `BlockName`, `League`) identical before/after

## Success Criteria

- [ ] All tests green; existing `format_test.go` untouched and still passing
- [ ] `mergeScores` is pure — no I/O, no globals, no clock
- [ ] Input slice and its pointees provably unmutated
- [ ] Ambiguity and orientation both covered by dedicated tests

## Risk Assessment

**Pointer aliasing is the sharpest hazard.** `Team.Result` is `*TeamResult`, and enriched events may be the same objects written into the Mongo cache by `GetEventsWithFallback`. In-place mutation would persist a Leaguepedia-derived score into the lolesports cache, making a later cache read indistinguishable from real upstream data. Step 4 + the aliasing test exist for this.

**Silent orientation inversion** would report every score backwards while looking perfectly healthy. Dedicated test.

**Name drift over a season.** Rebrands mid-split break the join. Failure mode is benign (falls back to `score pending`), but log unmatched events at debug so it is diagnosable.
