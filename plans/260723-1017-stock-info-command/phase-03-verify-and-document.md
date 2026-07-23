---
phase: 3
title: "Verify and Document"
status: completed
priority: P2
dependencies: [1, 2]
---

# Phase 3: Verify and Document

## Overview

Align public documentation and prove the new one-call command does not change
existing quote, portfolio, event, or dividend behavior.

## Requirements

- Functional: document syntax, compact fields, one-call SSI-only behavior, and
  best-effort upstream limitation in README.
- Non-functional: satisfy focused, repository, race, vet, build, lint, and diff gates.

## Architecture

No new runtime behavior. Verify command metadata/usage/docs as one contract and
exercise existing `/stock_price` plus portfolio quote callers as regressions.

## Related Code Files

- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\README.md`
- Verify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\prices_test.go`
- Verify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock_info_test.go`
- Verify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\cmd\server\command_menu_test.go`

## Implementation Steps

1. Update README with `/stock_info <ticker>` and its SSI-only behavior.
2. Run `gofmt` and focused stock/server command-discovery tests.
3. Run `go test -count=1 ./...` and stock race tests.
4. Run `go vet ./...`, `go build ./...`, `golangci-lint run`, and `git diff --check`.
5. Review all price callers and public contracts for unintended changes.

## Success Criteria

- [x] README, registration metadata, handler usage, and tests match exactly.
- [x] All focused and repository gates pass with no `/stock_price` regression.
- [x] Review confirms one SSI call and no storage/schema/provider-fallback changes.

## Risk Assessment

The SSI endpoint is undocumented. Document best-effort behavior and keep the
new surface additive so it can be removed without migration or data cleanup.
