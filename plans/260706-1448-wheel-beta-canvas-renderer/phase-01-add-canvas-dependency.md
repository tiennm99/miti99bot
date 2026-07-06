---
phase: 1
title: Add Canvas Dependency
status: completed
effort: ''
priority: P2
dependencies: []
---

# Phase 1: Add Canvas Dependency

## Overview

Add `github.com/tdewolff/canvas` as the renderer dependency and inspect its local API surface enough to choose the smallest integration path.

## Requirements

- Functional: project builds with the new dependency available to `internal/modules/misc`.
- Non-functional: avoid GUI/game/native-runtime dependencies; keep dependency addition Go-only.

## Architecture

Dependency is used only by the wheel beta renderer. GIF encoding remains in `image/gif`; canvas is only responsible for drawing frame imagery into a Go image.

## Related Code Files

- Modify: `go.mod`
- Modify: `go.sum`
- Read: module docs/source under Go module cache as needed.

## Implementation Steps

1. Run `go get github.com/tdewolff/canvas@latest`.
2. Inspect the installed package APIs for raster image rendering and font/text support.
3. Record any API constraints directly in implementation notes or comments only if useful.

## Success Criteria

- [x] `go.mod` includes `github.com/tdewolff/canvas`.
- [x] No native runtime package such as raylib/cairo/skia is introduced.
- [x] `go test ./internal/modules/misc -run TestWheelOfNamesBeta_RenderGIFTiming -count=1` still compiles or fails only on not-yet-migrated code.

## Risk Assessment

Risk: canvas latest is pseudo-version and API may differ from docs. Mitigation: inspect local package docs/source before editing renderer.
