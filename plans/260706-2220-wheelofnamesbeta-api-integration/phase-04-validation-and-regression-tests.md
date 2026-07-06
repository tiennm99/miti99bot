---
phase: 4
title: Validation And Regression Tests
status: completed
priority: P2
dependencies:
  - 1
  - 2
  - 3
---

# Phase 4: Validation And Regression Tests

## Overview

Add focused regression tests and run the repo quality gates required for a
command integration touching HTTP, env config, and Telegram upload behavior.

## Requirements

- Functional: Test remote success, remote failure fallback, token header, and
  malformed/non-GIF responses.
- Functional: Existing local renderer tests remain valid.
- Non-functional: Run lint/test gates required by `AGENTS.md`.
- Non-functional: Do not require a live wheelofnames service for unit tests.

## Architecture

Use `httptest.Server` for the wheelofnames API client. Keep tests in package
`misc` so they can use unexported helpers and existing test utilities.

Test matrix:

| Area | Scenario | Expected |
|---|---|---|
| Client | valid `200 image/gif` | returns GIF bytes |
| Client | default request payload | sends 6000ms spin, 1000ms hold, 20fps, 512px, classic |
| Client | token configured | sends `Authorization: Bearer ...` |
| Client | `401`/`500` | returns error |
| Client | `text/plain` 200 | returns error |
| Client | bad URL scheme | returns error |
| Command | env unset | local GIF path still sends animation |
| Command | remote success | upstream receives request; bot sends animation |
| Command | remote failure | bot sends local animation |
| Command | remote success in thread | message thread id preserved |

## Related Code Files

- Create: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/wheelofnames_beta_api_client_test.go`
- Modify: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/handlers_test.go`
- Verify: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/misc_test.go`

## Implementation Steps

1. Write client tests before wiring command behavior where practical.
2. Add command tests with `t.Setenv` and `httptest.Server`.
3. Confirm tests do not depend on real network.
4. Run focused package tests:
   ```sh
   go test ./internal/modules/misc
   ```
5. Run full gates:
   ```sh
   go test ./...
   go vet ./...
   ```
6. If docs changed only, no Telegram command registration update is needed.

## Success Criteria

- [ ] `go test ./internal/modules/misc` passes.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] Tests prove winner index is sent to remote API.
- [ ] Tests prove default render config is sent to remote API.
- [ ] Tests prove fallback keeps user-visible behavior stable.

## Risk Assessment

- Risk: Tests become flaky due package-level env.
  Mitigation: use `t.Setenv`, avoid `t.Parallel` in env-sensitive tests.
- Risk: Recording bot cannot inspect uploaded file bytes.
  Mitigation: assert upstream request count/body plus `sendAnimation` metadata;
  client tests validate returned bytes.
