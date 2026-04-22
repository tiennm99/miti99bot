# Loldle Module

Classic-mode League of Legends champion guessing game — ported from
[`tiennm99/loldle`](https://github.com/tiennm99/loldle) (`lib/classic-mode.js`).
Champion data is scraped weekly from [loldle.net](https://loldle.net/classic)'s
JS bundle into `champions.json` via `.github/workflows/scrape-loldle-data.yml`
(runs `node scripts/scrape-loldle-data.js`).

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/loldle` | public | Show current board, start a game, or submit a champion guess when an argument is provided |
| `/loldle_giveup` | public | Reveal the current loldle answer (auto-starts a fresh round) |
| `/loldle_stats` | public | Show your loldle stats (wins, streak) |

Submit a guess with `/loldle <champion>` — e.g. `/loldle Ahri`. Champion names
are matched case/space/punctuation-insensitive with a unique-prefix fallback
(see `lookup.js`). A round that ends (solved, gave up, or ran out of guesses)
immediately rolls into a fresh round — no manual "new round" command needed.

## Architecture

- `compare.js` — pure attribute comparison across 7 classic-mode attributes
  (gender, species, range, resource, region, lane, year). Returns `correct`,
  `partial`, or `wrong` per attribute, plus a `direction` hint for year.
- `lookup.js` — normalizes user input and resolves it to a champion record.
- `daily.js` — `pickRandom` / `pickDaily` (djb2-hashed date seed for future
  daily-mode use).
- `render.js` — Telegram HTML `<pre>` monospace table with auto-widthed label
  column (✅/🟨/❌ markers and ⬆️/⬇️ year direction hints).
- `state.js` — KV persistence with `MAX_GUESSES = 8`, per-subject stats with
  streak tracking.
- `handlers.js` — wires subject resolution (user id in DMs, chat id in groups)
  to the pure functions above.
- `champions.json` — auto-generated from loldle.net (do not edit by hand;
  regenerate with `npm run scrape:loldle-data`). Imported directly via
  `with { type: "json" }`.

Subject resolution: private chats track per-user games; groups track per-chat
shared games (everyone plays the same round).

## Database

KV namespace prefix: `loldle:`

| Key | Type | Description |
|-----|------|-------------|
| `game:<subject>` | JSON | Active round: target champion id, guesses, solved/giveup flags, startedAt |
| `stats:<subject>` | JSON | Aggregate stats: played, wins, streak, bestStreak, lastResultAt |

Active rounds expire after 7 days if untouched.

## Credits

Champion data © Riot Games. The comparison attribute definitions and scoring
rules are ported from the original `tiennm99/loldle` project.
