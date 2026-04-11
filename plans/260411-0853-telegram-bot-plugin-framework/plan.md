---
title: "Telegram Bot Plugin Framework on Cloudflare Workers"
description: "Greenfield JS Telegram bot with plug-n-play modules, KV-backed store, webhook on CF Workers."
status: pending
priority: P2
effort: 14h
branch: main
tags: [telegram, cloudflare-workers, grammy, plugin-system, javascript]
created: 2026-04-11
---

# Telegram Bot Plugin Framework

Greenfield JS bot on Cloudflare Workers. grammY + KV. Modules load from `MODULES` env var. Three command visibility levels (public/protected/private). Pluggable DB behind a minimal interface.

## Phases

| # | Phase | Status | Effort | Blockers |
|---|---|---|---|---|
| 01 | [Scaffold project](phase-01-scaffold-project.md) | pending | 1h | — |
| 02 | [Webhook entrypoint](phase-02-webhook-entrypoint.md) | pending | 1h | 01 |
| 03 | [DB abstraction](phase-03-db-abstraction.md) | pending | 1.5h | 01 |
| 04 | [Module framework](phase-04-module-framework.md) | pending | 2.5h | 02, 03 |
| 05 | [util module](phase-05-util-module.md) | pending | 1.5h | 04 |
| 06 | [Stub modules](phase-06-stub-modules.md) | pending | 1h | 04 |
| 07 | [Post-deploy register script](phase-07-deploy-register-script.md) | pending | 1h | 05 |
| 08 | [Tests](phase-08-tests.md) | pending | 2.5h | 04, 05, 06 |
| 09 | [Deploy + docs](phase-09-deploy-docs.md) | pending | 2h | 07, 08 |

## Key dependencies
- grammY `^1.30.0`, adapter `"cloudflare-mod"`
- wrangler (npm), Cloudflare account, KV namespace
- vitest (plain node pool — pure-logic tests only, no workerd)
- biome (lint + format, single tool)

## Architecture snapshot
- `src/index.js` — fetch handler: `POST /webhook` + `GET /` health. No admin HTTP surface.
- `src/bot.js` — memoized `Bot` instance + dispatcher wiring
- `src/db/` — `KVStore` interface (JSDoc with `getJSON/putJSON/list-cursor`), `cf-kv-store.js`, `create-store.js` (prefixing factory)
- `src/modules/registry.js` — load modules, build command tables (public/protected/private + unified `allCommands`), detect conflicts, expose `getCurrentRegistry`
- `src/modules/dispatcher.js` — grammY middleware: every command in `allCommands` registered via `bot.command()` regardless of visibility (private = hidden slash command)
- `src/modules/index.js` — static import map `{ util: () => import("./util/index.js"), ... }`
- `src/modules/{util,wordle,loldle,misc}/index.js` — module entry points
- `scripts/register.js` — post-deploy node script calling Telegram `setWebhook` + `setMyCommands`. Chained via `npm run deploy = wrangler deploy && npm run register`.

## Open questions
1. **grammY version pin** — pick exact version at phase-01 time (`npm view grammy version`). Plan assumes `^1.30.0`.

*Resolved in revision pass (2026-04-11):*
- Private commands are hidden slash commands (same regex, routed via `bot.command()`), not text-match easter eggs.
- No admin HTTP surface; post-deploy `scripts/register.js` handles `setWebhook` + `setMyCommands`. `ADMIN_SECRET` dropped entirely.
- KV interface exposes `expirationTtl`, `list()` cursor, and `getJSON` / `putJSON` helpers. No `metadata`.
- Conflict policy: unified namespace across all visibilities, throw at `buildRegistry`.
- `/help` parse mode: HTML.
