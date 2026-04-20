# Wordle Module

Classic 5-letter word guessing game — player has 6 guesses; each letter is
marked 🟩 correct / 🟨 partial / ⬜ wrong with the standard duplicate-letter
rules.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/wordle` | public | Show current board / start game / submit a guess when an argument is given |
| `/wordle_new` | public | Start a new round (auto-gives-up any in-progress one) |
| `/wordle_giveup` | public | Reveal the current wordle answer |
| `/wordle_stats` | public | Show your wordle stats (wins, streak) |

Submit a guess with `/wordle <word>` — e.g. `/wordle crane`.

## Architecture

Mirrors the loldle module layout:

- `compare.js` — two-pass letter comparison with duplicate-letter handling
- `lookup.js` — normalize + validate guesses against the dictionary Set
- `daily.js` — `pickRandom` / `pickDaily` (djb2 hash of date seed)
- `render.js` — 🟩🟨⬜ markers + upper-case letters, stacked board view
- `state.js` — KV persistence, `MAX_GUESSES = 6`, stats with streak tracking
- `handlers.js` — wires subject resolution (user in DM, chat in groups) to the pure functions
- `words-data.js` — auto-generated word list (do not edit by hand)

Subject resolution matches loldle: private chats track per-user games, groups
track per-chat shared games.

## Database

KV namespace prefix: `wordle:`

| Key | Type | Description | Example |
|-----|------|-------------|---------|
| `game:<subject>` | JSON | Active round: target word, guesses, solved/giveup flags | `{ "target": "crane", "guesses": [...], "solved": false }` |
| `stats:<subject>` | JSON | Aggregate stats per subject | `{ "played": 12, "wins": 8, "streak": 3, "bestStreak": 5 }` |

Active rounds expire after 7 days if untouched.

## Word list

The 14,855-word dictionary is sourced from
[dracos/dd0668f281e685bad51479e5acaadb93](https://gist.github.com/dracos/dd0668f281e685bad51479e5acaadb93)
— Anna Eilering's combined Wordle dictionary of allowed 5-letter guesses.
All credit for compiling the list goes to the gist author.

Regenerate the bundled list with:

```bash
node scripts/build-wordle-data.js
```

This writes `src/modules/wordle/words-data.js` as a sorted, deduplicated
ES-module export.
