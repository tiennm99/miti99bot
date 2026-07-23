---
phase: 2
title: "Add Stock Info Command"
status: completed
priority: P2
dependencies: [1]
---

# Phase 2: Add Stock Info Command

## Overview

Register `/stock_info <ticker>` and render a compact, sender-independent quote
snapshot from the Phase 1 SSI detail method.

## Requirements

- Functional: exact syntax `/stock_info <ticker>` and public metadata `<ticker>`.
- Functional: normalize ticker; handle usage, unknown ticker, no price, upstream,
  and decode failures with friendly replies.
- Functional: show Vietnamese company name with English fallback, exchange,
  current price, since-open amount/percent, reference amount/percent,
  open/high/low, and normal traded quantity.
- Non-functional: read-only, senderless, one SSI request, Telegram-safe text.

## Architecture

Create a narrow handler/formatter file. Calculate changes from matched price
against positive open/reference prices; otherwise show `N/A`. Reuse existing
VND, sign, and integer formatting where their output matches the command.

## Related Code Files

- Create: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock_info.go`
- Create: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock_info_test.go`
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock.go`
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers_test.go`
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\cmd\server\command_menu_test.go`

## Implementation Steps

1. Implement exact argument validation and ticker normalization.
2. Fetch with `chathelper.FetchContext` through the one-call SSI detail method.
3. Format compact fields with signed amount/percent and `N/A` for unavailable comparisons.
4. Register the command and update shared command-discovery expectations.
5. Test calculations, signs, missing fields, company fallback, senderless use,
   request count, usage, no-price, and upstream errors.

## Success Criteria

- [x] `/stock_info TCB` returns the agreed compact quote in one SSI call.
- [x] Since-open and reference changes are mathematically correct and signed.
- [x] `/stock_price` registration, output, and fallbacks remain unchanged.

## Risk Assessment

Avoid calling `FetchPrice` from the new handler because it can trigger multiple
providers. Keep the detailed quote method private to the new command path.
