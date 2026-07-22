---
phase: 1
title: Compact Portfolio Columns
status: completed
priority: P1
dependencies: []
effort: small
---

# Phase 1: Compact Portfolio Columns

## Overview

Apply the approved seven-column mobile layout to stock and coin position rows.

## Requirements

- Use `Sym | Qty | Avg | Now | Val | P&L | %` in both modules.
- Render stock `Avg` and `Now` in thousand VND without a `k` suffix.
- Omit currency from both titles and omit `$` from coin position money.
- Keep compact suffixes for stock value/P&L and coin position money.
- Preserve full summary formats, P&L calculations, and `N/A` behavior.

## Architecture

Only private position formatters and row renderers change. Exported formatters,
portfolio persistence, trading operations, quote providers, summaries, and
dividend workflows remain unchanged.

## Implementation Checklist

- [x] Add stock thousand-VND position formatter.
- [x] Split stock position P&L amount and percentage.
- [x] Remove `$` from coin position-only compact formatting.
- [x] Split coin position P&L amount and percentage.
- [x] Update renderer, edge-path, and formatter tests.
- [x] Update README and pass all verification gates.

## Risks

- Seven-column `N/A` rows could become misaligned; covered by partial and
  overflow tests.
- Removing currency markers is intentionally limited to position displays;
  coin summaries retain `$`, while stock uses implicit VND throughout.

