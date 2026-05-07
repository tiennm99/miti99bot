# Codebase Summary

## Overview

Telegram bot on Cloudflare Workers with a plug-n-play module system. grammY handles Telegram API; modules register commands with three visibility levels. Data stored in Cloudflare KV (behind a prefixed `KVStore` interface) or D1 (behind `SqlStore` interface).

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Runtime | Cloudflare Workers (V8 isolates) |
| Bot framework | grammY 1.x |
| Storage | Cloudflare KV + D1 |
| AI inference | Workers AI binding (`env.AI`) |
| Linter/Formatter | Biome |
| Tests | Vitest |
| Deploy | Wrangler CLI |

## Active Modules

| Module | Commands | Storage | Crons | Description |
|--------|----------|---------|-------|-------------|
| `util` | `/info`, `/help`, `/stickerid` (private) | — | — | Bot info, command help renderer, sticker file_id echo helper |
| `misc` | `/ping`, `/mstats`, `/fortytwo` | KV | — | Health check + DB demo stub |
| `trading` | `/trade_topup`, `/trade_buy`, `/trade_sell`, `/trade_convert`, `/trade_stats`, `/history` | D1 (trades) + KV (portfolio, symbol cache) | Daily 5PM trim | Paper trading — VN stocks with dynamic symbol resolution |
| `wordle` | `/wordle`, `/wordle_new`, `/wordle_giveup`, `/wordle_stats` | KV | — | 5-letter word guessing game. 14,855-word dict |
| `loldle` | `/loldle`, `/loldle_giveup`, `/loldle_stats` | KV | — | Classic-mode LoL champion guesser. Data synced from `tiennm99/loldle-data` |
| `lolschedule` | `/lolschedule_today`, `/lolschedule_week`, `/lolschedule_subscribe`, `/lolschedule_unsubscribe` | KV | Daily 01:00 UTC | LoL esports schedule + daily digest subscriptions |
| `semantle` | `/semantle`, `/semantle_giveup`, `/semantle_stats` | KV | — | English semantic word guessing via hosted word2sim service |
| `doantu` | `/doantu`, `/doantu_hint`, `/doantu_giveup`, `/doantu_stats` | KV | — | Vietnamese semantle via hosted phow2sim service |
| `twentyq` | `/twentyq`, `/twentyq_giveup`, `/twentyq_stats` | KV | — | Reverse-Akinator yes/no game. Workers AI (`@cf/google/gemma-4-26b-a4b-it`) generates round-start category+hint and judges each turn via one-line JSON |

## Key Data Flows

### Command Processing
```
Telegram update → POST /webhook → grammY secret validation
→ getBot(env) → dispatcher routes /cmd → module handler
→ handler reads/writes KV via db.getJSON/putJSON (or D1 via sql.all/run)
→ ctx.reply() → response to Telegram
```

### Scheduled Job (Cron)
```
Cloudflare timer fires (e.g., "0 17 * * *")
→ scheduled(event, env, ctx) handler
→ getRegistry(env) → load + init modules
→ dispatchScheduled(event, env, ctx, registry)
→ filter matching crons by event.cron
→ for each: handler reads/writes D1 via sql.all/run (or KV via db)
→ ctx.waitUntil(promise) keeps handler alive
```

### Deploy Pipeline
```
npm run deploy
→ wrangler deploy (upload to CF, set env vars and bindings)
→ npm run db:migrate (apply any new migrations to D1)
→ scripts/register.js → buildRegistry with stub KV
→ POST setWebhook + POST setMyCommands to Telegram API
```

## External Dependencies

| Dependency | Purpose | Version |
|-----------|---------|---------|
| `grammy` | Telegram Bot API framework | ^1.30.0 |
| `@biomejs/biome` | Linting + formatting (dev) | ^1.9.0 |
| `vitest` | Test runner (dev) | ^4.1.4 |
| `wrangler` | Cloudflare Workers CLI (dev) | ^4.84.0 |

## Module Documentation

Each module maintains its own `README.md` with commands, data model, and implementation details. See `src/modules/<name>/README.md`.

## Tests

`npm test` runs the full vitest suite (run in a few seconds — ~450 tests). Structure: one folder per module under `tests/modules/<name>/`, shared fakes under `tests/fakes/` (fake-kv-namespace, fake-d1, fake-bot, fake-modules, fake-ai). No workerd, no Telegram fixtures — pure-logic unit tests with injected fakes.
