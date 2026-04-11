# miti99bot

[My Telegram bot](https://t.me/miti99bot) — a plug-n-play bot on Cloudflare Workers.

Modules are added or removed via a single `MODULES` env var. Each module registers its own commands with three visibility levels (public / protected / private). Data lives in Cloudflare KV behind a thin `KVStore` interface, so swapping the backend later is a one-file change.

## Architecture snapshot

```
src/
├── index.js             # fetch handler: POST /webhook + GET / health
├── bot.js               # memoized grammY Bot, lazy dispatcher install
├── db/
│   ├── kv-store-interface.js   # JSDoc typedefs (the contract)
│   ├── cf-kv-store.js          # Cloudflare KV implementation
│   └── create-store.js         # per-module prefixing factory
├── modules/
│   ├── index.js         # static import map — register new modules here
│   ├── registry.js      # load, validate, build command tables
│   ├── dispatcher.js    # wires every command via bot.command()
│   ├── validate-command.js
│   ├── util/            # /info, /help (fully implemented)
│   ├── wordle/          # stub — proves plugin system
│   ├── loldle/          # stub
│   └── misc/            # stub
└── util/
    └── escape-html.js
scripts/
├── register.js          # post-deploy: setWebhook + setMyCommands
└── stub-kv.js
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
npm run dev      # wrangler dev — runs the Worker at http://localhost:8787
npm run lint     # biome check
npm test         # vitest
```

The local `wrangler dev` server exposes `GET /` (health) and `POST /webhook`. For end-to-end testing you'd ngrok/cloudflared the local port and point a test bot's `setWebhook` at it — but pure unit tests (`npm test`) cover the logic seams without Telegram.

## Deploy

Single command, idempotent:

```bash
npm run deploy
```

That runs `wrangler deploy` followed by `scripts/register.js`, which calls Telegram's `setWebhook` + `setMyCommands` using values from `.env.deploy`.

First-time deploy flow:

1. Run `wrangler deploy` once to learn the `*.workers.dev` URL printed at the end.
2. Paste it into `.env.deploy` as `WORKER_URL`.
3. Preview the register payloads without calling Telegram:
   ```bash
   npm run register:dry
   ```
4. Run the real thing:
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

## Planning docs

Full implementation plan in `plans/260411-0853-telegram-bot-plugin-framework/` — 9 phase files plus researcher reports.
