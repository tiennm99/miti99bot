---
title: Compact portfolio number format
status: approved
created: 2026-07-22
tags: [stock, coin, formatting, telegram]
---

# Compact Portfolio Number Format

## Summary

Stock and coin position tables need shorter monetary cells without hiding each
value's magnitude. Use automatic per-cell financial suffixes for the `Avg`,
`Now`, `Value`, and position-level `Unrealized P&L` columns.

## Problem

Full monetary values make Telegram's monospace portfolio tables wide and harder
to scan. A single fixed scale fails because stock and coin prices and position
values span several orders of magnitude.

Evidence is direct user feedback on the rendered portfolio. Success means the
four position columns become visibly narrower while their currencies,
magnitudes, signs, and P&L percentages remain understandable.

## Requirements

- Scale absolute values automatically: base below 1,000, then `k`, `M`, `B`,
  and `T` at successive powers of 1,000.
- Show at most three fractional digits and trim trailing zeroes.
- Preserve each module's convention: stock uses dot grouping and comma decimal;
  coin uses `$`, comma grouping, and dot decimal.
- Keep signs and P&L percentages unchanged.
- Apply compact formatting only to position-table `Avg`, `Now`, `Value`, and
  `Unrealized P&L` monetary amounts.
- Keep summary tables and standalone trade, price, top-up, and dividend replies
  unchanged.
- Keep `N/A` behavior unchanged when a price or valuation is unavailable.

## Evaluated Approaches

### Automatic magnitude per value — selected

Examples: stock `25,35k`, `126M`, `1,25B`; coin `$950`, `$50k`, `$1.234M`.
Each value carries its scale, so mixed rows remain unambiguous. Three fractional
digits balance width and display precision.

### Fixed unit per column

Predictable alignment, but produces awkward small values such as `0,125M` and
cannot fit the full range of coin prices cleanly.

### Unit in headers

Produces the narrowest cells, but one header scale cannot represent mixed
magnitudes without exceptions and ambiguity.

## Decision

Use the common financial suffix set `k/M/B/T`. Do not use strict SI `G` for
billion because it is unfamiliar in financial displays. Do not use Vietnamese
`tr/tỷ` because the requested stock-and-coin style should remain uniform and
compact.

## Implementation Considerations

- Keep compact helpers private to their modules so exported formatting
  contracts remain stable.
- Select a suffix from the absolute value, then preserve the original sign.
- Round to at most three scaled fractional digits before trimming zeroes.
- Add boundary tests around 1,000, 1M, 1B, 1T, negatives, and rounding rollover.
- Update renderer tests for both fully priced and unavailable-price paths.

## Success Criteria

- `999` remains unscaled; `1,000` becomes `1k`; `1,000,000` becomes `1M`.
- Stock examples use `25,35k` and `126M`.
- Coin examples use `$50k` and `$1.234M`.
- Position P&L amounts compact while percentages remain exact to two decimals.
- Summary and non-portfolio messages retain current output.
- Focused tests, full tests, vet, and lint pass.

## Next Steps

Create an implementation plan, revise the preliminary stock-only formatter,
add the coin formatter and renderer integration, then verify both modules.

## Unresolved Questions

None.
