---
phase: 2
title: "Add Telegram Command"
status: completed
priority: P2
dependencies: [1]
---

# Phase 2: Add Telegram Command

## Overview
Expose `/stock_events <ticker> [days]` as a public, read-only Telegram command. Keep it independent from sender-specific portfolio state and produce deterministic, Telegram-safe SSI event output.

## Requirements
- Functional: register public command metadata in `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock.go:13` with exact parameters `<ticker> [days]` and a concise description compatible with `/help` and Telegram’s native menu (`C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\command_presentation.go:6`, `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\validate.go:14`).
- Functional: accept exactly 1 required arg plus 1 optional days arg; default days to 30; allow only whole numbers `1..90`; reject missing or extra args with exact usage text.
- Functional: normalize ticker with `normalizeStockSymbol` at `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\symbols.go:18`; surface the same unknown-ticker wording used by current stock handlers at `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers.go:93`.
- Functional: do not require `senderInfo`, `LoadPortfolio`, `PendingDividendStore`, or any write path from `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers.go:59` and `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\portfolio.go:64`.
- Functional: respond cleanly for no events, upstream/provider failure, and parse failure.
- Non-functional: preserve provider order, bound every message below the repo’s 4000-char Telegram safety margin used at `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers.go:480` and `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stats\views.go:17`.

## Architecture
Data flow:
1. Dispatcher routes the public command to `handleStockEvents`.
2. The handler parses args, defaults/validates `days`, normalizes the ticker, and computes `after = now - days*24h`, `through = now`.
3. The handler calls the generic SSI provider from Phase 1 and receives chronologically sorted corporate actions.
4. Each event becomes a plain-text block with symbol, SSI type, bounded title, populated dates, and `SSI event: <id>` plus the raw source URL when present.
5. Blocks are packed into `<=4000`-char replies without splitting an event block; multi-message output adds deterministic part numbering. If one block still exceeds budget after field truncation, hard-truncate the final line with `…(truncated)`.

Formatting strategy:
- Reuse the package-level rune-safe truncation helper from `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\dividend_notifications.go:363` for long titles and labels.
- Prefer plain-text replies via `chathelper.Reply` instead of HTML tables so raw SSI URLs stay visible and chunking stays simple.
- Keep chronological ordering exactly as returned by the provider; do not re-sort by type or date label in the handler.

## Related Code Files
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers.go` — add state access to the generic provider if needed, but keep sender/store logic untouched for this command.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock.go` — register `/stock_events` metadata and handler.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\stock_events.go` — implement args parsing, provider call, event formatting, and reply chunking.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stock\handlers_test.go` — add registration, usage, validation, no-sender, no-events, upstream-error, and chunking coverage.

## Implementation Steps
1. Wire the state to a generic stock-event provider with a zero-write fallback path suitable for tests that do not initialize stores or sender data.
2. Register `stock_events` as `VisibilityPublic` with exact metadata: `Parameters: "<ticker> [days]"`, description concise enough to stay well under the 256-rune menu limit.
3. Implement the command parser so `/stock_events TCB`, `/stock_events TCB 7`, and only those shapes succeed; anything else returns exact usage or range errors.
4. Implement the read-only handler using `s.now()` and the provider from Phase 1. Do not touch portfolio/dividend history, pending buttons, or locks.
5. Format one event block per SSI row with only required fields. Include date labels only when present so missing `RecordDate` or `PaymentDate` does not generate noisy placeholders.
6. Pack reply blocks into bounded Telegram messages, preserve order across chunks, and keep no-events/failure replies single-message and deterministic.
7. Add handler tests for default 30-day behavior, explicit day range, invalid day values, extra args, senderless execution, provider window propagation, empty results, provider error mapping, and over-budget output chunking.

## Success Criteria
- [x] `/stock_events <ticker> [days]` is publicly registered with exact parameters/usage text and no sender requirement.
- [x] The handler produces deterministic chronological output, friendly empty/error replies, and message chunks below the Telegram safety budget.
- [x] The command path is provably read-only: no portfolio loads, pending-action writes, or dividend-history mutations are required for success.

## Risk Assessment
- High: oversized replies can be rejected by Telegram or split mid-event. Mitigation: chunk by whole event blocks against a 4000-char budget and add an oversized synthetic test.
- Medium: introducing a hidden sender or store dependency would break channel/anonymized use. Mitigation: add a senderless test case and keep the handler isolated from `senderInfo`.
- Medium: metadata drift between handler usage and registration breaks `/help` and the native menu. Mitigation: assert the exact `Parameters` string and usage text in tests.
- Rollback: remove the new command registration and `stock_events` handler changes. Phase 1’s additive provider can remain unused or be reverted separately without data cleanup.
