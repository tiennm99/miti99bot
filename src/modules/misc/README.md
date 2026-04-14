# Misc Module

Health check and DB demonstration. Exercises all three visibility levels.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/ping` | public | Health check — replies "pong" and records timestamp |
| `/mstats` | protected | Shows last `/ping` timestamp |
| `/fortytwo` | private | Easter egg — replies "The answer." |

## Architecture

- `/ping` writes a timestamp to KV on every call (best-effort — KV failure doesn't block the reply).
- `/mstats` reads the last ping timestamp and formats it as ISO string.
- `/fortytwo` is a hidden command demonstrating private visibility.

## Database

KV namespace prefix: `misc:`

| Key | Type | Description | Example |
|-----|------|-------------|---------|
| `last_ping` | JSON | Timestamp of the most recent `/ping` call | `{ "at": 1713100000000 }` |

### Schema: `last_ping`

```json
{
  "at": 1713100000000
}
```

- `at` — Unix epoch milliseconds (`Date.now()`)
