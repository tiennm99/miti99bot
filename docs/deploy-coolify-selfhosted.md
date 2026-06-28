# Deploy: Self-host (Coolify + MongoDB Atlas)

Run `miti99bot` as a long-lived container on [Coolify](https://coolify.io) with
[MongoDB Atlas](https://www.mongodb.com/atlas) (free M0) for storage. This
replaces the AWS Lambda + DynamoDB + EventBridge path.

## Architecture

```
   Telegram  <── long poll (getUpdates) ──  container  (outbound only)
   in-process scheduler ───────────────────> module crons
   MongoDB Atlas (db / one collection per module)
   Coolify env vars (plain secrets)
   NO public ingress (polling = outbound only; no domain, no /webhook, no TLS in)
```

Same Go binary (`cmd/server`) and module framework as AWS. Three things differ,
all selected automatically from env:

- **Storage** — `mongodb` auto-selected when `MONGO_URL` is set (no `KV_PROVIDER`).
- **Cron** — an in-process scheduler (`internal/cron`) runs unconditionally and
  fires each module cron on its `Schedule` (UTC). No EventBridge.
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
| `GEMINI_API_KEY` | optional | only the `twentyq` module needs it |

**Leave UNSET on self-host:** all `*_PARAMETER_NAME` vars (they force an
SSM/AWS lookup that fails with no AWS creds and bricks startup), `KV_PROVIDER`,
`PORT`, `TELEGRAM_WEBHOOK_SECRET`, `GOLD_VNAPP_API_KEY`, and the
`STOCK/COIN/GOLD *_API_URL` overrides (modules use coded defaults).

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
   > the surface beyond DynamoDB's IAM-gated posture (where the DB was never
   > internet-reachable). It is knowingly accepted for self-host. The mandatory
   > compensating controls are: (1) strong unique password, (2) least-privilege
   > `readWrite`-on-one-db user, (3) the connection string is a secret and is
   > never logged (the bot logs only the database name on startup).

4. Copy the `mongodb+srv://…` connection string into `MONGO_URL` and put the
   db name in `MONGO_DATABASE`.

> Storage layout: one collection per module; each document is a flattened native
> document — `{ _id: <user key>, ...payload fields, version, updatedAt }` with no
> `value` envelope. Payload fields are hoisted to the document root so they
> expand and are queryable in Compass. The two non-object values are wrapped in a
> named field: lolschedule subscribers under `subscribers` (array) and the daily
> push date under `date`. Concurrency uses the `version` field (optimistic lock);
> `updatedAt` is a BSON Date. The DynamoDB→Mongo migrator writes this shape
> directly, and is idempotent.

## 2. Coolify

1. New resource → from this Git repo (Docker Compose), or a prebuilt image.
   The committed [`docker-compose.yml`](../docker-compose.yml) defines the single
   `bot` service.
2. Set the env vars above in Coolify.
3. **No public domain / port** is needed — polling is outbound-only. Do not
   publish a port or attach a domain. `expose: 8080` keeps the health endpoint
   reachable only inside Coolify's network.
4. **Exactly one replica.** Telegram permits only one `getUpdates` consumer per
   bot token; a second poller gets HTTP 409, and a second in-process scheduler
   double-fires crons. Prefer **stop-first redeploys** so two containers never
   overlap near a cron time.
5. **deploynotify commit SHA:** Coolify injects `SOURCE_COMMIT` (a predefined
   runtime env var) into the container, and the compose `environment` forwards
   it, so the bot reads it at startup and DMs the owner the "new version"
   notice — no manual wiring. Without it, `deploynotify` stays silent (no
   crash) — but you lose that notification.
6. **Health check:** use Coolify's HTTP monitor against `GET /` (returns
   `text/plain` `miti99bot ok`). Do **not** use a compose `healthcheck` — the
   distroless image has no shell/curl and `cmd/server` has no `-healthcheck`
   flag. Note: `/` reports healthy even if Mongo is unreachable (the driver
   auto-reconnects on the next op); a DB outage will not auto-restart the
   container — accepted trade-off.

## 3. Register the command menu

Long polling needs no webhook registration — only the command menu:

```sh
TELEGRAM_BOT_TOKEN=… make telegram-commands-selfhost
```

## Cutover runbook

Zero-loss switch from the live AWS Lambda to the Coolify poller. Coordinate
users to pause activity during the brief window.

1. **Deploy the Coolify container but keep it stopped/scaled-to-0.** A running
   poller would 409 against the live Lambda webhook and its scheduler would
   overlap EventBridge. Use a fresh, empty Atlas DB.
2. **Disable/delete the EventBridge schedule** `miti99bot-lolschedule-daily-push`
   (it invokes the Lambda directly, independent of transport). The in-process
   scheduler's per-UTC-date idempotency guard is the backup.
3. **`deleteWebhook` (mandatory):**
   ```sh
   TELEGRAM_BOT_TOKEN=… make telegram-deletewebhook-selfhost
   ```
   This buffers incoming updates (Telegram retains ~24h) so nothing is lost, and
   releases the webhook so the poller won't 409. After this the Lambda stops
   receiving updates.
4. **Migrate + verify** (read-only on DynamoDB — see
   [`cmd/migrate-dynamo-to-mongo`](../cmd/migrate-dynamo-to-mongo/README.md)):
   ```sh
   export MONGO_URL=… MONGO_DATABASE=… AWS_PROFILE=miti99bot-migrate
   make migrate-dynamo-to-mongo DRY_RUN=1   # review counts
   make migrate-dynamo-to-mongo             # real run
   make migrate-verify                      # counts must match, exit 0
   ```
   Keep this window short (target minutes).
5. **Start the Coolify container** (1 replica). Its scheduler runs by default
   (safe now that EventBridge is off). On startup it `deleteWebhook`s again
   (idempotent) and begins polling, draining Telegram's buffered queue.
6. **Confirm:**
   ```sh
   TELEGRAM_BOT_TOKEN=… make telegram-webhook-info-selfhost
   ```
   `url` should be empty and `pending_update_count` should drain toward 0.
   Smoke `/ping`, `/stats`, and a coin/stock balance command — migrated state
   should be visible from Atlas.
7. **Tear down AWS** once verified — see
   [`aws-decommission-runbook.md`](./aws-decommission-runbook.md).

### Rollback

The only clean revert is **before `sam delete`**: stop the poller, then
re-`setWebhook` to the Lambda Function URL (the still-deployed Lambda runs its
own webhook code until teardown). Lossless **only until the first post-cutover
Mongo write** — after that, MongoDB/Coolify is the sole system of record. This
short-window RPO was an explicitly accepted decision; no reverse migrator
exists.

## Local smoke test

```sh
cp .env.example .env   # fill TELEGRAM_BOT_TOKEN, MONGO_URL, MONGO_DATABASE
docker compose up --build
```

Boot logs should show `storage backend backend=mongodb database=…` (no
connection string), `cron scheduler started`, and `telegram long polling
started`. `curl localhost:8080/` returns `miti99bot ok`. The bot's webhook must
be unset (the container clears it on startup) or `getUpdates` 409s.
