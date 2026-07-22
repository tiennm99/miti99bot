---
phase: 1
title: Define Compact Formatters
status: completed
priority: P1
dependencies: []
effort: small
---

# Phase 1: Define Compact Formatters

## Overview

Define private compact monetary formatters for stock VND and coin USD while
preserving the modules' exported formatting contracts.

## Requirements

- Functional: select base, `k`, `M`, `B`, or `T` from absolute magnitude.
- Functional: show at most three fractional digits and trim trailing zeroes.
- Functional: promote after rounding rollover, so `999.9999k` becomes `1M`.
- Functional: preserve negative signs and module-native separators.
- Non-functional: keep `FormatVND`, `FormatPnL`, `FormatUSD`, and
  `FormatPnLUSD` output unchanged outside position tables.

## Architecture

Each module owns a private compact formatter because stock uses dot grouping
plus comma decimals, while coin uses `$`, comma grouping, and dot decimals.
Both follow the same magnitude table: base `< 10^3`, then powers of 1,000
through `T`. P&L wrappers reuse existing percentage calculation behavior while
substituting only the monetary formatter.

## Related Code Files

- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/format.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/stock/format_test.go`
- Modify: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/coin/format.go`
- Create: `C:/Users/miti99/Workspaces/tiennm99/miti99bot/internal/modules/coin/format_test.go`

## Implementation Steps

1. Replace the preliminary stock `k`-only helper with automatic suffix selection.
2. Preserve stock grouping, decimal comma, sign, and nearest-VND input rounding.
3. Add the equivalent USD helper with `$` and English separators.
4. Add private compact P&L wrappers that retain signed two-decimal percentages.
5. Test base/`k`/`M`/`B`/`T` boundaries, trimmed fractions, negatives, zero,
   rounding rollover, and unchanged exported formatter results.

## Success Criteria

- [x] Stock examples include `999`, `1k`, `25,35k`, `126M`, and `1,25B`.
- [x] Coin examples include `$999.99`, `$1k`, `$50k`, and `$1.234M`.
- [x] Boundary rollover never emits `1000k`, `1000M`, or `1000B`.
- [x] Exported stock and coin formatters retain their current test outputs.

## Risk Assessment

Main risks: threshold off-by-one errors, locale separator drift, negative-sign
duplication, and rounding into the next suffix. Table-driven boundary tests
mitigate all four. No security or data-protection impact; formatting is local
and receives already-validated numeric values.
