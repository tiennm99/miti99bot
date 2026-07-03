---
phase: 3
title: Validate behavior
status: completed
priority: P2
dependencies:
  - 1
  - 2
---

# Phase 3: Validate behavior

## Overview

Add focused tests for the new handler and run command-surface validation required for command changes.

## Requirements

- Functional: tests cover registration, usage reply, one-option selection, trimming, and empty segment handling.
- Non-functional: run focused tests first, then full suite and vet because command/menu/shared behavior changed.

## Architecture

Use existing misc test harness:
- `installMisc` in `internal/modules/misc/handlers_test.go` wires the misc module to `testutil.RecordingBot`.
- `TestNew_RegistersExpectedCommands` in `internal/modules/misc/misc_test.go` locks command names and visibility.

Avoid brittle randomness assertions:
- Single-option input should return that option exactly.
- Multi-option input should assert the reply is one of the non-empty trimmed options.

## Related Code Files

- Modify: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/misc_test.go`
- Modify: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/handlers_test.go`
- Create: none
- Delete: none

## Implementation Steps

1. Update `TestNew_RegistersExpectedCommands` expected map:
   - add `wheelofnames: modules.VisibilityPublic`
2. Add handler tests:
   - `/wheelofnames` returns usage.
   - `/wheelofnames , ,` returns usage.
   - `/wheelofnames Alice` returns `Alice`.
   - `/wheelofnames Alice, Bob, Carol` returns one of those exact strings.
   - `/wheelofnames , Alice , , Bob ,` returns `Alice` or `Bob`, never empty/spaced.
3. Run formatting and focused tests:
   - `gofmt -w internal/modules/misc/misc.go internal/modules/misc/misc_test.go internal/modules/misc/handlers_test.go`
   - `go test ./internal/modules/misc`
4. Run broader validation because command menu/user-facing contract changes:
   - `go test ./cmd/server ./internal/modules/misc`
   - `go test ./...`
   - `go vet ./...`

## Success Criteria

- [x] Focused misc tests pass.
- [x] Command menu/server tests pass.
- [x] Full `go test ./...` passes.
- [x] Full `go vet ./...` passes.

## Risk Assessment

Risk: random selection makes tests nondeterministic. Mitigate by avoiding exact expected value for multi-option tests.

Rollback: remove `wheelOfNamesCommand`, helper, tests, README entry, and `telegram-commands.json` entry.
