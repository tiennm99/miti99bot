---
phase: 1
title: Remove Command and Retire Stats
status: completed
priority: P1
dependencies: []
effort: medium
---

# Phase 1: Remove Command and Retire Stats

## Overview

Delete the generic stock dividend command and add durable stats retirement for
its retained usage history.

## Requirements

- Preserve `/stock_cash_dividend` and `/stock_share_dividend` behavior.
- Remove all active `/stock_dividend` command surfaces.
- Retain stats rows rather than deleting usage history.
- Filter retired rows from every memory and Mongo stats view.
- Reconcile late legacy writes on future startup runs.
- Keep Mongo startup bounded with exact-command bulk operations.

## Architecture

The stats runtime owns a permanent retired-command rule. New increments for the
retired command are ignored and reads exclude it independently of the stored
flag. Startup physically restores `deleted: true`: memory uses versioned CAS;
Mongo uses indexed `UpdateMany` plus `CountDocuments`. The shared system marker
stores completion status and the current exact-command row count.

## Implementation Checklist

- [x] Remove command registration and combined handler.
- [x] Remove command-menu metadata and handler/menu tests.
- [x] Remove README command documentation.
- [x] Add stats/system startup wiring and migration marker.
- [x] Add permanent read/write retirement behavior.
- [x] Add memory and real-Mongo reconciliation tests.
- [x] Pass review, test, race, build, vet, lint, and diff gates.

## Risks

- A legacy process can write after migration; runtime filtering hides it and
  every later startup physically tombstones it again.
- Unbounded per-row Mongo startup work is avoided by exact indexed bulk calls.
- Marker count is derived from all matching rows, so retries do not undercount.

