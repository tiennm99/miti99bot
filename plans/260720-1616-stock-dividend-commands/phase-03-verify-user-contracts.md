---
phase: 3
title: Verify User Contracts
status: completed
effort: ''
priority: P1
dependencies:
  - 1
  - 2
---

# Phase 3: Verify User Contracts

<!-- Updated: Validation Session 1 - document manual caller responsibility -->

## Context Links

- [Approved command contracts](../reports/260720-1612-stock-dividend-command-brainstorm.md#approved-behavior)
- [Repository development rules](../../AGENTS.md#command-changes)

## Overview

Synchronize Telegram command-menu metadata and README documentation with the
implemented contracts, then run focused and repository-wide verification.

## Requirements

- Functional: command names, descriptions, argument order, examples, errors,
  and menu behavior agree across registry, handlers, JSON, tests, and README.
- Functional: remove `stock_bonus` from active user surfaces; document that
  cash is VND/share and share input is the `owned:new` notice ratio.
- Functional: document positive whole-VND input, acceptance of unreduced ratios,
  preserved ratio text, and deliberate repeat-call behavior.
- Non-functional: preserve plan-approved migration behavior and run every gate
  required for command, storage, migration, and shared startup changes.

## Architecture

Treat registered Go commands as runtime truth and `telegram-commands.json` as
the manual BotFather/menu source. Tests assert the final eight-command stock
registry and registered menu contents. README gives concise user-visible syntax
and the floor/pre-event behavior needed to use notices correctly. It states
that commands are manual adjustments: the caller verifies the notice and avoids
accidental duplicates; the bot enforces syntax and storage safety only.

## Related Code Files

- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\telegram-commands.json` — replace old dividend/bonus entries with three approved commands.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\README.md` — document syntax, VND/share, ratio direction, floor rounding, and combined behavior.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers_test.go` — final registry and user-facing text assertions.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\cmd\server\command_menu_test.go` — command-menu presence and removal checks.
- Verify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stats\startup_test.go` — migration contracts remain green.

## Implementation Steps

1. Replace `stock_bonus` and old cash-only `stock_dividend` menu descriptions
   with `stock_cash_dividend`, `stock_share_dividend`, and combined
   `stock_dividend`; keep Telegram ordering coherent.
2. Add a compact README stock-command section with all three syntaxes and one
   mixed example. State positive whole VND/share, `owned:new`, accepted
   unreduced ratios, preserved ratio replies, integer floor rounding,
   pre-event basis, and caller responsibility for repeat events.
3. Update registry/menu tests to assert new commands and reject old public
   `stock_bonus`; ensure descriptions stay within Telegram limits.
4. Run `gofmt` over every changed Go file.
5. Run focused packages: `go test ./internal/modules/stock`,
   `go test ./internal/modules/stats`, and `go test ./cmd/server`.
6. Run `go test ./...` and `go vet ./...`.
7. If `golangci-lint` is available, run `golangci-lint run`; record a skipped
   gate explicitly when the binary is absent.

## Success Criteria

- [x] Registry, handlers, JSON, README, and menu tests expose the same three contracts.
- [x] No active user-facing surface advertises `stock_bonus` or old cash-only `/stock_dividend` syntax.
- [x] README distinguishes manual correctness responsibility from mandatory parser/overflow safety.
- [x] Focused tests, `go test ./...`, and `go vet ./...` pass.
- [x] `golangci-lint run` passes when available.
- [x] No unresolved questions remain.

## Risk Assessment

- Stale menu/docs can cause financially wrong manual entries. Search all active
  surfaces for old names and syntax, while leaving migration fixtures intact.
- Mongo integration tests may require local infrastructure. Keep unit coverage
  mandatory and report environment-based integration skips accurately.

## Security Considerations

Documentation must use synthetic holdings and tickers only; no tokens,
production data, or private portfolio records.

## Next Steps

Implementation can proceed phase-by-phase after plan approval.
