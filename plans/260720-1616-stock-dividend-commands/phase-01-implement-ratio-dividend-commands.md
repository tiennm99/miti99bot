---
phase: 1
title: Implement Ratio Dividend Commands
status: completed
effort: ''
priority: P1
dependencies: []
---

# Phase 1: Implement Ratio Dividend Commands

<!-- Updated: Validation Session 1 - manual-input policy and ratio/cash contracts -->

## Context Links

- [Approved brainstorm](../reports/260720-1612-stock-dividend-command-brainstorm.md)
- [VSDC ratio research](../reports/260720-1608-dividend-notice-ratio-research.md)

## Overview

Replace the two ambiguous adjustment handlers with cash-only, share-only, and
combined dividend contracts. Centralize validated ratio math so all handlers
use the same overflow-safe, floor-rounded calculation.

## Requirements

- Functional: register `stock_cash_dividend`, `stock_share_dividend`, and
  `stock_dividend` with the approved argument order and clear usage examples.
- Functional: accept cash only as positive whole VND per share; reject signs,
  decimals, zero, parse overflow, and non-finite representations.
- Functional: accept only positive whole-number `owned:new` parts; reject
  missing/extra colons, decimals, signs, zero, parse overflow, and invalid ticker.
- Functional: accept equivalent unreduced ratios and preserve the user's exact
  valid ratio text in the success reply; do not require or display reduction.
- Functional: compute `floor(held * new / owned)` without overflowing `int64`;
  compute the minimum holding for a non-zero share result safely.
- Functional: share-only rejects a zero result and reports that minimum;
  combined credits cash and reports zero shares.
- Non-functional: cash and shares use the same pre-event holding; mutate only
  after all validation; call `SavePortfolio` exactly once per successful event.
- Non-functional: allow intentional repeated calls; add no event ID, notice
  lookup, history ledger, or duplicate-event guard.

## Architecture

Add a small dividend calculation helper beside the stock handlers. Parse cash
and ratio parts into positive integers while retaining the validated ratio
string for replies. Calculate quotient and remainder before
multiplication (or use checked operations) to preserve floor semantics without
`held * new` overflow. Each handler loads once, snapshots `held`, calculates
all outputs, mutates the in-memory portfolio, then saves once. The combined
handler must not let newly issued shares participate in its cash calculation.
Treat every successful invocation as an intentional manual adjustment.

## Related Code Files

- Create: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividends.go` — ratio parsing and checked entitlement math.
- Create: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividends_test.go` — parser, floor, minimum, and overflow boundaries.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers.go` — three handlers and user-facing replies.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers_test.go` — contracts, failures, pre-event basis, and one-save behavior.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock.go` — registry names, descriptions, handlers.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\portfolio_test.go` — portfolio outcomes where helper coverage belongs.

## Implementation Steps

1. Write table tests first for whole-VND parsing, `4:1`, `100:10`, equivalent
   unreduced ratios, malformed inputs, `int64` boundaries, exact division,
   floor division, preserved ratio text, and safe minimum holdings.
2. Add handler tests for usage text, invalid cash/ratio/ticker, no holdings,
   zero-result divergence, and persistence failures.
3. Add combined tests proving cash and shares derive from the same snapshot and
   one store write commits both; use an instrumented test store to count saves.
4. Implement parsing and overflow-safe integer entitlement helpers. Require
   positive whole VND/share and reject fractional or overflowing totals.
5. Replace `handleBonus`/old cash `handleDividend` routing with explicit cash,
   share, and combined handlers. Replies include ratio, old holding, cash, new
   shares, and final holding as applicable.
6. Update the command registry to eight stock commands and remove public
   registration of `stock_bonus`.
7. Run `gofmt` and focused stock tests.

## Success Criteria

- [x] All three approved commands and examples are registered exactly.
- [x] Cash rejects fractional VND; valid unreduced ratios are accepted and echoed unchanged.
- [x] Share math floors without overflow for every accepted `int64` input.
- [x] Share-only zero entitlement makes no change; combined zero entitlement credits cash and reports zero shares.
- [x] Successful combined events use pre-event holdings and one `SavePortfolio`.
- [x] Repeating a valid command applies the adjustment again; no deduplication state exists.
- [x] `go test ./internal/modules/stock` passes.

## Risk Assessment

- Overflow or invalid numeric acceptance could silently over-credit portfolios.
  Mitigate with integer parsing, quotient/remainder math, checked cash totals,
  and boundary tests.
- Partial mutation could diverge balances. Validate first, mutate in memory,
  persist once; verify the unchanged state on every rejected path.
- Manual repeated calls can double-credit an event by design. State caller
  responsibility clearly; do not silently infer or suppress duplicates.

## Security Considerations

No new authorization surface. Continue sender checks and strict ticker/input
validation; do not echo unbounded raw input.

## Next Steps

Phase 2 migrates persisted stats before the renamed commands are deployed.
