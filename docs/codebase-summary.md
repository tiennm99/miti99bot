# Codebase Summary

## Overview

Telegram bot on Cloudflare Workers with a plug-n-play module system. grammY handles Telegram API; modules register commands with three visibility levels. Data stored in Cloudflare KV behind a prefixed `KVStore` interface.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Runtime | Cloudflare Workers (V8 isolates) |
| Bot framework | grammY 1.x |
| Storage | Cloudflare KV |
| Linter/Formatter | Biome |
| Tests | Vitest |
| Deploy | Wrangler CLI |

## Active Modules

| Module | Status | Commands | Storage | Crons | Description |
|--------|--------|----------|---------|-------|-------------|
| `util` | Complete | `/info`, `/help` | — | — | Bot info and command help renderer |
| `trading` | Complete | `/trade_topup`, `/trade_buy`, `/trade_sell`, `/trade_convert`, `/trade_stats`, `/history` | D1 (trades) + KV (portfolio, symbol cache) | Daily 5PM trim | Paper trading — VN stocks with dynamic symbol resolution. Crypto/gold/forex coming soon. |
| `wordle` | Complete | `/wordle`, `/wordle_new`, `/wordle_giveup`, `/wordle_stats` | KV (game, stats) | — | Classic 5-letter word game. 14,855-word dict sourced from [dracos's gist](https://gist.github.com/dracos/dd0668f281e685bad51479e5acaadb93). |
| `loldle` | Complete | `/loldle`, `/loldle_new`, `/loldle_giveup`, `/loldle_stats` | KV (game, stats) | — | Classic-mode LoL champion guesser. Champion data synced from `tiennm99/loldle-data`. |
| `misc` | Stub | `/ping`, `/mstats`, `/fortytwo` | KV | — | Health check + DB demo |

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
| `vitest` | Test runner (dev) | ^2.1.0 |
| `wrangler` | Cloudflare Workers CLI (dev) | ^3.90.0 |

## Module Documentation

Each module maintains its own `README.md` with commands, data model, and implementation details. See `src/modules/<name>/README.md`.

## Test Coverage

200 tests across 21 test files (run via `npm test` — ~2s):

| Area | Tests | What's Covered |
|------|-------|---------------|
| DB layer (KV) | 19 | KV store, prefixing, JSON helpers, pagination |
| DB layer (D1) | — | Fake D1 in-memory implementation (fake-d1.js) backs trading tests |
| Module framework | 33 | Registry, dispatcher, validators, help renderer, cron validation |
| Utilities | 4 | HTML escaping |
| Trading module | 79 | Symbol resolution, formatters, flat portfolio CRUD, command handlers, history/retention |
| Loldle module | 18 | Classic-mode champion comparison, champion lookup, daily picker |
| Wordle module | 13 | Duplicate-letter two-pass comparison, guess validation |
