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
| `stats` | `/stats` (top commands), `/stats users`, `/stats user <username>`, `/stats cmd <command_name>` |

Disable modules with the `MODULES` environment variable.

## Command discovery

Public command registrations share their description plus optional `Parameters`
metadata between Telegram's native `/` menu and the bot's `/help` response.
Telegram renders the `/command` separately, so `/stock_buy` uses this native
description:

```text
<quantity> <ticker>. Buy VN stock at market price.
```

`/help` combines the full command syntax and summary on one line. Neither
discovery surface includes example invocations.

Future commands must follow the
[command parameter conventions](docs/command-parameter-conventions.md). Keep
command metadata, handler usage text, tests, and documentation aligned.

### Stock dividend commands

Stock dividends are manual portfolio adjustments:

- `/stock_cash_dividend <vnd_per_share> <ticker>` credits a positive whole-VND amount for each pre-event share held. Eg: `/stock_cash_dividend 1500 TCB`.
- `/stock_share_dividend <ratio(owned:new)> <ticker>` adds `floor(pre_event_shares × new / owned)` whole shares. Eg: `/stock_share_dividend 100:10 TCB`.
- `/stock_dividend <vnd_per_share> <ratio(owned:new)> <ticker>` applies both parts from the same pre-event holding and saves them together. Eg: `/stock_dividend 1500 100:10 TCB`.

Ratios use `owned:new` exactly as written in the issuer notice. Equivalent
unreduced ratios are accepted and the entered ratio is preserved in the reply.
The bot validates syntax, tickers, and arithmetic safety, but does not look up
notices or prevent duplicate calls. The caller is responsible for verifying the
notice and avoiding accidental repeated adjustments.

### Stock and coin P&L accounting

Stock and coin portfolios embed each open position under `assets.<symbol>`.
Both store `quantity` and total remaining `base`; stock positions additionally
store `dividendCheckedAt`. Stock cash is stored directly as `vnd`; coin cash
remains `usd`. Buys add their actual spend. Partial sells remove basis using the
weighted-average method and report realized P&L; full sells remove the position
and its basis. Stock share dividends add shares without adding cost, which
lowers the derived average price, while cash dividends do not change position
basis.

`/stock_portfolio` and `/coin_portfolio` show aligned monospace tables with
average entry price and unrealized P&L for each priced position. `Account P&L` remains the broader
account value minus all top-ups, so it also reflects realized proceeds,
dividend cash, and idle cash. If any current quote is unavailable, totals are
marked partial and numeric Account P&L is withheld.

For stock positions, `dividendCheckedAt` is a future dividend-event cursor. It
is initialized when a position is first bought, preserved across later buys and
sells, and advanced when a stock dividend is recorded. A full exit removes the
cursor; reopening the position starts it again. Coin positions do not store a
dividend cursor.

## Layout

```
cmd/server/                  entrypoint (long polling + in-process cron + HTTP health)
internal/server/             HTTP route (/ health only; cron has no HTTP route)
internal/telegram/           Telegram long-polling bot wrapper
internal/cron/               in-process cron scheduler
internal/modules/            Module framework, registry, dispatchers, modules
internal/storage/            typed DocStore[T] (Provider + Typed); mongodb runtime + memory (tests). Values persist as flattened native BSON root documents
internal/systemstate/        shared `system` collection helper for startup migration records
compose.yml                  Coolify self-host stack (single bot service)
docs/deploy-coolify-selfhosted.md    Self-host deploy and operations guide
```

## Run locally

In-memory storage requires no database. Set the environment variables for your
shell, then run the server with Go:

```powershell
# PowerShell
$env:TELEGRAM_BOT_TOKEN = "…"
$env:MODULES = ""
go run ./cmd/server
```

```sh
# POSIX shells (Linux/macOS)
export TELEGRAM_BOT_TOKEN="…"
export MODULES=""
go run ./cmd/server
```

The bot uses long polling, so a local run talks to Telegram directly — no
`ngrok` or public URL. The server clears any existing webhook on startup. The
dev bot is created manually; its token is injected through the environment.

Persistent MongoDB locally (auto-selected when `MONGO_URL` is set):

```sh
docker run -d --rm --name miti99bot-mongo -p 27017:27017 mongo:8
```

Then set `MONGO_URL=mongodb://127.0.0.1:27017` and
`MONGO_DATABASE=miti99bot_dev` using the shell syntax above before running
`go run ./cmd/server`. Stop the local database with
`docker stop miti99bot-mongo`.

MongoDB integration tests use Testcontainers to start MongoDB 8 automatically.
Keep Docker Desktop or another compatible Docker daemon running, then use the
normal Go test command. `MONGODB_TEST_URL` remains available as an optional
override when testing against an already-running MongoDB instance. If Docker is
unavailable and no override is set, MongoDB tests skip with an explicit warning;
use `go test -v ./...` to see individual skip reasons.

## Test

```sh
go vet ./...
go test -count=1 ./...
go build ./...
```

CI additionally runs the test suite with Go's race detector.

## Deploy

[`docs/deploy-coolify-selfhosted.md`](docs/deploy-coolify-selfhosted.md) covers
Coolify + MongoDB Atlas (free M0), long polling (no public ingress), and
in-process cron. Storage auto-selects `mongodb` when `MONGO_URL` is set; the
cron scheduler runs by default.

## License

[Apache-2.0](LICENSE).
