# Util Module

Core bot utilities — informational commands that read framework metadata.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/info` | public | Echoes chat id, thread id, sender id (debug helper) |
| `/help` | public | Renders all public + protected commands grouped by module |

## Architecture

- `/help` is a pure renderer over `getCurrentRegistry()` — it reads the registry's command maps and formats them as Telegram HTML. Modules with zero visible commands are omitted. Private commands are always excluded.
- `/info` reads grammY context fields (`ctx.chat.id`, `ctx.message.message_thread_id`, `ctx.from.id`). No external state.
- HTML injection is prevented via `escapeHtml()` on all user-influenced strings.

## Database

**No KV usage.** This module has no `init()` hook and does not store any data.
