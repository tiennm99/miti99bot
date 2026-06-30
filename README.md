# miti99bot

Plug-n-play Telegram bot framework in Go. Self-hosted on Coolify + MongoDB
Atlas via long polling and an in-process cron scheduler.

## Modules

| Module | What it does |
|---|---|
| `util` | `/help`, `/info`, `/stickerid` |
| `misc` | `/ping`, `/mstats`, `/trongtruonghop` disclaimer |
| `wordle` | Daily Wordle game |
| `loldle` | League-of-Legends "guess the champion" |
| `lolschedule` | Pro-match schedule + daily push |
| `wc` | World Cup schedule + silent daily push |
| `stock` | VN-stocks paper trading |
| `gold` | Gold paper trading (opt-in; primary VNAppMob SJC buy/sell VND/luong, fallback spot XAU) |
| `coin` | Crypto paper trading in USD (Binance -> Coinbase -> CoinGecko price fallback) |
| `stats` | `/stats` (top commands), `/stats users`, `/stats user <name>`, `/stats cmd <name>` |

Disable modules with the `MODULES` environment variable.

## Layout

```
cmd/server/                  entrypoint (long polling + in-process cron + HTTP health)
internal/server/             HTTP route (/ health only; cron has no HTTP route)
internal/telegram/           Telegram long-polling bot wrapper
internal/cron/               in-process cron scheduler
internal/modules/            Module framework, registry, dispatchers, modules
internal/storage/            typed DocStore[T] (Provider + Typed); mongodb runtime + memory (tests). Values persist as flattened native BSON root documents
compose.yml                  Coolify self-host stack (single bot service)
telegram-commands.json       Manual Telegram command menu source
docs/deploy-coolify-selfhosted.md    Self-host deploy and operations guide
```

## Run locally

In-memory storage (no database required):

```sh
TELEGRAM_BOT_TOKEN=… \
WC_FOOTBALL_DATA_TOKEN=… \
MODULES= \
go run ./cmd/server
```

The bot uses long polling, so a local run talks to Telegram directly — no `ngrok`, no public URL. Ensure the bot's webhook is unset (the server clears it on startup) or `getUpdates` 409s. The dev bot is created manually; token injected via env vars only.

Persistent MongoDB locally (auto-selected when `MONGO_URL` is set):

```sh
make mongo-local
TELEGRAM_BOT_TOKEN=… \
WC_FOOTBALL_DATA_TOKEN=… \
MONGO_URL=mongodb://127.0.0.1:27017 \
MONGO_DATABASE=miti99bot_dev \
go run ./cmd/server
```

For integration tests (each skips when its emulator env var is unset):
```sh
make mongo-local         # docker run mongo:7 on :27017
make test-mongo          # internal/storage typed-store tests against local MongoDB
```

## Test

```sh
make vet              # go vet
make test             # full unit suite (no emulator)
make test-mongo       # typed-store integration tests against local Mongo (requires Docker)
```

## Deploy

[`docs/deploy-coolify-selfhosted.md`](docs/deploy-coolify-selfhosted.md) covers
Coolify + MongoDB Atlas (free M0), long polling (no public ingress), and
in-process cron. Storage auto-selects `mongodb` when `MONGO_URL` is set; the
cron scheduler runs by default.

## License

[Apache-2.0](LICENSE).
