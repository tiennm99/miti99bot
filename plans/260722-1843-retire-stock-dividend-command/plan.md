---
title: Retire Generic Stock Dividend Command
description: Remove /stock_dividend while preserving and hiding historical usage statistics.
status: completed
priority: P1
effort: medium
branch: main
tags:
  - stock
  - stats
  - migration
  - telegram
created: 2026-07-22
---

# Retire Generic Stock Dividend Command

## Overview

Remove the combined manual dividend command because users can apply cash and
share dividends through the two specialized commands. Retain historical stats
as deleted legacy records and prevent rolling-deploy writes from exposing them.

## Phases

| Phase | Name | Status | Progress |
|---|---|---|---|
| 1 | [Remove Command and Retire Stats](./phase-01-remove-command-and-retire-stats.md) | Completed | 7/7 (100%) |

## Dependencies

- Uses the existing stats `deleted` field and shared `system` collection.
- No portfolio schema or dividend-history migration changes.

## Success Criteria

- [x] `/stock_dividend` is absent from registration, menu, help, handler, and README.
- [x] Cash and share dividend commands remain available and unchanged.
- [x] Anonymous and per-user historical rows remain stored with `deleted: true`.
- [x] Retired rows remain hidden even after a late legacy-process write.
- [x] Memory reconciliation is idempotent and Mongo reconciliation is bulk/indexed.
- [x] Exact-prefix commands such as `stock_dividend_extra` remain untouched.
- [x] Full, race, Mongo integration, build, vet, and lint checks pass.

