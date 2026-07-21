---
phase: 3
title: Verify P&L behavior and compatibility
status: completed
priority: P1
effort: medium
dependencies:
  - 1
  - 2
---

# Phase 3: Verify P&L behavior and compatibility

## Overview

Expose average entry price and unrealized position P&L in both portfolio views,
document the accounting model, and run full compatibility verification.

## Requirements

- Functional: each priced holding shows average price and unrealized P&L.
- Functional: retain cash, total value, invested deposits, and account P&L.
- Functional: unavailable quotes degrade gracefully without inventing P&L;
  aggregate account P&L is suppressed when any holding is unpriced.
- Non-functional: output remains deterministic and within Telegram limits.

## Architecture

Position metrics derive from persisted remaining basis:

```text
average_price = basis / quantity
unrealized_pnl = current_value - basis
unrealized_pct = unrealized_pnl / basis × 100
```

Label position performance `Unrealized P&L` and the existing net-worth minus
top-ups metric `Account P&L` so dividends, cash, and realized proceeds are not
confused with open-position performance. If any holding lacks a quote, label
priced values as partial and do not render aggregate account P&L.

## Related Code Files

- Modify: `internal/modules/stock/handlers.go`, `stats_test.go`
- Modify: `internal/modules/coin/views.go` and view/handler tests
- Modify: `README.md`, `docs/deploy-coolify-selfhosted.md`
- Create: completion journal during finalization

## Implementation Steps

1. Render average basis and signed unrealized P&L for each priced stock/coin
   position. Add explicit stock sorting; retain coin sorting.
2. Keep existing total-value and invested calculations; relabel the final line
   as `Account P&L` and add aggregate `Unrealized P&L` for priced positions.
3. For unavailable current quotes, show stored average price when valid, mark
   current value/P&L unavailable, label priced totals partial, and suppress
   aggregate account P&L.
4. Add a conservative Telegram reply budget with deterministic truncation and
   test worst-case multi-position output.
5. Update exact output, missing-price, reply-budget, migration compatibility,
   and command behavior tests.
6. Document startup migration, weighted-average basis, realized vs unrealized
   P&L, and dividend effects.
7. Run `gofmt`, focused stock/coin/server tests, MongoDB Testcontainers tests,
   `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and
   `golangci-lint run`; complete independent test/debug/review gates.

## Success Criteria

- [ ] Each open position shows average price and unrealized amount/percentage.
- [ ] Portfolio retains account-level P&L with a clearer label.
- [ ] Missing prices do not fabricate P&L or break the reply.
- [ ] Stock output is sorted and both portfolio replies remain within Telegram
  limits under worst-case supported holdings.
- [ ] Legacy memory and MongoDB documents remain readable and migrate once.
- [ ] All focused and repository-wide verification passes.
- [ ] No command name, parameter, stats-history, or price-provider contract
  changes.

## Risk Assessment

- Users may confuse account and position metrics; explicit labels and README
  examples define both.
- Missing quotes make totals partial today; the new output must label them
  partial and suppress account P&L rather than report a false loss.
- Longer replies require budget tests for multi-asset coin portfolios.
