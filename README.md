# miti99bot

Plug-n-play Telegram bot framework in Go. Self-hosted on Coolify + MongoDB Atlas via long polling and an in-process cron scheduler. (Previously ran on AWS Lambda + DynamoDB + EventBridge — now retired; see [`docs/deploy-aws.md`](docs/deploy-aws.md) for history.)

## Modules

| Module | What it does |
|---|---|
| `util` | `/help`, `/info`, `/stickerid` |
| `misc` | `/ping`, `/mstats`, `/trongtruonghop` disclaimer |
| `wordle` | Daily Wordle game |
| `loldle` | League-of-Legends "guess the champion" |
| `lolschedule` | Pro-match schedule + daily push |
| `twentyq` | 20-questions game (requires Gemini API key) |
| `stock` | VN-stocks paper trading |
| `gold` | Gold paper trading (opt-in; primary VNAppMob SJC buy/sell VND/luong, fallback spot XAU) |
| `coin` | Crypto paper trading in USD (Binance -> Coinbase -> CoinGecko price fallback) |
| `stats` | `/stats` (top commands), `/stats users`, `/stats user <name>`, `/stats cmd <name>` |

Disable any module by editing `MODULES` in `template.yaml`.

## Layout

```
cmd/server/                  entrypoint (long polling + in-process cron + HTTP health)
cmd/migrate-dynamo-to-mongo/ one-off DynamoDB → MongoDB Atlas data migrator
internal/server/             HTTP routes (/ health, /cron/{name} manual trigger)
internal/telegram/           Telegram long-polling bot wrapper
internal/cron/               in-process cron scheduler (replaces EventBridge)
internal/modules/            Module framework, registry, dispatchers, modules
internal/storage/            typed DocStore[T] (Provider + Typed); mongodb runtime + memory (tests). Values persist as flattened native BSON root documents
internal/ai/                 Gemini client (used by twentyq)
docker-compose.yml           Coolify self-host stack (single bot service)
docs/deploy-coolify-selfhosted.md    Self-host onboarding + cutover runbook
docs/aws-decommission-runbook.md     AWS teardown (post-cutover)
template.yaml, aws/          Retired AWS SAM IaC + setup (kept for history)
```

## Run locally

In-memory KV (no database required):

```sh
TELEGRAM_BOT_TOKEN=… \
MODULES= \
go run ./cmd/server
```

The bot uses long polling, so a local run talks to Telegram directly — no `ngrok`, no public URL. Ensure the bot's webhook is unset (the server clears it on startup) or `getUpdates` 409s. The dev bot is created manually; token injected via env vars only.

Persistent MongoDB locally (auto-selected when `MONGO_URL` is set):

```sh
make mongo-local
TELEGRAM_BOT_TOKEN=… \
MONGO_URL=mongodb://127.0.0.1:27017 \
MONGO_DATABASE=miti99bot_dev \
go run ./cmd/server
```

For integration tests (each skips when its emulator env var is unset):
```sh
make mongo-local         # docker run mongo:7 on :27017
make test-mongo          # internal/storage typed-store tests against local MongoDB
make test-dynamodb       # DynamoDB→Mongo migrator e2e against DynamoDB Local + local Mongo
```

## Test

```sh
make vet              # go vet
make test             # full unit suite (no emulator)
make test-mongo       # typed-store integration tests against local Mongo (requires Docker)
make test-dynamodb    # migrator e2e against DynamoDB Local + Mongo (requires Docker)
```

## Deploy

**Self-host (current):** [`docs/deploy-coolify-selfhosted.md`](docs/deploy-coolify-selfhosted.md) — Coolify + MongoDB Atlas (free M0), long polling (no public ingress), in-process cron. Storage auto-selects `mongodb` when `MONGO_URL` is set; the cron scheduler runs by default. Migrate existing data with [`cmd/migrate-dynamo-to-mongo`](cmd/migrate-dynamo-to-mongo/README.md), then tear down AWS via [`docs/aws-decommission-runbook.md`](docs/aws-decommission-runbook.md).

**AWS (retired):** [`docs/deploy-aws.md`](docs/deploy-aws.md) — kept for history.

## License

[Apache-2.0](LICENSE).
