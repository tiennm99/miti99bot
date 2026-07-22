---
phase: 3
title: Verify Formatting Contracts
status: completed
priority: P1
dependencies:
  - 2
effort: small
---

# Phase 3: Verify Formatting Contracts

## Overview

Verify formatter correctness, both portfolio flows, and repository-wide quality
without expanding the feature scope.

## Requirements

- Functional: every approved example and boundary has automated coverage.
- Non-functional: changed Go files are formatted; tests, vet, and lint pass.
- Non-functional: public functions, commands, schemas, and persistence remain
  unchanged.

## Architecture

Verification proceeds from deterministic formatter tests to renderer tests,
then the full repository gates. A final diff review maps each changed call site
to the requested four columns and checks that unrelated formatting is untouched.

## Related Code Files

- Inspect: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock`
- Inspect: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/coin`
- Inspect: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/README.md`
- Inspect: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/docs`

## Implementation Steps

1. Run `gofmt` on every changed Go file and `git diff --check`.
2. Run `go test ./internal/modules/stock ./internal/modules/coin`.
3. Run `go test ./...` and `go vet ./...`.
4. Run `golangci-lint run` when available, matching the repository CI gate.
5. Review the final diff for scope, exported-contract stability, secrets, and
   accidental summary or standalone-message changes.
6. Decide whether README/docs need an evergreen update; avoid documentation
   churn when tests and the approved report sufficiently capture presentation.

## Success Criteria

- [x] All focused and repository-wide tests pass.
- [x] `go vet ./...` passes.
- [x] `golangci-lint run` passes when installed.
- [x] `git diff --check` is clean.
- [x] Review finds no changes to commands, storage, migrations, APIs, or quotes.
- [x] Final portfolio examples match the approved brainstorm report.

## Risk Assessment

The main risk is incomplete branch coverage for partial or overflowed prices.
Run existing partial/overflow tests and add only focused assertions if a gap is
found. Rollback is a localized renderer/formatter revert; no data migration is
needed. Security impact is none beyond the standard staged-diff secret scan.
