# lolschedule

LoL esports match schedule via the **lolesports.com** esports-api (the data feed behind the official lolesports.com website).

## Commands

| Command | Description |
|---|---|
| `/lolschedule_today` | Today's matches (ICT), grouped by league. Scores for played + live, times for upcoming. |
| `/lolschedule_week`  | Next 7 days, grouped by day → league. |
| `/lolschedule_subscribe` | Opt the current chat into the daily 08:00 ICT digest. |
| `/lolschedule_unsubscribe` | Stop receiving the digest. |

## Cron

| Schedule (UTC) | Local (ICT) | Purpose |
|---|---|---|
| `0 1 * * *` | 08:00 | Fan today's major-league schedule out to every chat subscribed via `/lolschedule_subscribe`. Skipped when no subscribers, no token, or no matches today. |

Subscribers are stored under the module's `subscribers` KV key as a JSON array of chat ids. Per-chat failures during fan-out are logged and swallowed so a single blocked chat can't stop the others.

## Data source

- Endpoint: `https://esports-api.lolesports.com/persisted/gw/getSchedule`
- Header: `x-api-key: <public key>` — the same key lolesports.com's own web client sends. No registration, no token lifecycle. If Riot rotates it, lift the new value from their public JS bundle.
- Companion endpoints used during design: `getLive`, `getLeagues`.

## Why not Leaguepedia?

Initial design hit Fandom's anonymous IP rate limit even from Cloudflare Worker egress (~1–2 req/min). See `plans/reports/researcher-260421-0845-leaguepedia-api-verification.md` and the auth-token follow-up. lolesports.com API is:

- Official Riot-operated
- Rate-limit friendly (powers the live site)
- Richer shape: state (`unstarted` / `inProgress` / `completed`), per-team `result.gameWins`, league metadata, bestOf strategy

## Event shape (relevant fields)

```
{
  startTime: "2026-04-21T09:00:00Z",
  state: "unstarted" | "inProgress" | "completed",
  blockName: "Week 4",
  league: { name: "LCK", slug: "lck" },
  match: {
    id: "...",
    teams: [
      { name, code, image, result?: { outcome, gameWins }, record?: {...} },
      ...
    ],
    strategy: { type: "bestOf", count: 3 }
  }
}
```

## Caching

Cache-first with KV. Key is `matches:{fromIso}:{toIso}`.

- Fresh TTL: 120 s (catches live score updates quickly)
- Stale fallback: up to 1 h on upstream failure
- No cron pre-warm needed — upstream is cheap

## Grouping

- `/lolschedule_today` — one section per league (header + match lines).
- `/lolschedule_week`  — one section per ICT day; within each day, leagues are sub-grouped.
- League ordering follows `LEAGUE_ORDER` in `format.js` (worlds / msi / first_stand first, then LCK / LPL / LEC / LCS, then the rest).

## Subscribers

`/lolschedule_subscribe` adds `ctx.chat.id` to the module's `subscribers` KV key (JSON array). `/lolschedule_unsubscribe` removes it. Both are idempotent and reply with the new state. The daily cron reads this list; empty list means the cron skips cleanly.

## Time zone

All rendering is in **ICT (UTC+7)**. `startTime` is UTC ISO; day boundaries for the `/lolschedule_today` and `/lolschedule_week` windows are anchored to ICT midnight.

## Files

- `index.js` — module contract
- `api-client.js` — getSchedule client with pagination + cache
- `format.js` — pure renderers (`formatEventLine`, `renderToday`, `renderWeek`)
- `handlers.js` — grammY command handlers, ICT day boundaries, cron fan-out
- `subscribers.js` — KV-backed add/list/remove for the daily-push list
