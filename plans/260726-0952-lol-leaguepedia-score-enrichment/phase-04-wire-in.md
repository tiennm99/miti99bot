---
phase: 4
title: "Wire-in"
status: pending
effort: "S"
priority: P2
dependencies: [3]
---

# Phase 4: Wire-in

## Overview

Connect client + join to the four commands and the daily push, in one shared place. Add the CC-BY-SA attribution and document the env var.

## Requirements

- Every render path benefits: `/lol`, `/lol_tomorrow`, `/lol_this_week`, `/lol_next_week`, daily push
- **Zero** extra HTTP requests when nothing needs enrichment
- No new user-visible error path — enrichment failure is invisible except for the unchanged `☑️ score pending`
- One integration point, not five (DRY)

## Architecture

`replyForRange` (`handlers.go:96-124`) already funnels all four commands through fetch → filter → render. `runDailyPush` (`cron.go:174-253`) duplicates that sequence. So there are exactly **two** call sites.

Insert between filter and render:

```go
filtered := FilterMajor(events)
filtered = s.enrichScores(ctx, filtered, from, to)   // new
text = renderDay(filtered, from, emptyLine)
```

```go
// enrichScores fills scores lolesports has not published, from Leaguepedia.
// Best-effort by design: any failure returns events unchanged so the digest
// still sends. Makes no request when nothing needs enrichment.
func (s *state) enrichScores(ctx context.Context, events []ScheduleEvent, from, to time.Time) []ScheduleEvent {
    if !leaguepediaEnabled() || !slices.ContainsFunc(events, needsScore) {
        return events
    }
    results, err := s.leaguepedia.FetchResults(ctx, from, to)
    if err != nil {
        log.Warn("lol_leaguepedia_fetch_fail", "err", err)
        return events
    }
    merged, filled := mergeScores(events, results)
    log.Info("lol_leaguepedia_enriched", "filled", filled, "candidates", ...)
    return merged
}
```

The `ContainsFunc(events, needsScore)` short-circuit is what keeps steady-state cost at zero — on a normal day no event needs a score and no request is made.

`state` gains `leaguepedia *leaguepediaClient`, wired in `New` (`lol.go:17-22`) beside `client: &Client{}`.

### Attribution

CC-BY-SA 3.0 obliges crediting Leaguepedia when its data is displayed. Cheapest honest placement: a footer line appended **only when at least one score was filled**, so unaffected digests stay clean.

```
Scores via Leaguepedia (CC BY-SA 3.0)
```

This makes attribution conditional on actual use — decide during implementation whether `renderDay`/`renderWeek` take a footer argument or the caller appends. Prefer the caller appending, to keep the renderers pure.

## Related Code Files

- Modify: `internal/modules/lol/handlers.go` (one call in `replyForRange`)
- Modify: `internal/modules/lol/cron.go` (one call in `runDailyPush`)
- Modify: `internal/modules/lol/lol.go` (wire `leaguepedia` into `state`)
- Modify: `docs/deploy-coolify-selfhosted.md` (add `LOL_LEAGUEPEDIA_ENABLED` to the env table at line ~28, `Required: optional`)
- Modify: `.env.example` (the docs table links to it, so both must stay in sync)
- Create: `internal/modules/lol/score_enrich_integration_test.go` (or extend `handlers_test.go`)

## Implementation Steps

1. Add `leaguepedia *leaguepediaClient` to `state` (`handlers.go:17-29`); wire in `New`
2. Add `enrichScores` method — put it in `score_enrich.go` next to `mergeScores`, not in `handlers.go`, keeping handlers thin
3. Call it in `replyForRange` after `FilterMajor`
4. Call it in `runDailyPush` after `FilterMajor`, **before** `claimDailyPush` — a Leaguepedia hiccup must not consume the day's idempotency claim, matching the existing rationale at `cron.go:195-197`
5. Conditional attribution footer when `filled > 0`
6. Document the env var; note default-off and the rate-limit rationale

## Tests

Existing `handlers_test.go` already injects a fake lolesports server; extend that harness with a fake Cargo server.

- [ ] `/lol` with an unscored event + Leaguepedia having it → reply contains `✅` and the real score
- [ ] Same, Leaguepedia 500 → reply contains `☑️ … score pending`, **no error to user**
- [ ] Same, env disabled → `☑️ … score pending`, zero Cargo requests
- [ ] All events already scored → zero Cargo requests (assert counter == 0)
- [ ] Daily push enriches, and a Cargo failure still sends the digest
- [ ] Daily push: Cargo failure does **not** consume the push claim (second run still sends)
- [ ] Attribution footer present only when a score was filled
- [ ] Week view enriches across multiple days in one Cargo request (assert counter == 1)

## Success Criteria

- [ ] `go test ./...` fully green
- [ ] `go vet ./...` clean; changed files gofmt-clean modulo the repo's pre-existing CRLF
- [ ] Feature off by default — behaviour byte-identical to `570ee94` with the env var unset
- [ ] Zero Cargo requests in the steady state
- [ ] Attribution shown when data is used
- [ ] Manual check with the var set against the real API, confirming a real score renders

## Risk Assessment

**Regression surface is the four commands + daily push.** Mitigated by the default-off flag: with it unset the diff is inert, so a bad merge cannot degrade production until someone opts in.

**Latency.** Adds up to `leaguepediaTimeout` (6s) to a `/lol` reply when a gap exists. Telegram tolerates this, and the short-circuit means it only happens on stall days. If it becomes annoying, cache — deliberately deferred per `plan.md` (the TTL index's partial filter is scoped to `matches:`, so a new prefix would never expire and needs its own index).

**Push ordering.** Step 4 exists because enriching after the claim would let a Cargo failure burn the day's claim and silently skip the digest.
