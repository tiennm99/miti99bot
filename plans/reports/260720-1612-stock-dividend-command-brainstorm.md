---
type: brainstorm-report
topic: stock-dividend-command-naming
created_at: 2026-07-20T16:12:33+07:00
status: approved
modes:
  html: false
  wiki: false
---

# Brainstorm Report: Stock Dividend Commands

## Summary

Approved three explicit commands:

```text
/stock_cash_dividend <vnd_per_share> <TICKER>
/stock_share_dividend <owned:new> <TICKER>
/stock_dividend <vnd_per_share> <owned:new> <TICKER>
```

Cash uses VND per existing share. Share distributions use the official-notice
ratio form, such as `4:1` or `100:10`. Mixed dividends calculate both outcomes
from the pre-event holding and persist them atomically.

## Problem-First Analysis

### Solution-Jumping Diagnosis

The original names encoded outcomes inconsistently: `/stock_bonus` added shares
while `/stock_dividend` added cash. Users could not distinguish a true bonus
share event from a stock dividend or represent a mixed dividend notice.

### Underlying Problem

Users need command names and inputs that map directly to Vietnamese dividend
notices, while preserving simple manual paper-portfolio accounting.

### Assumptions and Validation

| Assumption | Risk if wrong | Validation |
|---|---|---|
| Notices provide share ratios | Users must calculate quantities | Confirmed by VSDC `4:1` and `100:10` notices |
| Cash per share is sufficient | Percentage input may be expected | Notices explicitly state VND received per share |
| Fractions round down | Portfolio could over-credit shares | Confirmed by both VSDC examples |
| Mixed event uses one record-date holding | Cash may include newly issued shares | Calculate both before mutation |

### Problem Statement

Paper-trading users cannot accurately record cash-only, share-only, and mixed
dividends because current command semantics are incomplete and `stock_bonus`
does not necessarily mean a stock dividend. Success means each notice maps to
one obvious command and produces auditable integer-share and cash results.

### Alternative Framings

1. Keep current commands: smallest change, but mixed events need two commands
   and terminology remains ambiguous.
2. Use explicit cash/share commands plus a combined command: clear, direct,
   matches actual notices. Selected.
3. Add a generic corporate-action command with subtypes: extensible but too
   complex for three small manual operations.

### Evidence Status

Strong for syntax and rounding: two recent official VSDC mixed-dividend notices
use cash-per-share explanations, `owned:new` ratios, and floor rounding.

### Validation Plan

- Unit-test ratio parsing, overflow boundaries, and floor division.
- Table-test cash-only, share-only, and combined handlers.
- Verify combined calculation uses the same pre-event holding.
- Test one-time stats migrations for anonymous and per-user rows.
- Reject the design if real notices require unsupported fractional settlement
  rather than discarded fractions.

### Stakeholder Message

Use explicit cash and share commands for single-form notices, and the combined
command for mixed notices. Inputs mirror VSDC wording, reducing manual math and
making bot replies easy to compare against source notices.

## Evaluated Approaches

| Approach | Pros | Cons | Decision |
|---|---|---|---|
| Absolute new-share quantity | Reuses current handler behavior | Manual calculation; hides rounding | Reject |
| Percentage share input | Familiar shorthand | Ambiguous; less faithful to notices | Reject |
| `owned:new` share ratio | Mirrors notices; deterministic | Needs parser and integer safety | Approve |

## Approved Behavior

### Cash-only

```text
/stock_cash_dividend 1500 IDC
cash = existing_shares * 1500 VND
```

### Share-only

```text
/stock_share_dividend 100:10 IDC
new_shares = floor(existing_shares * 10 / 100)
```

Reject a share-only event when the valid ratio produces zero new shares and
report the minimum holding needed.

### Mixed

```text
/stock_dividend 1500 100:10 IDC
```

Calculate cash and new shares from the same pre-event holding. When the share
result is zero, still credit cash and report zero new shares. Save once so cash
and shares cannot diverge after a partial failure.

## Compatibility and Touchpoints

- Rename `/stock_bonus` to `/stock_share_dividend`.
- Move historical `/stock_bonus` stats to `/stock_share_dividend`.
- Move historical cash-only `/stock_dividend` stats to
  `/stock_cash_dividend` before reusing `/stock_dividend` for mixed events.
- Update `internal/modules/stock/stock.go`, handlers, handler tests, and
  `telegram-commands.json`.
- Add idempotent startup migration through the shared `system` collection;
  cover anonymous rows, per-user rows, target-row merging, and repeated startup.

## Risks

- Integer multiplication can overflow before division; validate bounds or use a
  safe quotient/remainder calculation.
- Reusing `/stock_dividend` without migrating old stats mislabels historical
  cash-only usage as combined usage.
- Applying shares before cash calculation overpays the same event.
- Accepting decimal or zero ratio components creates undefined rounding.

## Success Criteria

- `4:1` and `100:10` parse; malformed or non-positive ratios fail clearly.
- Share calculations use integer floor semantics.
- Cash/share/mixed commands match their documented examples.
- Mixed updates are atomic and use pre-event holdings.
- Existing command statistics remain preserved under the correct new meanings.
- Focused tests, `go test ./...`, and `go vet ./...` pass.

## Sources

- [VSDC IDC mixed dividend notice](https://www.vsd.vn/vi/ad/197421)
- [VSDC PTB mixed dividend notice](https://vsd.vn/vi/ad/195203)
- [Supporting research report](./260720-1608-dividend-notice-ratio-research.md)

## Next Steps

Create a tests-first implementation plan because this change renames public
commands, changes persisted stats attribution, and modifies financial
calculation behavior.

## Unresolved Questions

None.
