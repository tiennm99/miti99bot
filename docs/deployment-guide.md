# Deployment Guide

## Prerequisites

- Node.js ≥ 20.6
- Cloudflare account with Workers + KV + D1 enabled
- Telegram bot token from [@BotFather](https://t.me/BotFather)
- `wrangler` CLI authenticated: `npx wrangler login`

## Environment Setup

### 1. Cloudflare D1 Database (Optional but Recommended)

If your modules need relational data or append-only history, set up a D1 database:

```bash
npx wrangler d1 create miti99bot-db
```

Copy the database ID from the output, then add it to `wrangler.toml`:

```toml
[[d1_databases]]
binding = "DB"
database_name = "miti99bot-db"
database_id = "<paste-id-here>"
```

After this, run migrations to set up tables:

```bash
npm run db:migrate
```

The migration runner discovers all `src/modules/*/migrations/*.sql` files and applies them.

### 2. Cloudflare KV Namespaces

```bash
npx wrangler kv namespace create miti99bot-kv
npx wrangler kv namespace create miti99bot-kv --preview
```

Paste returned IDs into `wrangler.toml`:

```toml
[[kv_namespaces]]
binding = "KV"
id = "<production-id>"
preview_id = "<preview-id>"
```

### 3. Worker Secrets

```bash
npx wrangler secret put TELEGRAM_BOT_TOKEN
npx wrangler secret put TELEGRAM_WEBHOOK_SECRET
```

`TELEGRAM_WEBHOOK_SECRET` — any high-entropy string (e.g. `openssl rand -hex 32`). grammY validates it on every webhook update via `X-Telegram-Bot-Api-Secret-Token`.

### 4. Local Dev Config

```bash
cp .dev.vars.example .dev.vars    # for wrangler dev
cp .env.deploy.example .env.deploy # for register script
```

Both are gitignored. Fill in matching token + secret values.

`.env.deploy` also needs:
- `WORKER_URL` — the `*.workers.dev` URL (known after first deploy)
- `MODULES` — comma-separated, must match `wrangler.toml` `[vars].MODULES`

## Deploy

### Cron Configuration (if using scheduled jobs)

If any of your modules declare crons, they MUST also be registered in `wrangler.toml`:

```toml
[triggers]
crons = ["0 17 * * *", "0 2 * * *"]  # list all cron schedules used by modules
```

The schedule string must exactly match what modules declare. For details on cron expressions and examples, see [`docs/using-cron.md`](./using-cron.md).

### First Time

```bash
npx wrangler deploy                # learn the *.workers.dev URL
# paste URL into .env.deploy as WORKER_URL
npm run db:migrate                 # apply any migrations to D1
npm run register:dry               # preview payloads
npm run deploy                     # deploy + register webhook + commands
```

### Subsequent Deploys

```bash
npm run deploy
```

This runs `wrangler deploy`, `npm run db:migrate`, then `scripts/register.js` (setWebhook + setMyCommands).

### What the Register Script Does

`scripts/register.js` imports the same registry the Worker uses, builds it with a stub KV to derive public commands, then calls two Telegram APIs:

1. `setWebhook` — points Telegram at `WORKER_URL/webhook` with the secret token
2. `setMyCommands` — pushes public command list to Telegram's `/` menu

### Dry Run

```bash
npm run register:dry
```

Prints both payloads (webhook secret redacted) without calling Telegram.

## Secret Rotation

1. Generate new secret: `openssl rand -hex 32`
2. Update Cloudflare: `npx wrangler secret put TELEGRAM_WEBHOOK_SECRET`
3. Update `.env.deploy` with same value
4. Redeploy: `npm run deploy` (register step re-calls setWebhook with new secret)

Both values MUST match — mismatch causes 401 on every webhook.

## Adding/Removing Modules

When changing the active module list:

1. Update `MODULES` in `wrangler.toml` `[vars]`
2. Update `MODULES` in `.env.deploy`
3. If adding: ensure module exists in `src/modules/index.js` import map
4. `npm run deploy`

Removing a module from `MODULES` makes it inert — its KV data remains but nothing loads it.

## Rollback

Cloudflare Workers supports instant rollback via the dashboard or:

```bash
npx wrangler rollback
```

To disable a specific module without rollback, remove its name from `MODULES` and redeploy.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| 401 on all webhooks | Secret mismatch between CF secret and `.env.deploy` | Re-set both to same value, redeploy |
| Bot doesn't respond to commands | `MODULES` missing the module name | Add to both `wrangler.toml` and `.env.deploy` |
| `command conflict` at deploy | Two modules register same command name | Rename one command |
| `missing env: X` from register | `.env.deploy` incomplete | Add missing variable |
| `--env-file` not recognized | Node < 20.6 | Upgrade Node |
| `/help` missing a module | Module has no public or protected commands | Add at least one non-private command |
