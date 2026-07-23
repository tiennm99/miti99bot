# Stock Info Command

**Date**: 2026-07-23 12:25
**Severity**: Medium
**Component**: `stock` module, SSI quote client, command docs/tests
**Status**: Resolved

## What Happened

We added a new read-only `/stock_info <ticker>` command to show a compact SSI iBoard quote snapshot with company, exchange, current price, gain/loss since open, change versus reference price, open/high/low, and volume. `/stock_price` stayed unchanged on purpose.

The implementation had to be forced into a safer shape after review. The first pass reused the same SSI quote DTO for legacy price paths, which was a bad idea for an undocumented upstream schema. We split the detail response into its own DTO so the old decode path would not get dragged into the new fields, and we kept `/stock_price` on the existing fallback chain.

## The Brutal Truth

This was the kind of change that looks simple until it starts biting in review. The upstream API is undocumented, so every assumption becomes a liability. The frustrating part is that we had to spend time proving that the “easy” version would have been brittle: redirects, malformed fields, and bogus math all showed up as real failure modes. That is annoying, but it is also exactly why the extra guardrails were necessary.

## Technical Details

- `/stock_info` uses exactly one SSI GET and no KBS/VCI fallback.
- Since-open change is computed from `matchedPrice - openPrice`; reference change remains separate.
- Review blockers fixed:
  - separate SSI detail DTO to protect legacy `/stock_price` decoding
  - redirect refusal so the command cannot silently fan out into extra requests
  - finite-safe change math to avoid `Inf%`
  - explicit zero volume preserved as `0` while nil/invalid volume stays `N/A`
  - company/exchange text bounded so Telegram replies stay safe
- Best-effort behavior is still required because SSI is undocumented.
- Verification passed: focused stock tests, race tests, `go vet ./...`, `go build ./...`, `golangci-lint run`, and repository test gates.

## What We Tried

- Started with a shared quote structure and a direct SSI detail fetch.
- Tightened the handler and formatting tests around the compact output.
- Reworked the client after adversarial review exposed the legacy decode and redirect risks.

## Root Cause Analysis

The root mistake was treating an undocumented provider like a stable contract. That made the first design too optimistic: one shared DTO, unchecked redirect behavior, and derived math that assumed normal values. The code worked until review forced the ugly cases into the open.

## Lessons Learned

When the upstream schema is not under our control, isolate the new path instead of extending old contracts in place. Keep the user-facing command additive, keep the legacy command stable, and assume the provider will return missing, malformed, or absurd values.

## Next Steps

Keep `/stock_info` best-effort and monitor for SSI schema drift. If the upstream shape changes again, update only the detail path and leave `/stock_price` untouched. No migration work is needed; the owner is the stock module maintainer.
