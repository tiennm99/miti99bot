---
phase: 3
title: Validate Behavior
status: completed
effort: ''
priority: P1
dependencies:
  - 2
---

# Phase 3: Validate Behavior

## Overview

Verify the canvas-backed renderer preserves command behavior and does not introduce lint, vet, or repo-wide test failures.

## Requirements

- Functional: all acceptance criteria from `plan.md` are verified by tests or code inspection.
- Non-functional: lint and vet pass before any commit.

## Architecture

Validation centers on existing misc tests plus repo-wide Go gates because command behavior and renderer internals share misc package files.

## Related Code Files

- Read/verify: `internal/modules/misc/handlers_test.go`
- Read/verify: `internal/modules/misc/wheelofnames_beta*.go`
- Read/verify: `go.mod`

## Implementation Steps

1. Run focused wheel beta tests first.
2. Run full misc package tests.
3. Run `go test ./...`.
4. Run `go vet ./...`.
5. Run `golangci-lint run`.
6. Review diff for public contract changes and unintended docs/command surface changes.

## Success Criteria

- [x] Focused wheel beta tests pass.
- [x] `go test ./internal/modules/misc -count=1` passes.
- [x] `go test ./...` passes.
- [x] `go vet ./...` passes.
- [x] `golangci-lint run` passes.
- [x] Diff contains no command rename, menu, storage, or Telegram payload contract changes.

## Risk Assessment

Risk: transitive dependencies increase lint/test time. Mitigation: measure via standard gates and keep changes scoped.
