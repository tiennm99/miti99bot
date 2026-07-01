# Deploy: Self-host (Coolify + MongoDB Atlas)

Run `miti99bot` as a long-lived container on [Coolify](https://coolify.io) with
[MongoDB Atlas](https://www.mongodb.com/atlas) (free M0) for storage.

## Architecture

```
   Telegram  <── long poll (getUpdates) ──  container  (outbound only)
   in-process scheduler ───────────────────> module crons
   MongoDB Atlas (db / one collection per module + system metadata)
   Coolify env vars (plain secrets)
   NO public ingress (polling = outbound only; no domain, no /webhook, no TLS in)
```

- **Storage** — `mongodb` auto-selected when `MONGO_URL` is set (no `KV_PROVIDER`).
- **Cron** — an in-process scheduler (`internal/cron`) runs unconditionally and
  fires each module cron on its `Schedule` (UTC).
- **Transport** — long polling (`b.Start`) is the **only** transport. The bot
  opens an outbound connection to Telegram and pulls updates, so there is no
  public domain, no `/webhook`, and no webhook secret. The container clears any
  leftover webhook on startup (`deleteWebhook`) before polling.

## Required environment

Copy [`.env.example`](../.env.example) → `.env` (gitignored) and fill in.

| Var | Required | Notes |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | ✅ | from @BotFather |
| `MONGO_URL` | ✅ | Atlas SRV string **incl. credentials** — secret, never logged |
| `MONGO_DATABASE` | ✅ | e.g. `miti99bot` |
| `MODULES` | optional | CSV; empty = all modules |
| `OWNER_ID` | optional | owner-only commands (renamed from `BOT_OWNER_ID`) |
| `ADMIN_IDS` | optional | CSV of admin ids (renamed from `ADMIN_USER_IDS`) |
| `WC_FOOTBALL_DATA_TOKEN` | optional | football-data.org token for the `wc` module |

**Leave UNSET on self-host:** `KV_PROVIDER`, `PORT`,
`TELEGRAM_WEBHOOK_SECRET`, and `GOLD_VNAPP_API_KEY`. Stock, coin, and gold URL
overrides are not supported in runtime env; modules use coded defaults.

> Cron runs in-process (`internal/cron`) — there is no `/cron` HTTP route and no
> `CRON_SHARED_SECRET`. The scheduler is the sole trigger; nothing inbound.

## 1. MongoDB Atlas (M0)

1. Create a free **M0** cluster (512 MB — ample for the tiny paper-trading KV).
2. **Database user (least privilege):** create a user with role
   **`readWrite` on the single app database only** (e.g. `miti99bot`) — never
   Atlas admin or cluster-wide. Use a **strong unique password**.
3. **Network access:** add `0.0.0.0/0`.

   > **Accepted trade-off (validated decision).** The Coolify host has no stable
   > egress IP, so the Atlas IP allow-list is open to the internet. This widens
   > the database surface. The mandatory compensating controls are: (1) strong
   > unique password, (2) least-privilege `readWrite`-on-one-db user, (3) the
   > connection string is a secret and is never logged (the bot logs only the
   > database name on startup).

4. Copy the `mongodb+srv://…` connection string into `MONGO_URL` and put the
   db name in `MONGO_DATABASE`.

> Storage layout: one collection per module. Each document is a flattened native
> document — `{ _id: <user key>, ...payload fields, version, updatedAt }` with no
> `value` envelope. Payload fields are hoisted to the document root so they
> expand and are queryable in Compass. The two non-object values are wrapped in a
> named field: `lol` schedule subscribers under `subscribers` (array) and the
> daily push date under `date`. Concurrency uses the `version` field (optimistic lock);
> `updatedAt` is a BSON Date.
>
> The `stats` collection uses queryable aggregate documents for command/user
> counts and creates indexes on startup. Deleted legacy command rows are retained
> with `deleted: true`; `/stats` queries filter those rows from visible results.
> A historical `system` collection may remain in MongoDB with completed migration
> records and can be reused if a future one-time startup migration is needed.

## 2. Coolify

1. New resource → from this Git repo (Docker Compose), or a prebuilt image.
   The committed [`compose.yml`](../compose.yml) defines the single
   `bot` service.
2. Set the env vars above in Coolify.
3. **No public domain / port** is needed — polling is outbound-only. Do not
   publish a port or attach a domain. `expose: 8080` keeps the health endpoint
   reachable only inside Coolify's network.
4. **Exactly one replica.** Telegram permits only one `getUpdates` consumer per
   bot token; a second poller gets HTTP 409, and a second in-process scheduler
   double-fires crons. Prefer **stop-first redeploys** so two containers never
   overlap near a cron time.
5. **deploynotify commit SHA:** `SOURCE_COMMIT` is a Coolify predefined
   variable. The bot reads it at startup and DMs the owner on every boot;
   outside Coolify (local `docker compose up`) it is unset and the DM shows
   `unknown`. Keep "Include Source Commit in Build" disabled: that setting
   affects build args only, is not needed for this runtime path, and would
   invalidate Docker cache on every commit. Do not add `SOURCE_COMMIT` to
   `compose.yml`; an interpolated empty value can override Coolify's runtime
   env-file value.
6. **Health check:** use Coolify's HTTP monitor against `GET /` (returns
   `text/plain` `miti99bot ok`). Do **not** use a compose `healthcheck` — the
   distroless image has no shell/curl and `cmd/server` has no `-healthcheck`
   flag. Note: `/` reports healthy even if Mongo is unreachable (the driver
   auto-reconnects on the next op); a DB outage will not auto-restart the
   container — accepted trade-off.

## 3. Command menu

The bot registers its Telegram command menu from loaded public modules on
startup. The manual target remains useful for repairs or local experiments:

```sh
TELEGRAM_BOT_TOKEN=… make telegram-commands
```

## Operations

The live deployment is the Coolify container and MongoDB is the sole system of
record. Keep exactly one replica running. To confirm Telegram is in polling mode:

```sh
TELEGRAM_BOT_TOKEN=… make telegram-webhook-info
```

`url` should be empty. If needed, clear the webhook explicitly:

```sh
TELEGRAM_BOT_TOKEN=… make telegram-deletewebhook
```

## Local smoke test

```sh
cp .env.example .env   # fill TELEGRAM_BOT_TOKEN, MONGO_URL, MONGO_DATABASE
docker compose up --build
```

Boot logs should show `storage backend backend=mongodb database=…` (no
connection string), `cron scheduler started`, and `telegram long polling
started`. `curl localhost:8080/` returns `miti99bot ok`. The bot's webhook must
be unset (the container clears it on startup) or `getUpdates` 409s.
