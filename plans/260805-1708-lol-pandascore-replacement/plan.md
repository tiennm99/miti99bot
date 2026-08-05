---
title: Replace LoL schedule source with PandaScore
description: >-
  Swap the lol module's upstream from the lolesports.com gql persisted-query
  client to PandaScore's REST API as the sole schedule/results source
status: completed
priority: P2
branch: main
tags:
  - lol
  - external-api
  - migration
blockedBy: []
blocks: []
created: '2026-08-05T10:12:46.240Z'
createdBy: 'ck:plan'
source: skill
---

# Replace LoL schedule source with PandaScore

## Overview

Replace the lolesports.com `/api/gql` persisted-query transport (commit `b76d0ca`)
with PandaScore's REST API as the **only** upstream for the lol module. User
decision (2026-08-05): full replacement, not fallback — accepting the trade-off
that official-source data and the gql client are dropped in exchange for a
stable, versioned, documented API contract that does not break on Riot frontend
deploys. Free tier: 1000 req/h, Bearer token (user already has one).

Public contract unchanged: `ScheduleEvent` shape, formatters, handlers, cron,
subscriber flows, and the 60-min stale-cache fallback all stay. Only the
transport + response mapping in `api_client.go` change, mirroring the b76d0ca
rewrite pattern.

Supersedes plan `260726-0952-lol-leaguepedia-score-enrichment`: PandaScore's
match `results` carry final series scores directly, absorbing the
score-enrichment job. That plan is marked cancelled in Phase 3.

Evidence: `plans/reports/research-260805-1627-stable-free-lol-schedule-source-report.md`.

## Key design decisions

- **One mapping table, client-side.** PandaScore league slugs differ from
  Riot's; the client maps PandaScore league → existing canonical slug so
  `format.go`'s `majorLeagueSlugs` allowlist and `leagueOrder` stay untouched.
  Phase 1 discovers the real slugs with the live token.
- **Token via env** `LOL_PANDASCORE_TOKEN`, matching the
  `GOLD_VNAPP_API_KEY` pattern (`internal/modules/gold/vnappmob_client.go:52`).
  Missing token → fetch error at call time (stale cache may still serve);
  never panic at startup.
- **Single window endpoint.** `GET /lol/matches?range[begin_at]=<from>,<to>`
  covers past+running+upcoming in one call; page-walk `per_page=100`.
  Exact `[from,to)` filtering stays client-side (proven pattern).
- **Status mapping**: `not_started→unstarted`, `running→inProgress`,
  `finished→completed`; `canceled`/`postponed` dropped (Phase 1 verifies the
  full status vocabulary).

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Feasibility and mapping discovery](./phase-01-feasibility-and-mapping-discovery.md) | Completed |
| 2 | [PandaScore client rewrite](./phase-02-pandascore-client-rewrite.md) | Completed |
| 3 | [Wire-in cleanup and supersede](./phase-03-wire-in-cleanup-and-supersede.md) | Completed |

Phase 1 is a cheap kill/adjust gate (curl only, needs `LOL_PANDASCORE_TOKEN`
exported); Phases 2-3 must not start before its questions are answered.

## Acceptance criteria

- [x] `/lol`, `/lol_tomorrow`, `/lol_this_week`, `/lol_next_week` render from
      PandaScore data with identical message format — live probe rendered a
      real week: `🔴 LIVE JDG 1–0 LGD · Bo3 (Group Ascend)`, ICT day grouping
- [x] Major-league filter keeps working via slug mapping (LCK/LPL verified
      live; minor leagues dropped)
- [x] Finished series show real scores from `results`; winner bolded
      (unit-tested; outcome gated on `winner_id`)
- [x] No references to lolesports.com remain in module code beyond
      doc-comment history; gql client fully removed
- [x] Missing/invalid token degrades to fetch error + stale cache
      (`lol_token_missing`, no upstream call — unit-tested)
- [x] `go test ./...`, `go vet`, `golangci-lint` green; tests use `httptest`
      fixtures in PandaScore shape, no live calls
- [x] Live smoke: today+tomorrow and this-week windows returned plausible
      events incl. a live LPL match; quota 1000/h confirmed in Phase 1
- [x] Plan 260726-0952 marked cancelled/superseded with pointer here

Code review (code-reviewer subagent): all 9 criteria PASS,
DONE_WITH_CONCERNS; both concerns fixed same day (page-budget warn log +
live budget 3→5; bracket encoding verified by live probe; phantom test
removed).

## Dependencies

- Supersedes: `260726-0952-lol-leaguepedia-score-enrichment` (marked in Phase 3).
- External: PandaScore REST API, free tier, `LOL_PANDASCORE_TOKEN` (user holds token).

## Open questions

- PandaScore full ToS text unreviewed (developer docs show no attribution
  requirement for the free Fixtures plan) — low risk, revisit if the bot's
  audience grows.
- Deployment env (`Coolify`) needs `LOL_PANDASCORE_TOKEN` set by the owner
  before the next deploy; local `.env` already has it.
