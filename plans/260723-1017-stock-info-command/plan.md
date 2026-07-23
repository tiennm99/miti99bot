---
title: Stock Info Command
description: >-
  Add a one-call SSI quote-detail command while preserving /stock_price and its
  provider fallbacks.
status: completed
priority: P2
branch: main
tags:
  - feature
  - stock
  - telegram
  - api
blockedBy: []
blocks: []
created: '2026-07-23T03:17:04.304Z'
createdBy: 'ck:plan'
source: skill
---

# Stock Info Command

## Overview

Add public `/stock_info <ticker>` for a compact SSI quote snapshot. The command
uses exactly one SSI single-ticker GET and reports company, exchange, current
price, since-open and reference changes, open/high/low, and volume. Existing
`/stock_price`, batch quotes, portfolio valuation, and provider fallbacks stay
unchanged.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Model SSI Quote Details](./phase-01-model-ssi-quote-details.md) | Completed |
| 2 | [Add Stock Info Command](./phase-02-add-stock-info-command.md) | Completed |
| 3 | [Verify and Document](./phase-03-verify-and-document.md) | Completed |

## Dependencies

- No cross-plan dependencies; all existing stock plans are completed.
- Reuse the current SSI request headers, client, timeout, and response envelope.
- No storage, portfolio, stats, event, dividend, migration, or environment changes.

## Contract

- Syntax: `/stock_info <ticker>`.
- One SSI request only; never fall back to KBS or VCI.
- Missing optional quote fields render consistently as `N/A`.
- A missing/non-positive matched price returns a friendly no-info response.

## Validation

- Focused provider/handler/metadata tests plus `/stock_price` regression coverage.
- `go test -count=1 ./...`, stock race tests, `go vet ./...`, `go build ./...`,
  `golangci-lint run`, and `git diff --check`.

## Open Questions

None.
