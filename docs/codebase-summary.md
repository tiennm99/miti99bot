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

| Module | Status | Commands | Description |
|--------|--------|----------|-------------|
| `util` | Complete | `/info`, `/help` | Bot info and command help renderer |
| `trading` | Complete | `/trade_topup`, `/trade_buy`, `/trade_sell`, `/trade_convert`, `/trade_stats` | Paper trading — VN stocks with dynamic symbol resolution. Crypto/gold/forex coming soon. |
| `misc` | Stub | `/ping`, `/mstats`, `/fortytwo` | Health check + DB demo |
| `wordle` | Stub | `/wordle`, `/wstats`, `/konami` | Placeholder for word game |
| `loldle` | Stub | `/loldle`, `/ggwp` | Placeholder for LoL game |

## Key Data Flows

### Command Processing
```
Telegram update → POST /webhook → grammY secret validation
→ getBot(env) → dispatcher routes /cmd → module handler
→ handler reads/writes KV via db.getJSON/putJSON
→ ctx.reply() → response to Telegram
```

### Deploy Pipeline
```
npm run deploy → wrangler deploy (upload to CF)
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

105 tests across 11 test files:

| Area | Tests | What's Covered |
|------|-------|---------------|
| DB layer | 19 | KV store, prefixing, JSON helpers, pagination |
| Module framework | 33 | Registry, dispatcher, validators, help renderer |
| Utilities | 4 | HTML escaping |
| Trading module | 49 | Dynamic symbol resolution, formatters, flat portfolio CRUD, command handlers |
