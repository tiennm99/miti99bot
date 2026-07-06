---
title: Wheel Beta Canvas Renderer
description: >-
  Migrate /wheelofnamesbeta frame rendering from manual pixel drawing to
  github.com/tdewolff/canvas while preserving Telegram behavior.
status: completed
priority: P2
branch: main
tags:
  - feature
  - backend
  - graphics
blockedBy: []
blocks: []
created: '2026-07-06T07:48:34.819Z'
createdBy: 'ck:plan'
source: skill
---

# Wheel Beta Canvas Renderer

## Overview

Migrate `/wheelofnamesbeta` frame rendering to `github.com/tdewolff/canvas` so wheel slices, text, highlights, rim, pointer, and celebration effects are drawn through a higher-level vector renderer. Keep the existing Telegram command contract and stdlib GIF encoding.

Expected output: `/wheelofnamesbeta <options>` still sends a 320x320 animated GIF named `wheelofnamesbeta.gif`, but frames are generated through canvas-backed rendering instead of manual low-level pixel/text stamping.

Acceptance criteria:
- GIF frame count, loop count, delays, duration, width, and height stay unchanged.
- Winner/current option tracking remains aligned with the 3h pointer.
- Vietnamese option text still renders without `?` glyph replacement.
- Slice labels still rotate with their slices and appear inside the wheel.
- `/wheelofnamesbeta` send behavior and caption stay unchanged.
- Focused misc tests, `go test ./...`, `go vet ./...`, and `golangci-lint run` pass.

Scope boundary:
- Do not add `gifski`, ffmpeg, shell commands, cgo graphics stacks, or external runtime binaries.
- Do not change command names, command menu metadata, Telegram API payload shape, or spin-profile math.
- Do not refactor unrelated misc commands.

Constraints:
- Go-only/headless deployment for Coolify.
- Keep embedded DejaVu Sans font use; no runtime font discovery.
- Keep changes localized to `internal/modules/misc` and module dependency files unless tests require otherwise.
- Preserve public/internal function signatures where tests and command code depend on them.

Scout context:
- Project is a Go 1.25 Telegram bot with pluggable modules under `internal/modules`.
- Current wheel beta code lives in `internal/modules/misc/wheelofnames_beta.go`, `wheelofnames_beta_drawing.go`, `wheelofnames_beta_spin_profile.go`, and `wheelofnames_beta_command.go`.
- Existing tests in `internal/modules/misc/handlers_test.go` cover GIF timing, frame transitions, pointer alignment, label placement/rotation, Vietnamese glyph support, and Telegram send payloads.
- Prior completed plan `plans/260703-0404-wheelofnames-misc-command` is complete and non-blocking.
- Research report `plans/reports/260706-1441-wheel-beta-gif-rendering-library-research.md` recommends `tdewolff/canvas` as renderer and keeping `image/gif` first.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Add Canvas Dependency](./phase-01-add-canvas-dependency.md) | Completed |
| 2 | [Migrate Renderer](./phase-02-migrate-renderer.md) | Completed |
| 3 | [Validate Behavior](./phase-03-validate-behavior.md) | Completed |

## Dependencies

Cross-plan dependencies: none.

Implementation dependencies:
- Add `github.com/tdewolff/canvas` via `go get`.
- Continue using stdlib `image/gif` for animation encoding.
- Continue using current embedded `internal/modules/misc/fonts/dejavu-sans.ttf`.

## Touchpoints

| File | Action |
|---|---|
| `go.mod` / `go.sum` | Add canvas module and transitive dependency metadata. |
| `internal/modules/misc/wheelofnames_beta.go` | Route frame generation through canvas-backed renderer while preserving GIF contract. |
| `internal/modules/misc/wheelofnames_beta_drawing.go` | Replace or reduce manual draw helpers with canvas equivalents where practical. |
| `internal/modules/misc/handlers_test.go` | Adjust/add tests only where rendering internals legitimately change. |

## Verification

- `go test ./internal/modules/misc -count=1`
- `go test ./...`
- `go vet ./...`
- `golangci-lint run`
