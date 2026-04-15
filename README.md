# miti99bot

[My Telegram bot](https://t.me/miti99bot) — a plug-n-play bot framework for Cloudflare Workers.

Modules are added or removed via a single `MODULES` env var. Each module registers its own commands with three visibility levels (public / protected / private). Data lives in Cloudflare KV behind a thin `KVStore` interface, so swapping the backend later is a one-file change.

## Why

- **Drop-in modules.** Write a single file, list the folder name in `MODULES`, redeploy. No registration boilerplate, no manual command wiring.
- **Three visibility levels out of the box.** Public commands show in Telegram's `/` menu and `/help`; protected show only in `/help`; private are hidden slash-command easter eggs. One namespace, loud conflict detection.
- **Dual storage backends.** Modules talk to a small `KVStore` interface (Cloudflare KV for simple state) or `SqlStore` interface (D1 for relational data, scans, leaderboards). Swappable with one-file changes.
- **Scheduled jobs.** Modules declare cron-based cleanup, stats refresh, or maintenance tasks — registered via `wrangler.toml` and dispatched automatically.
- **Zero admin surface.** No in-Worker `/admin/*` routes, no admin secret. `setWebhook` + `setMyCommands` run at deploy time from a local node script.
- **Tested.** 105+ vitest unit tests cover registry, storage, dispatcher, cron validation, help renderer, validators, HTML escaping, and the trading module.

## How a request flows

```
Telegram sends update
        │
        ▼
POST /webhook  ◄── grammY validates X-Telegram-Bot-Api-Secret-Token (401 on miss)
        │
        ▼
getBot(env) ──► first call only: installDispatcher(bot, env)
        │                │
        │                ├── loadModules(env.MODULES.split(","))
        │                ├── per module: init({ db: createStore(name, env), env })
        │                ├── build publicCommands / protectedCommands / privateCommands
        │                │     + unified allCommands map (conflict check)
        │                └── for each entry: bot.command(name, handler)
        ▼
bot.handleUpdate(update) ──► grammY routes /cmd → registered handler
        │
        ▼
handler reads/writes via db.getJSON / db.putJSON (auto-prefixed as "module:key")
        │
        ▼
ctx.reply(...) → response back to Telegram
```

## Architecture snapshot

```
src/
├── index.js             # fetch + scheduled handlers: POST /webhook + cron triggers
├── bot.js               # memoized grammY Bot, lazy dispatcher + registry install
├── types.js             # JSDoc typedefs (central: Env, Module, Command, Cron, etc.)
├── db/
│   ├── kv-store-interface.js    # KVStore contract (JSDoc)
│   ├── cf-kv-store.js           # Cloudflare KV implementation
│   ├── create-store.js          # KV per-module prefixing factory
│   ├── sql-store-interface.js   # SqlStore contract (JSDoc)
│   ├── cf-sql-store.js          # Cloudflare D1 implementation
│   └── create-sql-store.js      # D1 per-module prefixing factory
├── modules/
│   ├── index.js             # static import map — register new modules here
│   ├── registry.js          # load, validate, build command + cron tables
│   ├── dispatcher.js        # wires every command via bot.command()
│   ├── cron-dispatcher.js   # dispatches cron handlers by schedule match
│   ├── validate-command.js  # command contract validator
│   ├── validate-cron.js     # cron contract validator
│   ├── util/                # /info, /help (fully implemented)
│   ├── trading/             # paper trading — VN stocks (D1 storage, daily cron)
│   │   └── migrations/
│   │       └── 0001_trades.sql
│   ├── wordle/              # stub — proves plugin system
│   ├── loldle/              # stub
│   └── misc/                # stub (KV storage)
└── util/
    └── escape-html.js

scripts/
├── register.js              # post-deploy: setWebhook + setMyCommands
├── migrate.js               # discover + apply D1 migrations
└── stub-kv.js               # no-op KV binding for deploy-time registry build

tests/
└── fakes/
    ├── fake-kv-namespace.js
    ├── fake-d1.js           # in-memory SQL for testing
    ├── fake-bot.js
    └── fake-modules.js
```

## Command visibility

| Level | In `/` menu | In `/help` | Callable |
|---|---|---|---|
| `public` | yes | yes | yes |
| `protected` | **no** | yes | yes |
| `private` | **no** | **no** | yes (hidden slash command — easter egg) |

All three are slash commands. Private commands are just hidden from both surfaces. They're not access control — anyone who knows the name can invoke them.

Command names must match `^[a-z0-9_]{1,32}$` (Telegram's slash-command limit). Conflict detection is unified across all visibility levels — two modules cannot register the same command name no matter the visibility. Registry build throws at load time.

## Prereqs

- Node.js ≥ 20.6 (for `node --env-file`)
- A Cloudflare account with Workers + KV
- A Telegram bot token from [@BotFather](https://t.me/BotFather)

## Setup

1. **Install dependencies**
   ```bash
   npm install
   ```

2. **Create KV namespaces** (production + preview)
   ```bash
   npx wrangler kv namespace create miti99bot-kv
   npx wrangler kv namespace create miti99bot-kv --preview
   ```
   Paste the returned IDs into `wrangler.toml` under `[[kv_namespaces]]`, replacing both `REPLACE_ME` placeholders.

3. **Set Worker runtime secrets** (stored in Cloudflare, used by the deployed Worker)
   ```bash
   npx wrangler secret put TELEGRAM_BOT_TOKEN
   npx wrangler secret put TELEGRAM_WEBHOOK_SECRET
   ```
   `TELEGRAM_WEBHOOK_SECRET` can be any high-entropy string — e.g. `openssl rand -hex 32`. It gates incoming webhook requests; grammY validates it on every update.

4. **Create `.dev.vars`** for local development
   ```bash
   cp .dev.vars.example .dev.vars
   # fill in the same TELEGRAM_BOT_TOKEN + TELEGRAM_WEBHOOK_SECRET values
   ```
   Used by `wrangler dev`. Gitignored.

5. **Create `.env.deploy`** for the post-deploy register script
   ```bash
   cp .env.deploy.example .env.deploy
   # fill in: token, webhook secret, WORKER_URL (known after first deploy), MODULES
   ```
   Gitignored. The `TELEGRAM_BOT_TOKEN` and `TELEGRAM_WEBHOOK_SECRET` values MUST match what you set via `wrangler secret put` — mismatch means every incoming webhook returns 401.

## Local dev

```bash
npm run dev         # wrangler dev — runs the Worker at http://localhost:8787
npm run lint        # biome check + eslint
npm test            # vitest
npm run db:migrate  # apply D1 migrations (--local for local dev, --dry-run to preview)
```

The local `wrangler dev` server exposes `GET /` (health), `POST /webhook` (Telegram), and `/__scheduled?cron=...` (cron simulation). For end-to-end testing you'd ngrok/cloudflared the local port and point a test bot's `setWebhook` at it — but pure unit tests (`npm test`) cover the logic seams without Telegram.

## Deploy

Single command, idempotent:

```bash
npm run deploy
```

That runs `wrangler deploy`, applies D1 migrations, then `scripts/register.js`, which calls Telegram's `setWebhook` + `setMyCommands` using values from `.env.deploy`.

First-time deploy flow:

1. Create D1 database: `npx wrangler d1 create miti99bot-db` and paste ID into `wrangler.toml`.
2. Run `wrangler deploy` once to learn the `*.workers.dev` URL printed at the end.
3. Paste it into `.env.deploy` as `WORKER_URL`.
4. Apply migrations: `npm run db:migrate`.
5. Preview the register payloads without calling Telegram:
   ```bash
   npm run register:dry
   ```
6. Run the real deploy:
   ```bash
   npm run deploy
   ```

Subsequent deploys: just `npm run deploy`.

## Adding a module

See [`docs/adding-a-module.md`](docs/adding-a-module.md) for the full guide.

TL;DR:

1. Create `src/modules/<name>/index.js` with a default export `{ name, commands, init? }`.
2. Add a line to `src/modules/index.js` static map.
3. Add `<name>` to `MODULES` in both `wrangler.toml` `[vars]` and `.env.deploy`.
4. `npm test` + `npm run deploy`.

## Troubleshooting

| Symptom | Cause |
|---|---|
| 401 on every webhook | `TELEGRAM_WEBHOOK_SECRET` differs between `wrangler secret` and `.env.deploy`. |
| `/help` is missing a module's section | Module has no public or protected commands — private-only modules are hidden. |
| Module loads but no commands respond | `MODULES` does not list the module. Check `wrangler.toml` AND `.env.deploy`. |
| `command conflict: /foo ...` at deploy | Two modules register the same command name. Rename one. |
| `npm run register` exits `missing env: X` | Add `X` to `.env.deploy`. |
| `--env-file` flag not recognized | Node < 20.6. Upgrade Node. |

## Further reading

- [`docs/architecture.md`](docs/architecture.md) — deeper dive: cold-start, module lifecycle, KV + D1 storage, cron dispatch, deploy flow, design tradeoffs.
- [`docs/adding-a-module.md`](docs/adding-a-module.md) — step-by-step guide to authoring a new module (commands, KV storage, D1 + migrations, crons).
- [`docs/using-d1.md`](docs/using-d1.md) — when to use D1, writing migrations, SQL API reference, worked examples.
- [`docs/using-cron.md`](docs/using-cron.md) — scheduling syntax, handler signature, wrangler.toml registration, local testing, worked examples.
- [`docs/deployment-guide.md`](docs/deployment-guide.md) — D1 + KV setup, migration, secret rotation, rollback.
- `plans/260415-1010-d1-cron-infra/` — phased implementation plan for D1 + cron support (6 phases + reports).
- `plans/260411-0853-telegram-bot-plugin-framework/` — original plugin framework implementation plan (9 phases + reports).
