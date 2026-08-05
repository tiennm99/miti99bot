---
phase: 3
title: Wire-in cleanup and supersede
status: completed
effort: 'S — env docs, live smoke, plan bookkeeping'
dependencies:
  - 2
---

# Phase 3: Wire-in cleanup and supersede

## Overview

Deployment plumbing, live verification with the real token, documentation,
and cross-plan bookkeeping (cancel the superseded Leaguepedia plan).

## Implementation Steps

1. **Env plumbing.** Add `LOL_PANDASCORE_TOKEN` to the deployment environment
   (wherever `GOLD_VNAPP_API_KEY` lives — user applies the secret; never
   commit it). Confirm the bot process picks it up.
2. **Docs.** Update README / docs env-var table if one exists (check
   `README.md`, `docs/`) with `LOL_PANDASCORE_TOKEN` (required for lol module
   fetches; without it /lol replies degrade to fetch-error message).
   Attribution note if Phase 1 ToS check requires it.
3. **Live smoke (temporary probe test, delete after):** today + this-week
   windows through the real client; verify major leagues present, scores on
   finished series, ICT rendering; check quota headers after the run.
4. **Supersede plan 260726-0952:** set `status: cancelled` in its `plan.md`
   frontmatter + add one-line note: "Superseded by
   260805-1708-lol-pandascore-replacement — PandaScore results carry final
   scores, absorbing this enrichment." Do not delete files.
5. **Full gates:** `go test ./...`, `go vet ./...`, `golangci-lint run`.
6. **Commit** (on user request, conventional message, e.g.
   `feat(lol): replace schedule source with PandaScore API`).

## Success Criteria

- [ ] Live /lol and /lol_this_week verified with real token on the server
- [ ] Env var documented; secret nowhere in repo or logs
- [ ] Plan 260726-0952 marked cancelled with supersede pointer
- [ ] All gates green; probe test removed
- [ ] Commit created (user-approved)

## Risk Assessment

- Daily push (08:00 ICT cron) is the first unattended consumer — watch first
  push after deploy; stale cache + error message paths already tested.
- Rollback: revert commit → b76d0ca gql client returns (its persisted ID may
  need re-extraction per that commit's package doc).
