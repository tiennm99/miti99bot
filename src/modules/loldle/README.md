# Loldle Module

League of Legends guessing game — currently a stub proving the plugin system.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/loldle` | public | Start a loldle game (stub) |
| `/lstats` | protected | Show loldle stats (stub) |
| `/ggwp` | private | Easter egg — "gg well played" |

## Architecture

- All commands return stub responses. Real game logic is not yet implemented.
- No `init()` hook — this module does not use KV storage yet.

## Database

**No KV usage currently.** When game logic is implemented, it will use KV namespace prefix `loldle:`.
