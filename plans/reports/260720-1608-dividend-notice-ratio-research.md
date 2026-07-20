---
type: research-report
topic: dividend-notice-ratio-examples
conducted_at: 2026-07-20T16:08:21+07:00
status: complete
---

# Research Report: Dividend Notice Ratio Examples

## Summary

Official VSDC notices support ratio-based share-dividend input. Notices express
the ratio as existing shares or rights to new shares, such as `4:1` and
`100:10`. Resulting fractional shares are rounded down and discarded.

Keep cash input as VND per existing share. Although notices also state a cash
percentage, that percentage is based on par value; accepting the actual VND per
share avoids needing par-value data.

## Methodology

- Sources: 2 recent official VSDC notices
- Notice dates: 2026-05-05 and 2026-06-26
- Terms: `trả cổ tức bằng tiền`, `trả cổ tức bằng cổ phiếu`, `tỷ lệ thực hiện`
- Scope: input semantics and rounding for the three proposed Telegram commands

## Findings

### IDC mixed dividend

- Cash: 15% per share, explicitly `1 share receives 1,500 VND`.
- Shares: `100:10`, meaning 100 existing rights receive 10 new shares.
- Example: 139 existing shares produce `139 / 100 * 10 = 13.9`, rounded down
  to 13 new shares; the 0.9 fraction is discarded.

Source: [IDC: cash and stock dividends for 2025](https://www.vsd.vn/vi/ad/197421)

### PTB mixed dividend

- Cash: 5% per share, explicitly `1 share receives 500 VND`.
- Shares: `4:1`, meaning 4 existing shares receive 1 new share.
- Example: 2,026 existing shares produce `2,026 / 4 * 1 = 506.5`, rounded down
  to 506 new shares; the 0.5 fraction is discarded.

Source: [PTB: cash and stock dividends for 2025](https://vsd.vn/vi/ad/195203)

## Comparative Analysis

| Input | Benefit | Problem |
|---|---|---|
| Absolute new-share quantity | Matches current `/stock_bonus` behavior | User must calculate entitlement manually |
| Percentage | Familiar for rates such as 10% | Ambiguous parsing and less faithful to notice wording |
| `owned:new` ratio | Mirrors VSDC notices; easy to audit | Requires integer parsing and floor rounding |

Recommendation: accept the `owned:new` ratio.

## Command Recommendation

```text
/stock_cash_dividend <vnd_per_share> <TICKER>
/stock_share_dividend <owned:new> <TICKER>
/stock_dividend <vnd_per_share> <owned:new> <TICKER>
```

Examples:

```text
/stock_cash_dividend 1500 IDC
/stock_share_dividend 100:10 IDC
/stock_dividend 1500 100:10 IDC

/stock_cash_dividend 500 PTB
/stock_share_dividend 4:1 PTB
/stock_dividend 500 4:1 PTB
```

Calculation:

```text
new_shares = floor(existing_shares * new / owned)
cash_vnd   = existing_shares * vnd_per_share
```

For a mixed dividend, calculate both values from the same pre-event holding,
then save cash and shares atomically.

## Validation and Pitfalls

- Require positive whole-number ratio parts; reject `0:1`, `4:0`, negatives,
  decimals, missing colon, and extra colons.
- Reduce equivalent ratios internally if useful, but preserve the entered ratio
  in the reply for auditability.
- Use integer arithmetic and floor division for shares; avoid floating-point
  rounding.
- Reject events producing zero new shares, or explicitly confirm zero payout;
  recommended behavior: reject with the minimum holding required.
- Calculate cash before adding new shares so newly distributed shares do not
  receive cash from the same event.
- Keep cash input in VND/share. Do not derive it from a percentage unless the
  portfolio also stores the security's par value.

## Next Steps

1. Confirm zero-share-result behavior.
2. Finalize the three command contracts and reply wording.
3. Plan command renames, combined atomic update, stats migrations, and tests.

## Unresolved Questions

- When a valid ratio yields zero new shares, should the command reject the
  event or record a zero-share result while still applying cash in the combined
  command?
