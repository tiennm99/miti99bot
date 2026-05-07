# Loldle Module

Classic-mode League of Legends champion guessing game. Players get 8 guesses
(default; per-subject override via the hidden `/loldle_setmax` command, capped
at 10) to identify a hidden champion; each guess is compared across 7
attributes and the board is rendered as a monospace Telegram table.

## Data source

All champion data comes from **[loldle.net](https://loldle.net/classic)**.
The site embeds its classic-mode dataset in plaintext inside its JS bundle,
so we scrape and store it verbatim — no transformation, no external merge.

`scripts/scrape-loldle-data.js` fetches `loldle.net/classic`, extracts the
hashed `js/index.<hash>.js` bundle URL, pulls every champion record via a
single regex, and writes the array to `src/modules/loldle/champions.json`.

The bot imports that JSON directly (`with { type: "json" }`). No build step,
no wrapper module.

**Regenerate manually:** `npm run scrape:loldle-data`
**Weekly refresh:** `.github/workflows/scrape-loldle-data.yml` (Mon 06:00 UTC)
opens a PR whenever the data changes.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/loldle` | public | Show current board, or submit a champion guess |
| `/loldle_giveup` | public | Reveal the answer and end the round |
| `/loldle_stats` | public | Show your wins / streak |
| `/loldle_setmax <n>` | private | Override max guesses (1-10) for this subject; applies to next round |

Submit a guess with `/loldle <champion>` (e.g. `/loldle Ahri`). Names match
case/space/punctuation-insensitive with a unique-prefix fallback. A round
that ends (solved, gave up, or out of guesses) is cleared; the next round
is created lazily on the first `/loldle` call, and the timer (`startedAt`)
only starts when the player submits their first actual guess — viewing an
empty board gives no hints, so it shouldn't count against the clock.

## Architecture

- `compare.js` — attribute comparison across 7 axes from loldle.net's raw
  schema (`gender`, `species`, `range_type`, `resource`, `regions`,
  `positions`, `release_date`). Returns `correct` / `partial` / `wrong` per
  row, plus an up/down `direction` hint for the year.
- `lookup.js` — normalizes user input to a champion record.
- `render.js` — Telegram HTML `<pre>` monospace table with auto-widthed
  label column.
- `state.js` — KV persistence (`MAX_GUESSES = 8` default, `MAX_GUESSES_CAP = 10`,
  per-subject stats and config override).
- `handlers.js` — subject resolution (user id in DMs, chat id in groups) +
  command flow.
- `flavor.js` — win-message text helpers.
- `stickers.js` — Telegram sticker pools per outcome.
- `champions.json` — auto-generated data (never edit by hand).

Subject resolution: private chats track per-user games; groups track
per-chat shared games.

## Storage

KV namespace prefix: `loldle:`

| Key | Value |
|-----|-------|
| `game:<subject>` | `{ target, guesses, startedAt }` — active round (TTL 7 days). `guesses` is a championName array; comparison rows are recomputed at render time. |
| `stats:<subject>` | `{ played, wins, streak, bestStreak }` |
| `config:<subject>` | `{ maxGuesses }` — optional per-subject round-length override (1-10). Absent ⇒ default `MAX_GUESSES`. |

## Credits

Champion data © Riot Games, via loldle.net.
