---
phase: 2
title: Implement weighted-average accounting
status: completed
priority: P1
effort: medium
dependencies:
  - 1
---

# Phase 2: Implement weighted-average accounting

## Overview

Update stock and coin trade mutations to maintain remaining cost basis and show
realized P&L on every successful sale.

## Requirements

- Functional: buys add actual transaction spend to the symbol's total basis.
- Functional: sells remove proportional basis and report realized P&L.
- Functional: full exits remove both holding and basis keys.
- Functional: stock share dividends change quantity but not total basis.
- Non-functional: rejected trades and failed saves leave portfolio state intact.
- Non-functional: positive holdings with absent, nonpositive, or non-finite
  basis fail closed instead of producing fabricated accounting.

## Architecture

For a sale, capture pre-sale holding and basis, then calculate:

```text
sold_basis = total_basis × sold_quantity / held_quantity
realized_pnl = proceeds - sold_basis
realized_pct = realized_pnl / sold_basis × 100
```

The remaining average price is unchanged after a partial sale. A share dividend
adds shares without cost, so `basis / new_quantity` automatically lowers the
average price. No lot ledger or cumulative realized-P&L field is introduced.

## Related Code Files

- Modify: stock/coin `portfolio.go`, trade handlers, and format helpers
- Modify: stock dividend handler tests to assert unchanged total basis
- Modify: stock/coin portfolio and handler tests

## Implementation Steps

1. Add small portfolio methods to add purchase basis and remove proportional
   basis with finite/overflow guards appropriate to each module's precision.
2. Update buys so quantity, cash deduction, and basis addition persist in the
   same mutation.
3. Update sells so quantity, cash credit, and basis reduction persist together;
   append `Realized P&L` amount and percentage to the reply.
4. Determine coin full exit from normalized post-sale holdings. If quantity
   disappears at the dust threshold, assign all remaining basis to that sale
   and delete the basis key. Keep monetary validation separate from quantity
   dust normalization.
5. Assert cash/share/combined dividend behavior: cash never changes basis and
   share additions preserve total basis.
6. Cover weighted repeated buys, gain/loss/breakeven partial sells, full sells,
   insufficient holdings, overflow/invalid data, conflicts, and save failure.

## Success Criteria

- [ ] `average price == remaining basis / remaining quantity` after every buy,
  sell, and share dividend.
- [ ] Stock and coin sell replies report correct realized P&L.
- [ ] Full exits leave no holding or basis entry.
- [ ] Failed/rejected operations do not mutate persisted state.
- [ ] Missing or corrupt runtime basis blocks affected accounting operations
  with a clear retry/operator message.

## Risk Assessment

- Stock uses float64 currency already; calculations must reject non-finite or
  unsafe results. Coin retains its existing dust normalization.
- Mutation order must not delete quantity before sold basis is calculated.
- Existing account `Meta.Invested` continues to mean deposits and must not be
  repurposed as position basis.
