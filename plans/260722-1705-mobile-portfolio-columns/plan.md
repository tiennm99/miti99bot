---
title: Mobile Portfolio Columns
description: Compact stock and coin position tables for narrow Telegram screens.
status: completed
priority: P1
effort: small
branch: main
tags:
  - stock
  - coin
  - telegram
  - formatting
created: 2026-07-22
---

# Mobile Portfolio Columns

## Overview

Shorten stock and coin position tables while preserving calculations, summary
formatting, missing-price behavior, and Telegram reply limits.

## Phases

| Phase | Name | Status | Progress |
|---|---|---|---|
| 1 | [Compact Portfolio Columns](./phase-01-compact-portfolio-columns.md) | Completed | 6/6 (100%) |

## Dependencies

- Builds on the completed compact-number formatter plan.
- No storage, command, provider, or migration dependency.

## Success Criteria

- [x] Both position tables use `Sym`, `Qty`, `Avg`, `Now`, `Val`, `P&L`, `%`.
- [x] Stock `Avg` and `Now` use implicit thousand VND without `k`.
- [x] Coin position money uses implicit USD without `$`.
- [x] P&L amount and signed percentage render in separate columns.
- [x] Summary formatting and unavailable-price behavior remain compatible.
- [x] Focused/full tests, build, vet, lint, and diff checks pass.

