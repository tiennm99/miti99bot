# miti99bot

Plug-n-play Telegram bot framework in Go. Self-hosted on Coolify + MongoDB
Atlas via long polling and an in-process cron scheduler.

## Modules

| Module | What it does |
|---|---|
| `util` | `/help`, `/info`, `/stickerid` |
| `misc` | `/ping`, `/ping_stats`, `/random`, `/wheelofnames`, `/ff`, `/xlt1`, `/the_answer`, `/trongtruonghop` + `/tth`, `/trongtruonghopvng` + `/tthvng` disclaimers |
| `amlich` | Vietnamese lunar calendar: `/amlich` (dương lịch → âm lịch, defaults to today), `/duonglich` (âm lịch → dương lịch, `nhuan` flag for leap months); dates accept `d`, `d/m`, or `d/m/yyyy` — missing parts fill from today in the input's calendar. Years 1800–2199 only |
| `wordle` | Daily Wordle game |
| `loldle` | League-of-Legends "guess the champion" |
| `lol` | Pro-match schedule (`/lol`, `/lol_tomorrow`, `/lol_this_week`, `/lol_next_week`), per-chat digest opt-in (`/lol_subscribe`, `/lol_unsubscribe`) + daily push at 08:00 ICT |
| `stock` | VN-stocks paper trading |
| `gold` | Gold paper trading (VNAppMob SJC buy/sell VND/luong) |
| `coin` | Crypto paper trading in USD (Binance -> Coinbase -> CoinGecko price fallback) |
| `stats` | `/stats` (top commands), `/stats users`, `/stats user <username>`, `/stats cmd <command_name>` |
| `sticker` | `/addsticker` — append a replied sticker, image, video or GIF to one shared pack. See [docs/sticker-packs.md](docs/sticker-packs.md) |
| `alias` | `/alias <name>` save a replied message under a name, then send it back with `/insert <name>`, bare `/<name>`, or inline `@botname <prefix>`; `/aliases` lists, `/unalias` deletes. See [docs/aliases.md](docs/aliases.md) |
| `monkeyd` | `/monkeyd_crawl <url> [font_size]` export a monkeydd.com novel as a PDF, `/monkeyd_tags <url>` list its tags as hashtags |

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

### Lunar calendar accuracy

`/amlich` and `/duonglich` port Hồ Ngọc Đức's truncated-Meeus algorithm, the
same one behind most Vietnamese calendar apps, over the years 1800–2199.
Output matches published Vietnamese calendars on every verifiable date. It
diverges from Chinese-calendar sources in some years by design (Vietnam uses
UTC+7, China UTC+8), and month boundaries from 2072 on carry irreducible
uncertainty. See
[amlich known issues](docs/amlich-known-issues.md) for the decision record and
the full edge-case list.

### Stock corporate events

`/stock_events <ticker> [days]` lists SSI iBoard corporate actions for a VN
stock without reading or changing a portfolio. The lookback defaults to 30
days; `days` must be a whole number from 1 to 90. Results are returned in
chronological order, split into Telegram-safe chunks when needed, and show the
raw SSI corporate-action details. The feature is best-effort because SSI's API
is undocumented.

### Stock quote details

`/stock_info <ticker>` shows a compact SSI iBoard quote snapshot: company,
exchange, current price, gain/loss since open, change versus the reference
price, open/high/low prices, and normal traded volume. It makes exactly one
SSI single-ticker request and does not use the KBS or VCI price fallbacks.
Unavailable optional fields are shown as `N/A`. This read-only command is
best-effort because SSI's API is undocumented. The existing `/stock_price`
command and its provider fallbacks are unchanged.

### Stock dividend commands

Stock dividends are manual portfolio adjustments:

- `/stock_cash_dividend <vnd_per_share> <ticker>` credits a positive whole-VND amount for each pre-event share held and lowers the position's cost basis by the total payout. Eg: `/stock_cash_dividend 1500 TCB`.
- `/stock_share_dividend <ratio(owned:new)> <ticker>` adds `floor(pre_event_shares × new / owned)` whole shares. Eg: `/stock_share_dividend 100:10 TCB`.

The combined `/stock_dividend` shortcut was retired. Use the specialized cash
and share commands above for new adjustments.

Ratios use `owned:new` exactly as written in the issuer notice. Equivalent
unreduced ratios are accepted and the entered ratio is preserved in the reply.
The bot validates syntax, tickers, and arithmetic safety. `/stock_portfolio`
also checks SSI iBoard for cash and explicit share-dividend events published in
the preceding 30 days. The portfolio is always sent first. Every retained
unprocessed event is re-sent on each `/stock_portfolio` until it is processed
or expires after 90 days: future events are informational messages without a
button, while events from Record date include an `Apply dividend` button.
Events with no Record date remain informational while the bot rechecks their
original publication window for SSI updates.

Suggestions expire after 24 hours and are bound to the Telegram user who
requested the portfolio, the originating chat, and the event message. Another
group member cannot apply them. Acceptance calculates from the user's current
holding at click time and marks the per-user event processed atomically with the
portfolio change. The current position must have opened on or before Record
date. SSI iBoard is an undocumented, best-effort source; failures do not prevent
the portfolio from being shown. The bot does not persist dated lots, so
suggestions are not legal record-date entitlement calculations. Users should
verify the issuer notice.

Repeated portfolio requests can create multiple valid buttons for the same
event. Processing is idempotent: the first accepted button marks the event
processed, and later buttons cannot credit it again.

Normalized SSI history is retained under
`dividends.<ticker>.<ssi_event_id>`, separate from active assets so a full sale
does not permit the same event to be applied after a repurchase. Records are
removed 90 days after Record date. If SSI never supplies Record date, they are
removed 90 days after publication. A later SSI response that omits an event
does not remove or suppress the retained per-user record.

The manual commands remain available, but they do not carry an SSI event ID.
Applying an event manually and then accepting its button can therefore record
the same dividend twice; use one method for a given event.

### Stock and coin P&L accounting

Stock and coin portfolios embed each open position under `assets.<symbol>`.
Both store `quantity` and total remaining `base`; stock positions additionally
store an `openedAt` lifecycle marker. Stock cash is
stored directly as `vnd`; coin cash remains `usd`. Buys add their actual spend.
Partial sells remove basis using the weighted-average method and report realized
P&L; full sells remove the position and its basis. Stock share dividends add
shares without adding cost, which lowers the derived average price. Cash
dividends credit the balance and reduce the position basis by the payout
(floored at zero, never negative) as a return of capital, so the ticker's
unrealized P&L includes dividends already received.

`/stock_portfolio` and `/coin_portfolio` show compact aligned monospace tables
with separate unrealized P&L amount and percentage columns for each priced
position. Stock `Avg` and `Now` use thousand VND as their implicit unit; coin
position amounts use implicit USD without a `$` prefix. `Account P&L` remains the broader
account value minus all top-ups, so it also reflects realized proceeds,
dividend cash, and idle cash. If any current quote is unavailable, totals are
marked partial and numeric Account P&L is withheld.

Stock dividend discovery has no per-position cursor. SSI queries use a rolling
30-day publication window and overlap the previous Asia/Saigon calendar day at
the provider boundary; caller-side filtering restores the exact interval.
Per-user event history provides notification state and processing idempotency.
The stock-only `assets.<ticker>.openedAt` marker identifies the current position
lifecycle, invalidates buttons after a full sale and later repurchase, and
prevents a position opened after Record date from applying an older event.

### Novel PDF export

`/monkeyd_crawl <url> [font_size]` downloads every chapter of a monkeydd.com
novel and sends it back as a single PDF document, sized for reading on a phone.
The crawling and rendering come from the
[monkeyd-crawler](https://github.com/tiennm99/monkeyd-crawler) submodule; the
module is the Telegram surface around it.

`font_size` is the body text size in points and accepts half points. It ranges
from 6 to 24 and defaults to the crawler's own default of 10, which fits roughly
43 characters per line across 26 lines on the 90×160 mm page. Larger values
trade characters per line for legibility: 12 gives about 36. Headings and the
title page scale with it. Omitting the argument passes no size at all, so the
crawler's default applies rather than a second one defined here. The document
caption reports the size that was used.

The command is public, so any member of a chat can run it. One invocation makes
hundreds of outbound requests spread over several minutes, so two things bound
the cost and are deliberate: only `monkeydd.com` URLs are accepted, and exactly
one export runs at a time across the whole bot. A missing scheme is filled in,
so a pasted bare hostname works.

The host allowlist is not only about the parser — the extractor is written
against that site's markup and would find nothing elsewhere — it also stops the
command being used to make the bot fetch arbitrary URLs.

The bot replies immediately that the export started, then sends the PDF when it
is ready; a second request while one is in flight is told which novel is
currently running. Requests are spaced out by the crawler, so a run is
deliberately slow rather than aggressive.

Raw pages are cached under the system temp directory, so re-exporting the same
novel costs no requests. The cache is not pruned and a container restart clears
it. Finished PDFs are deleted after upload. Telegram caps bot uploads at 50 MB;
a larger book is reported instead of being sent.

The PDF embeds a font covering Vietnamese diacritics. The crawler prefers a
system font and falls back to one compiled into the binary, so the runtime
image needs no fonts installed.

### Novel tags

`/monkeyd_tags <url>` reports a novel's tags as a hashtag line, sent as a code
block so it can be copied in one tap:

```text
#MonkeyD #CổĐại #GiaĐình

https://monkeydd.com/truong-an-gwem.html
```

Each label becomes one hashtag with spaces and punctuation removed and every
word capitalised, because Telegram ends a hashtag at the first character that is
not a letter, digit, or underscore. Diacritics are kept. A label with no letters
is dropped rather than emitted as a bare `#`.

The tags are the novel's own genres, identified by their `itemprop="genre"`
microdata. The same page also links every genre on the site as navigation — 69
of them against one novel's 6 in a sampled page — so matching the category URL
shape instead would return the whole menu.

This command costs a single request and runs inline rather than in the
background, bounded by a short timeout: handlers are dispatched one at a time, so
a stalled fetch would hold up every other command. It shares the export page
cache, so a lookup for a novel that was already exported costs no request at
all.

## Layout

```
cmd/server/                  entrypoint (long polling + in-process cron + HTTP health)
internal/server/             HTTP route (/ health only; cron has no HTTP route)
internal/telegram/           Telegram long-polling bot wrapper
internal/cron/               in-process cron scheduler
internal/modules/            Module framework, registry, dispatchers, modules
internal/storage/            typed DocStore[T] (Provider + Typed); mongodb runtime + memory (tests). Values persist as flattened native BSON root documents
internal/systemstate/        shared `system` collection helper for startup migration records
third_party/monkeyd-crawler/ git submodule; resolved by a go.mod replace directive
compose.yml                  Coolify self-host stack (single bot service)
docs/deploy-coolify-selfhosted.md    Self-host deploy and operations guide
docs/command-parameter-conventions.md  Command parameter syntax rules
docs/amlich-known-issues.md  Lunar algorithm decision and known edge cases
```

## Run locally

Clone with submodules — the `monkeyd` module builds against
`third_party/monkeyd-crawler`, and Go resolves it through a `replace` directive
pointing at that directory:

```sh
git clone --recurse-submodules https://github.com/tiennm99/miti99bot.git

# already cloned without them:
git submodule update --init --recursive
```

Without the submodule checked out, every Go command fails to resolve
`github.com/tiennm99/monkeyd-crawler`.

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

The `lol` module reads its schedule from the PandaScore REST API and needs
`LOL_PANDASCORE_TOKEN` (free tier). Without it every `/lol*` fetch fails while
the rest of the bot runs normally. See [`.env.example`](.env.example) for the
full variable list.

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
