---
phase: 3
title: "Verify and Document"
status: completed
priority: P2
dependencies: [1, 2]
---

# Phase 3: Verify and Document

## Overview
Lock the public contract, prove dividend behavior did not regress, and update user-facing docs. This phase is the quality gate before implementation is considered done.

## Requirements
- Functional: document `/stock_events <ticker> [days]`, default 30 days, valid range `1..90`, and read-only SSI behavior in `C:\Users\miti99\Workspaces\tiennm99\miti99bot\README.md:22`.
- Functional: keep existing dividend event behavior unchanged for `/stock_portfolio`, including SSI overlap/refetch behavior already covered in `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_flow_test.go:94`, `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_flow_test.go:154`, and `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_flow_test.go:491`.
- Non-functional: run the repo gates required by `AGENTS.md`: `gofmt`, focused stock tests, `go test ./...`, `go vet ./...`, `go build ./...`, and `golangci-lint run` when the binary is available.

## Architecture
This phase does not introduce runtime behavior. It validates that:
1. command metadata matches handler usage and README wording;
2. the new generic SSI path did not alter dividend discovery or notification flows;
3. repo-wide compilation, tests, and vet/lint still pass after the additive command lands.

Test matrix:
- Unit: generic SSI normalization/order tests, days parser, reply chunking.
- Integration: registration metadata, handler no-events/error/senderless behavior, provider window propagation.
- Regression: dividend SSI/provider tests plus `dividend_flow_test.go` scenarios.
- Repository: full `go test ./...`, `go vet ./...`, `go build ./...`, optional `golangci-lint run`.

## Related Code Files
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\README.md` — add the new command contract beside the stock command documentation.
- Verify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers_test.go` — command registration/usage/read-only coverage.
- Verify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi_test.go` — generic SSI fetch plus dividend-regression coverage.
- Verify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_flow_test.go` — unchanged `/stock_portfolio` dividend notifications.

## Implementation Steps
1. Update the README stock section with the exact command syntax, default day window, allowed range, and SSI-source caveat so docs match the public command contract.
2. Run focused stock-module tests first to catch fast feedback on provider normalization, handler parsing, chunking, and dividend regressions.
3. Run `gofmt` on touched Go files, then execute `go test ./...`, `go vet ./...`, and `go build ./...`.
4. Run `golangci-lint run` if the binary is available locally; if it is unavailable, record that explicitly rather than silently skipping it.
5. If any gate fails because of the new command/provider work, fix the owning phase before marking this plan complete.

## Success Criteria
- [x] README, command metadata, and handler usage text all match `/stock_events <ticker> [days]` exactly.
- [x] Focused stock tests cover the new command and confirm `/stock_portfolio` dividend behavior is unchanged.
- [x] Repo-wide test, vet, and build gates pass; lint status is either passing or explicitly reported as unavailable/pre-existing.

## Risk Assessment
- Medium: docs can drift from the exact `Parameters` string and create `/help` confusion. Mitigation: compare README text against the registered `Parameters` and handler usage string before closing.
- Medium: additive SSI changes can still regress dividend behavior indirectly. Mitigation: rerun the provider/dividend-flow suites, not just the new command tests.
- Low: repo-wide gates may surface unrelated pre-existing failures. Mitigation: separate new failures from baseline issues and avoid hiding them behind the feature summary.
- Rollback: revert the README and command changes if validation reveals an unacceptable regression. No data migration or cleanup step is needed.
