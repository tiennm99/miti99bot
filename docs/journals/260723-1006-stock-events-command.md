# Stock Events Command

**Date**: 2026-07-23 10:06
**Severity**: Medium
**Component**: `stock` module, command metadata, docs
**Status**: Resolved

## What Happened

We shipped a new read-only `/stock_events <ticker> [days]` command for SSI iBoard corporate actions. It defaults to 30 days, accepts 1..90, and returns chronological, Telegram-safe chunks. The important design choice was to keep this path additive and raw: the command reads SSI event data directly instead of forcing it through the strict dividend-normalization path used by `/stock_portfolio`.

That decision mattered because the review found a bad assumption in the original normalization approach: optional SSI fields like ex-right, record, or payment date can be malformed without the event itself being unusable. We stopped trying to force every event into a dividend-shaped class and exposed the raw fields instead. That is the right tradeoff for an undocumented provider.

## The Brutal Truth

This was more fragile than it needed to be. We had to correct a command-menu metadata regression after the feature landed, and the review exposed that strict parsing was silently dropping valid SSI events. That is exactly the kind of bug that wastes time because it looks “clean” until you realize you are throwing away real data.

## Technical Details

- Feature landed in `feat(stock): add stock events lookup`.
- Command registration had to be kept in sync with Telegram help/menu metadata for `<ticker> [days]`.
- Review showed the generic path should not parse raw SSI events into a rigid class.
- Verification gates passed after fixes: focused stock tests, `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, `golangci-lint run`, and `git diff --check`.

## What We Tried

- Reused the SSI corporate-action fetcher.
- Initially normalized events like dividends.
- Fixed the menu metadata mismatch in the shared command contract.
- Dropped the rigid event-class mapping and kept the raw provider payload.

## Root Cause Analysis

We overfit an undocumented API to our internal dividend model. That made the feature look structured but hid malformed optional fields and risked losing valid events. The real mistake was prioritizing type shape over provider fidelity.

## Lessons Learned

When the source API is undocumented, raw field preservation beats premature normalization. Keep user-facing contracts exact, but do not invent structure that the provider does not guarantee.

## Next Steps

Monitor for SSI schema drift, keep `/stock_events` best-effort, and expand tests only around the raw fields and command contract. The owner is the stock module maintainer; no follow-up migration work is needed.
