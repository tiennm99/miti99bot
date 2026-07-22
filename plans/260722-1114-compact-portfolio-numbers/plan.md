---
title: Compact Portfolio Number Formatting
description: >-
  Compact stock and coin portfolio position values with automatic k/M/B/T
  financial suffixes.
status: completed
priority: P2
branch: main
tags:
  - refactor
  - stock
  - coin
  - telegram
blockedBy: []
blocks: []
created: '2026-07-22T04:14:02.795Z'
createdBy: 'ck:plan'
source: skill
---

# Compact Portfolio Number Formatting

## Overview

Replace wide monetary values in stock and coin position tables with automatic
`k/M/B/T` formatting. Apply only to `Avg`, `Now`, `Value`, and position-level
`Unrealized P&L`; preserve summary tables and standalone replies.

Source: [approved brainstorm report](../reports/260722-1112-compact-portfolio-number-format.md).

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Define Compact Formatters](./phase-01-define-compact-formatters.md) | Completed |
| 2 | [Integrate Portfolio Renderers](./phase-02-integrate-portfolio-renderers.md) | Completed |
| 3 | [Verify Formatting Contracts](./phase-03-verify-formatting-contracts.md) | Completed |

## Dependencies

- No cross-plan dependencies.
- Implementation revised the preliminary stock-only `k` work into the approved
  automatic formatter and added matching coin support.
- Go toolchain plus optional `golangci-lint` for verification.

## Boundaries

- No command, storage, API, migration, price-fetching, or P&L calculation changes.
- No compact formatting in summary tables or non-portfolio messages.
- No shared cross-module abstraction unless duplication creates real complexity.
