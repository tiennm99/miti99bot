---
title: Wheelofnamesbeta API Integration
description: >-
  Route /wheelofnamesbeta GIF rendering through the self-hosted wheelofnames API
  when configured, with the current local renderer as fallback.
status: completed
priority: P2
branch: main
tags:
  - feature
  - backend
  - api
blockedBy: []
blocks: []
created: '2026-07-06T15:20:57.412Z'
createdBy: 'ck:plan'
source: skill
---

# Wheelofnamesbeta API Integration

## Overview

Integrate the existing Go `/wheelofnamesbeta` command with the new
`wheelofnames` Remotion service by reading the API endpoint from system env.
When `WHEELOFNAMES_API_URL` is set, the command sends the parsed options and
locally selected `winnerIndex` to that endpoint and uploads the returned GIF to
Telegram. When the env is unset, invalid, timed out, unauthorized, or the
service returns non-GIF/non-2xx, the bot keeps using the current local GIF
renderer.

Checked service repo state on 2026-07-06:
- `/config/workspaces/tiennm99/wheelofnames` is clean.
- Latest commit: `4cf72ea fix: align wheel labels and compose env`.
- Service now has dedicated radial label layout logic in
  `src/remotion/wheel-label-layout.js`, tests for label layout, and
  `compose.yml` env defaults via `${VAR:-default}`.
- API schema still accepts `durationMs`, `holdMs`, `fps`, `size`, and `theme`.
- Service composition duration is `durationMs + holdMs`; the requested bot
  payload below renders a 7-second GIF.

Scope Challenge:
- Existing code: `/wheelofnamesbeta` already parses options, picks the winner,
  renders local GIF bytes, sends `sendAnimation`, preserves message threads,
  and falls back to text on local render/upload failure.
- Minimum changes: add a small API client, wire command render path through
  remote-or-local selection, add env docs, and test remote success/failure.
- Complexity: expected 5-7 touched files, one new client file, four focused
  phases. No command rename, stats migration, persistent jobs, or async flow.
- Selected mode: SCOPE REDUCTION / fast plan. The API service already exists;
  this plan only integrates the command.

## Architecture

```text
/wheelofnamesbeta message
  |
  | splitWheelOptions + pickWheelOption (existing)
  v
renderWheelOfNamesBetaAnimation(ctx, options, winner)
  |
  | if WHEELOFNAMES_API_URL unset
  |   -> existing renderWheelOfNamesBetaGIF fallback
  |
  | POST WHEELOFNAMES_API_URL
  | Authorization: Bearer $WHEELOFNAMES_API_TOKEN when set
  | JSON: { options, winnerIndex, durationMs, holdMs, fps, size, theme }
  v
remote GIF bytes -> Telegram sendAnimation
  |
  | on any remote error
  v
existing local GIF renderer -> Telegram sendAnimation
```

Environment contract:
- `WHEELOFNAMES_API_URL`: optional full endpoint URL, expected to include
  `/api/gif`, for example `http://wheelofnames:3000/api/gif`.
- `WHEELOFNAMES_API_TOKEN`: optional Bearer token. Required for production
  `wheelofnames` service deployments because that service refuses production
  startup without `API_TOKEN`.

Remote render request values should be fixed in code for now:

```json
{
  "options": ["alice", "bob", "carol"],
  "winnerIndex": 1,
  "durationMs": 6000,
  "holdMs": 1000,
  "fps": 20,
  "size": 512,
  "theme": "classic"
}
```

Do not add user-facing flags, persistent storage, or dynamic theme selection in
this integration pass.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [API Client And Env Config](./phase-01-api-client-and-env-config.md) | Completed |
| 2 | [Command Integration And Fallback](./phase-02-command-integration-and-fallback.md) | Completed |
| 3 | [Deployment Docs And Env Surfaces](./phase-03-deployment-docs-and-env-surfaces.md) | Completed |
| 4 | [Validation And Regression Tests](./phase-04-validation-and-regression-tests.md) | Completed |

## Dependencies

## Cross-Plan Dependencies

| Relationship | Plan | Status |
|---|---|---|
| Uses output from | `plans/260706-1441-wheelofnames-remotion-api/plan.md` | completed |

No unfinished project plans overlap this scope.

## Key Files

| File | Action |
|---|---|
| `/config/workspace/tiennm99/miti99bot/internal/modules/misc/wheelofnames_beta_api_client.go` | Create remote API client and env loader |
| `/config/workspace/tiennm99/miti99bot/internal/modules/misc/wheelofnames_beta_command.go` | Route command through remote-or-local renderer |
| `/config/workspace/tiennm99/miti99bot/internal/modules/misc/wheelofnames_beta.go` | Keep local renderer unchanged as fallback |
| `/config/workspace/tiennm99/miti99bot/internal/modules/misc/handlers_test.go` | Add command-level remote success/fallback tests |
| `/config/workspace/tiennm99/miti99bot/internal/modules/misc/wheelofnames_beta_api_client_test.go` | Create focused client tests |
| `/config/workspace/tiennm99/miti99bot/.env.example` | Document optional API env vars |
| `/config/workspace/tiennm99/miti99bot/compose.yml` | Add commented Coolify/private-network env hints |
| `/config/workspace/tiennm99/miti99bot/docs/deploy-coolify-selfhosted.md` | Document deployment wiring with wheelofnames service |

## Acceptance Criteria

- With `WHEELOFNAMES_API_URL` unset, `/wheelofnamesbeta` behaves like today.
- With `WHEELOFNAMES_API_URL` set and remote returns `200 image/gif`, the bot
  uploads those GIF bytes via `sendAnimation`.
- The remote request uses `durationMs=6000`, `holdMs=1000`, `fps=20`,
  `size=512`, and `theme=classic` by default.
- The request body contains the original parsed options and the locally chosen
  `winnerIndex`; the service must not be the source of truth for the winner.
- If `WHEELOFNAMES_API_TOKEN` is set, request includes
  `Authorization: Bearer <token>` and tests verify it.
- Remote failures fall back to the existing local GIF renderer and do not reveal
  the winner in the caption.
- Timeout is bounded with an HTTP client timeout; no goroutine can hang on a
  wedged render service.
- Docs and env examples explain that URL is optional and token is required for
  production wheelofnames service auth.
- Focused tests pass, then `go test ./...` and `go vet ./...` pass.

## Out Of Scope

- Replacing `/wheelofnames`; only `/wheelofnamesbeta` changes.
- Removing the local Go GIF renderer.
- Async job polling, object storage, or cached GIF URLs.
- User-selectable themes, sizes, fps, or command syntax changes.
- Stats migrations or Telegram command menu changes.

## Unresolved Questions

None.
