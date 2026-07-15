---
title: Bold Wrapped Wheel Labels
description: >-
  Render wheel option labels in bold and wrap long text into balanced lines
  without changing the API or wheel geometry.
status: completed
priority: P2
branch: main
tags:
  - feature
  - frontend
blockedBy: []
blocks: []
created: '2026-07-15T03:47:02.124Z'
createdBy: 'ck:plan'
source: skill
---

# Bold Wrapped Wheel Labels

## Overview

Replace whole-label shrink-to-fit behavior with deterministic multi-line layout. Short labels remain one line; long labels are split at sensible boundaries, balanced across the available radial track, and sized from the longest rendered line. The Remotion composition renders the returned lines at bold weight and clips the text block to its label area.

## Scope

- Modify label layout and rendering only.
- Preserve `/api` request/response schemas, themes, and wheel geometry.
- Use JavaScript + JSDoc and existing Remotion/CSS patterns; add no custom font or SVG work.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Implementation](./phase-01-implementation.md) | Completed |
| 2 | [Verification](./phase-02-verification.md) | Completed |

## Dependencies

- Cross-plan: none.
- Runtime: existing React, Remotion, and CSS rendering stack.
- Quality gates: Node.js 24, `pnpm lint`, `pnpm typecheck`, `pnpm test`, and `pnpm render:smoke`.

## Acceptance Criteria

- Labels render with a bold font weight.
- Long labels become balanced multiple lines within the radial label track.
- Font sizing uses the longest rendered line, not the original full label.
- Short labels remain one line.
- Multi-line content remains clipped within the computed label area.
- API/schema, themes, and wheel geometry remain unchanged.
