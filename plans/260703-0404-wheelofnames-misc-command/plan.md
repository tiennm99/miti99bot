---
title: Wheel of Names Misc Command
description: >-
  Add public /wheelofnames command to pick one random comma-separated option in
  the misc module.
status: completed
priority: P2
branch: main
tags:
  - feature
  - backend
blockedBy: []
blocks: []
created: '2026-07-03T04:04:35.848Z'
createdBy: 'ck:plan'
source: skill
---

# Wheel of Names Misc Command

## Overview

Add `/wheelofnames` to the existing `misc` module. The command accepts comma-separated options after the command text, trims whitespace, ignores empty entries, then replies with one randomly selected option. No storage, migrations, auth changes, or new module needed.

Assumptions:
- Public command, visible in `/help` and Telegram command menu.
- Usage reply when no valid comma-separated entries remain.
- One valid entry is allowed and returns that entry.
- Duplicate entries stay duplicated, so users can weight an option intentionally.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Implement command](./phase-01-implement-command.md) | Completed |
| 2 | [Update command surfaces](./phase-02-update-command-surfaces.md) | Completed |
| 3 | [Validate behavior](./phase-03-validate-behavior.md) | Completed |

## Cross-Plan Dependencies

None. No unfinished project plans found under `plans/`.

## Key Files

| File | Action |
|------|--------|
| `internal/modules/misc/misc.go` | Add command registration, parser/helper, handler |
| `internal/modules/misc/misc_test.go` | Update registration expectations |
| `internal/modules/misc/handlers_test.go` | Add handler behavior tests |
| `telegram-commands.json` | Add public Telegram command menu entry |
| `README.md` | Update misc module command list |

## Acceptance Criteria

- `/wheelofnames a, b, c` replies with exactly one trimmed item from `a`, `b`, `c`.
- `/wheelofnames` and `/wheelofnames , ,` reply with usage text.
- Empty comma segments do not appear as choices.
- Command is public and included in module registration, `/help`, and `telegram-commands.json`.
- Focused tests pass, then full `go test ./...` and `go vet ./...` pass.

## Unresolved Questions

None.
