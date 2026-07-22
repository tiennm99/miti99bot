---
date: 2026-07-22
component: internal/modules/stock
status: completed
---

# Stock Portfolio Currency Title

## Context

The stock portfolio repeated `VND` in every monetary table cell.

## What Happened

The renderer now declares `VND` once in the `Stock Portfolio (VND)` title,
labels the balance row `Cash`, and uses suffix-free monetary values in the
portfolio table. Standalone stock replies retain their explicit `VND` suffix.

## Reflection

A portfolio-specific display helper kept this presentation change from altering
the exported formatters used by trades, prices, and dividends.

## Decisions

- Keep command, storage, and exported formatting contracts unchanged.
- Cover the title, cash label, suffix-free values, and single currency marker in
  the portfolio regression test.

## Next

No follow-up required. Focused and full tests, vet, and lint passed.
