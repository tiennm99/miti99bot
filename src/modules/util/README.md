# Util Module

Core bot utilities — informational commands that read framework metadata.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/info` | public | Echoes chat id, thread id, sender id (debug helper) |
| `/help` | public | Renders all public + protected commands grouped by module |
| `/stickerid` | private | Reply to a sticker to get its bot-scoped `file_id` (used to collect sticker pools for other modules) |

## Architecture

- `/help` is a pure renderer over `getCurrentRegistry()` — it reads the registry's command maps and formats them as Telegram HTML. Modules with zero visible commands are omitted. Private commands are always excluded.
- `/info` reads grammY context fields (`ctx.chat.id`, `ctx.message.message_thread_id`, `ctx.from.id`). No external state.
- `/stickerid` reads `ctx.message.reply_to_message.sticker` and echoes `file_id` + `file_unique_id`. Private visibility keeps it out of `/help` and the Telegram menu. Used to collect sticker IDs for hard-coded pools in other modules (e.g., loldle win/lose/giveup stickers).
- HTML injection is prevented via `escapeHtml()` on all user-influenced strings.

## Database

**No KV usage.** This module has no `init()` hook and does not store any data.
