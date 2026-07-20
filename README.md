# miti99bot

Plug-n-play Telegram bot framework in Go. Self-hosted on Coolify + MongoDB
Atlas via long polling and an in-process cron scheduler.

## Modules

| Module | What it does |
|---|---|
| `util` | `/help`, `/info`, `/stickerid` |
| `misc` | `/ping`, `/ping_stats`, `/random`, `/wheelofnames`, `/ff`, `/the_answer`, `/trongtruonghop` + `/tth`, `/trongtruonghopvng` + `/tthvng` disclaimers |
| `wordle` | Daily Wordle game |
| `loldle` | League-of-Legends "guess the champion" |
| `lol` | Pro-match schedule (`/lol`, `/lol_tomorrow`, `/lol_this_week`, `/lol_next_week`) + daily push |
| `stock` | VN-stocks paper trading |
| `gold` | Gold paper trading (opt-in; VNAppMob SJC buy/sell VND/luong) |
| `coin` | Crypto paper trading in USD (Binance -> Coinbase -> CoinGecko price fallback) |
| `stats` | `/stats` (top commands), `/stats users`, `/stats user <username>`, `/stats cmd <command>` |

Disable modules with the `MODULES` environment variable.

### Stock dividend commands

Stock dividends are manual portfolio adjustments:

- `/stock_cash_dividend <vnd_per_share> <TICKER>` credits a positive whole-VND amount for each pre-event share held. Example: `/stock_cash_dividend 1500 TCB`.
- `/stock_share_dividend <owned:new> <TICKER>` adds `floor(pre_event_shares × new / owned)` whole shares. Example: `/stock_share_dividend 100:10 TCB`.
- `/stock_dividend <vnd_per_share> <owned:new> <TICKER>` applies both parts from the same pre-event holding and saves them together. Example: `/stock_dividend 1500 100:10 TCB`.

Ratios use `owned:new` exactly as written in the issuer notice. Equivalent
unreduced ratios are accepted and the entered ratio is preserved in the reply.
The bot validates syntax, tickers, and arithmetic safety, but does not look up
notices or prevent duplicate calls. The caller is responsible for verifying the
notice and avoiding accidental repeated adjustments.

## Layout

```
cmd/server/                  entrypoint (long polling + in-process cron + HTTP health)
internal/server/             HTTP route (/ health only; cron has no HTTP route)
internal/telegram/           Telegram long-polling bot wrapper
internal/cron/               in-process cron scheduler
internal/modules/            Module framework, registry, dispatchers, modules
internal/storage/            typed DocStore[T] (Provider + Typed); mongodb runtime + memory (tests). Values persist as flattened native BSON root documents
internal/systemstate/        shared `system` collection helper for future startup migrations
compose.yml                  Coolify self-host stack (single bot service)
telegram-commands.json       Manual Telegram command menu source
docs/deploy-coolify-selfhosted.md    Self-host deploy and operations guide
```

## Run locally

In-memory storage (no database required):

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
make test-mongo          # MongoDB integration tests against local MongoDB
```

## Test

```sh
make vet              # go vet
make test             # full unit suite (no emulator)
make test-mongo       # MongoDB integration tests against local Mongo (requires Docker)
```

## Deploy

[`docs/deploy-coolify-selfhosted.md`](docs/deploy-coolify-selfhosted.md) covers
Coolify + MongoDB Atlas (free M0), long polling (no public ingress), and
in-process cron. Storage auto-selects `mongodb` when `MONGO_URL` is set; the
cron scheduler runs by default.

## License

[Apache-2.0](LICENSE).
