---
phase: 2
title: Update command surfaces
status: completed
priority: P2
dependencies:
  - 1
---

# Phase 2: Update command surfaces

## Overview

Update all user-facing command surfaces required by `AGENTS.md` for a new Telegram command. This is an additive command, so no stats migration is needed.

## Requirements

- Functional: command appears in `/help` because it is public and registered in the misc module.
- Functional: command appears in `telegram-commands.json` for Telegram command menu setup.
- Functional: README misc module summary reflects the new command.
- Non-functional: no command rename/delete, so preserve stats behavior by doing nothing special.

## Architecture

The source of runtime behavior is `internal/modules/misc/misc.go`. `telegram-commands.json` is a manual Telegram command menu source, and `README.md` is the human-facing module overview.

Stats compatibility note:
- Adding `/wheelofnames` creates new command usage rows naturally when the stats hook records invocations.
- No existing stats rows need migration because no command is renamed or deleted.

## Related Code Files

- Modify: `/config/workspace/tiennm99/miti99bot/telegram-commands.json`
- Modify: `/config/workspace/tiennm99/miti99bot/README.md`
- Create: none
- Delete: none

## Implementation Steps

1. Add command entry to `telegram-commands.json` near other misc commands:
   - `command`: `wheelofnames`
   - `description`: concise public description, e.g. `Pick one random comma-separated option`
2. Update README Modules table for `misc` to include `/wheelofnames`.
3. Do not update stats migrations or system state; this is add-only behavior.

## Success Criteria

- [x] `telegram-commands.json` remains valid JSON.
- [x] README `misc` command list includes `/wheelofnames`.
- [x] No migration or legacy stats marker added.

## Risk Assessment

Risk: runtime registration and manual command menu source drift. Mitigate by updating both `misc.go` and `telegram-commands.json` in same implementation.
