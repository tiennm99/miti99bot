# Wordle Module

Word guessing game — currently a stub proving the plugin system.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/wordle` | public | Start a wordle game (stub) |
| `/wstats` | protected | Show wordle stats |
| `/konami` | private | Easter egg — "secret wordle mode" |

## Architecture

- All commands return stub responses. Real game logic is not yet implemented.
- `/wstats` reads a `stats` key from KV (returns 0 games played when empty).
- Module captures `db` in closure via `init()` for future game state persistence.

## Database

KV namespace prefix: `wordle:`

| Key | Type | Description | Example |
|-----|------|-------------|---------|
| `stats` | JSON | Aggregate game statistics (planned) | `{ "gamesPlayed": 0 }` |

### Schema: `stats` (planned)

```json
{
  "gamesPlayed": 0
}
```

No data is currently written — the module only reads this key defensively.
