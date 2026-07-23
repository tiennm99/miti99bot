---
phase: 1
title: Model SSI Quote Details
status: completed
priority: P2
dependencies: []
---

# Phase 1: Model SSI Quote Details

## Overview

Extend the existing SSI single-stock response with a read-only detailed quote
model and a dedicated one-request fetch method. Do not change `FetchPrice`.

## Requirements

- Functional: capture symbol, Vietnamese/English company name, exchange,
  matched/open/reference/high/low prices, and normal traded quantity from SSI.
- Functional: perform one GET to `/stock/<ticker>` with existing SSI headers.
- Functional: reject missing/non-positive matched price as `ErrNoPrice`.
- Non-functional: preserve KBS/VCI/SSI fallback behavior for `FetchPrice` and
  batch portfolio pricing.

## Architecture

Add an SSI-specific quote DTO and dedicated fetch method on `PriceClient`.
Reuse `newSSIRequest`, `doSSIJSON`, `baseURL`, and the shared HTTP client.
The command receives the single decoded response directly; it never calls
`FetchPrice`.

## Related Code Files

- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\prices_ssi.go`
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\prices_test.go`

## Implementation Steps

1. Extend the SSI quote struct with only the fields used by `/stock_info`.
2. Add a dedicated detailed-quote fetch method that issues one GET.
3. Keep current `fetchSSIPrice`, `FetchPrice`, and `FetchPrices` contracts intact.
4. Test path, headers, one-call count, decoded fields, invalid price, HTTP, and decode errors.

## Success Criteria

- [x] Detailed quote fetch uses exactly one SSI GET.
- [x] Required and optional fields decode without inventing fallback requests.
- [x] Existing price and batch provider tests remain unchanged and pass.

## Risk Assessment

SSI is undocumented and may omit fields. Keep optional fields nullable by value
and make the handler degrade to `N/A`; isolate the new method from trading and
portfolio callers.
