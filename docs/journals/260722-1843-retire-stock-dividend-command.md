---
title: Retire Redundant Stock Dividend Command
date: 2026-07-22 18:43
component: stock module
status: completed
---

# Retire Redundant Stock Dividend Command

## Context

The stock module had one command too many. `/stock_dividend` overlapped with the specialized cash and share dividend commands, and the combined path was doing more harm than good. It blurred intent, made command handling harder to reason about, and created extra surface area for bugs that did not buy us any actual user value.

## What Happened

We removed the redundant `/stock_dividend` command and kept the specialized `/stock_cash_dividend` and `/stock_share_dividend` flows. That decision matches the product behavior: cash and share dividends are distinct operations, and forcing them through one combined command was an abstraction tax with no payoff.

Historical stats were not deleted. They were marked `deleted:true` so the record stays intact for analytics and compatibility while the visible views stop treating the command as active. That is the right compromise for a retired command: preserve history, suppress the live surface.

We also fixed two implementation traps that showed up while wiring the removal. First, the rolling deploy finding made it clear that a one-time migration was not enough; read/write suppression and every-boot reconciliation are now permanent so mixed-version deployments do not resurrect a dead command. Second, the Mongo path had an N+1 shape, so we replaced the row-by-row updates with `UpdateMany` plus `CountDocuments` instead of pretending the loop was acceptable.

## Brutal Truth

This was one of those changes where the codebase was telling us the truth before we were willing to hear it. Keeping a redundant command around because it “might be convenient” would have meant carrying dead complexity forever. The frustrating part is that the bug shape was predictable: once the command surface drifted from the underlying model, every extra path became another place to desync.

## Technical Details

- `/stock_dividend` removed from the live command surface
- `/stock_cash_dividend` and `/stock_share_dividend` retained as the explicit paths
- legacy stats rows preserved with `deleted:true`
- exact-prefix matching kept for safety so we do not suppress unrelated command names
- Mongo writes changed from per-document updates to `UpdateMany` plus `CountDocuments`

## What We Tried

- Considered leaving `/stock_dividend` as a compatibility alias, but that kept the ambiguity alive.
- Considered deleting stats history, but that would have broken reporting and destroyed useful audit data.
- Considered a row-by-row Mongo rewrite, but the N+1 behavior was the same old inefficiency in a different coat.

## Root Cause Analysis

The root cause was command overloading. We tried to make one command cover two different user intents, and the result was a brittle contract that was harder to maintain than the feature justified. The deploy/reconciliation issue was the same class of mistake: assuming a one-time action would stay correct across rolling versions. It will not.

## Lessons Learned

- Prefer explicit commands when the operations are semantically different.
- Retain legacy stats, but mark retired commands as deleted instead of pretending they never existed.
- Exact-prefix checks matter when suppressing command behavior; sloppy matching is how unrelated names get caught in the blast radius.
- If a deploy has mixed versions, assume the old behavior will reappear unless read/write suppression and reconciliation are both permanent.

## Verification

- focused stock, stats, and server tests passed
- focused race-enabled tests passed
- full `go test -count=1 ./...` passed
- real MongoDB 8 integration ran through Testcontainers and passed
- `go build ./...` and `go vet ./...` passed
- `golangci-lint run` reported zero issues
- total statement coverage measured `77.3%`; the repository has no configured coverage threshold

## Next Steps

The code is done and verified. The only remaining step is a commit, pending user approval. After that, the retired-command behavior should be watched in real deployments for any command-menu or stats edge cases.
