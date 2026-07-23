---
title: Stock Events Command
description: >-
  Add a public /stock_events command for read-only SSI corporate action lookup
  with bounded Telegram output.
status: completed
priority: P2
effort: 8h
branch: main
tags:
  - feature
  - backend
  - telegram
  - stock
blockedBy: []
blocks: []
created: 2026-07-23T00:00:00.000Z
---

# Stock Events Command

## Overview

Add public `/stock_events <ticker> [days]` for read-only SSI corporate-action lookup. Reuse SSI pagination helpers from `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events_ssi.go:113` without changing the dividend-history path used by `/stock_portfolio` (`C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_notifications.go:56`).

## Scope Challenge

- Existing code: command metadata/help comes from `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock.go:13`, `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\command_presentation.go:6`, and `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\validate.go:14`; ticker normalization already exists at `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\symbols.go:18`.
- Minimum change: add one parallel read-only SSI event path plus one public command; no portfolio writes, no dividend buttons, no stats/storage migration.
- Complexity: 7 touched files, 1 new command, 0 schema changes, 3 sequential phases.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Extend SSI Corporate Events](./phase-01-extend-ssi-corporate-events.md) | Completed |
| 2 | [Add Telegram Command](./phase-02-add-telegram-command.md) | Completed |
| 3 | [Verify and Document](./phase-03-verify-and-document.md) | Completed |

## Dependencies

- Cross-plan: None. `plans\260722-1114-compact-portfolio-numbers\plan.md`, `plans\260722-1356-per-user-dividend-history\plan.md`, `plans\260722-1705-mobile-portfolio-columns\plan.md`, and `plans\260722-1843-retire-stock-dividend-command\plan.md` are already completed.
- Phase 2 depends on Phase 1's additive SSI corporate-action fetch contract.
- Phase 3 depends on Phases 1-2 for command behavior, regression tests, and docs alignment.

## Compatibility

- Keep dividend-only contracts unchanged: `DividendEventProvider` stays specific to `/stock_portfolio` at `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_events.go:12`, retained history stays under `Portfolio.Dividends` at `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\portfolio.go:44`.
- No Mongo/index/system-state/stats work. Out-of-scope items remain untouched.

## Validation

- Unit: SSI generic normalization, overlap/date filtering, deterministic order, days parsing, and reply chunking/truncation.
- Integration: command registration metadata, usage/error replies, senderless read-only execution, no-events path, upstream failure path, and dividend regressions.
- Repo gate: `gofmt`, focused stock tests, `go test ./...`, `go vet ./...`, `go build ./...`, and `golangci-lint run` when available.

## Open Questions

None.
