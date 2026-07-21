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
and `Example` metadata between Telegram's native `/` menu and the bot's `/help`
response. Telegram renders the `/command` separately, and its native description
field supports only single-line plain text—no copyable code block. For
`/stock_buy`, the bot therefore sends this description:

```text
<quantity> <ticker>. Buy VN stock at market price. Example: /stock_buy 100 TCB
```

`/help` combines the command and parameter syntax, adds the same summary, then
puts the example on the next line in a copyable code block. When `Example` is
omitted, both surfaces use the bare command (for example, `/ping`).

For future commands, use lowercase descriptive parameter names. Include units
or currencies when they affect meaning (`<vnd_amount>`, `<usd_to_spend>`), use
square brackets for optional input (`[date]`), append `...` when an argument
accepts remaining text (`[target...]`), and describe structured input in
parentheses (`<ratio(owned:new)>`, `<options(comma-separated)>`). Keep command
metadata, handler usage text, examples, tests, and this documentation aligned.

### Stock dividend commands

Stock dividends are manual portfolio adjustments:

- `/stock_cash_dividend <vnd_per_share> <ticker>` credits a positive whole-VND amount for each pre-event share held. Example: `/stock_cash_dividend 1500 TCB`.
- `/stock_share_dividend <ratio(owned:new)> <ticker>` adds `floor(pre_event_shares × new / owned)` whole shares. Example: `/stock_share_dividend 100:10 TCB`.
- `/stock_dividend <vnd_per_share> <ratio(owned:new)> <ticker>` applies both parts from the same pre-event holding and saves them together. Example: `/stock_dividend 1500 100:10 TCB`.

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
