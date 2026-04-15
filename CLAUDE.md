# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
npm run dev          # local dev server (wrangler dev) at http://localhost:8787
npm run lint         # biome check src tests scripts + eslint src
npm run format       # biome format --write
npm test             # vitest run (all tests)
npx vitest run tests/modules/trading/format.test.js   # single test file
npx vitest run -t "formats with dot"                  # single test by name
npm run db:migrate   # apply migrations to D1 (prod)
npm run db:migrate -- --local                         # apply to local dev D1
npm run db:migrate -- --dry-run                       # preview without applying
npm run deploy       # wrangler deploy + db:migrate + register webhook/commands
npm run register:dry # preview setWebhook + setMyCommands payloads without calling Telegram
```

## Architecture

grammY Telegram bot on Cloudflare Workers. Modules are plug-n-play: each module is a folder under `src/modules/` that exports `{ name, commands[], init? }`. A single `MODULES` env var controls which modules are loaded.

**Request flow:** `POST /webhook` → grammY validates secret header → `getBot(env)` (memoized per warm instance) → `installDispatcher` builds registry on first call → `bot.command(name, handler)` for every command → handler runs.

**Key abstractions:**
- `src/modules/registry.js` — loads modules from static import map (`src/modules/index.js`), validates commands, detects name conflicts across all visibility levels, builds four maps (public/protected/private/all). Memoized via `getCurrentRegistry()`.
- `src/db/create-store.js` — wraps Cloudflare KV with auto-prefixed keys per module (`moduleName:key`). Modules never touch `env.KV` directly.
- `scripts/register.js` — post-deploy script that imports the same registry to derive public commands, then calls Telegram `setWebhook` + `setMyCommands`. Uses `stub-kv.js` to satisfy KV binding without real IO.

**Three command visibilities:** public (in Telegram `/` menu + `/help`), protected (in `/help` only), private (hidden easter eggs). All three are registered via `bot.command()` — visibility controls discoverability, not access.

## Adding a Module

1. Create `src/modules/<name>/index.js` with default export `{ name, commands, init? }`
2. Add one line to `src/modules/index.js` static import map
3. Add `<name>` to `MODULES` in `wrangler.toml` `[vars]`
4. Full guide: `docs/adding-a-module.md`

## Module Contract

```js
{
  name: "mymod",                              // must match folder + import map key
  init: async ({ db, sql, env }) => { ... },  // optional — db: KVStore, sql: SqlStore|null
  commands: [{
    name: "mycmd",                            // ^[a-z0-9_]{1,32}$, no leading slash
    visibility: "public",                     // "public" | "protected" | "private"
    description: "Does a thing",              // required for all visibilities
    handler: async (ctx) => { ... },          // grammY context
  }],
  crons: [{                                   // optional scheduled jobs
    schedule: "0 17 * * *",                   // cron expression
    name: "daily-cleanup",                    // unique within module
    handler: async (event, ctx) => { ... },   // receives { db, sql, env }
  }],
}
```

- Command names must be globally unique across ALL modules and visibilities. Conflicts throw at load time.
- Cron schedules declared here MUST also be registered in `wrangler.toml` `[triggers] crons`.
- For D1 setup (migrations, table naming), see [`docs/using-d1.md`](docs/using-d1.md).
- For cron syntax and testing, see [`docs/using-cron.md`](docs/using-cron.md).

## Testing

Pure-logic unit tests only — no workerd, no Telegram fixtures. Tests use fakes from `tests/fakes/` (fake-kv-namespace, fake-bot, fake-modules) injected via parameters, not `vi.mock`.

For modules that call `fetch` (like trading/prices), stub `global.fetch` with `vi.fn()` in tests.

## Code Style

Biome enforces: 2-space indent, double quotes, semicolons, trailing commas, 100-char line width, sorted imports. Run `npm run format` before committing. Keep files under 200 lines — split into focused submodules when approaching the limit.

## Environment

- Secrets (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`): set via `wrangler secret put`, mirrored in `.env.deploy` (gitignored) for register script
- `.dev.vars`: local dev secrets (gitignored), copy from `.dev.vars.example`
- Node >=20.6 required (for `--env-file` flag)
