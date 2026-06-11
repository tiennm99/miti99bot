---
title: "Gold module plan review"
date: 2026-06-11
status: completed
reviewers: [planner, researcher, codebase-fit]
---

# Gold module plan review

## Summary

Three ClaudeKit sub-agents reviewed `plans/260611-0735-gold-module-trading-parity/`. All completed with `DONE_WITH_CONCERNS`; no blocker, but plan needed tightening before implementation.

## Accepted Findings

- Pricing decision was inconsistent: plan used spot XAU but left spot-vs-SJC unresolved while considering default enablement.
- GoldPrice.org endpoint works today but is undocumented; treat as best-effort soft dependency.
- ExchangeRate-API open endpoint needs attribution awareness, cache handling, and 429 handling.
- Free/no-key SJC JSON source is not credible for v1; exact SJC retail price is out of scope.
- Fractional `luong` arithmetic needed explicit epsilon/dust behavior.
- Parser tests must cover `NaN`, `Inf`, `+Inf`, `-Inf`, and overflow values accepted by `strconv.ParseFloat`.
- Real `cmd/server` factory wiring and trading/gold namespace isolation needed tests.

## Plan Changes Applied

- V1 pricing locked to world spot XAU converted to VND per `luong`.
- `gold` kept opt-in for first deploy; default `template.yaml` enablement deferred.
- Runtime provider override and HTTPS/localhost validation added to price-client phase.
- FX cache and 429 handling added.
- Dust rule added: absolute balance below `1e-9` normalizes to zero.
- Special-float, catalog wiring, and namespace-isolation tests added.

## Unresolved Questions

None.
