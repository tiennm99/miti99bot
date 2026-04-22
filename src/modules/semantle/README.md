# Semantle Module

Word2vec similarity guessing game. A secret word is picked from our hosted
[word2sim](https://github.com/tiennm99/word2sim) instance; each guess is
scored by cosine similarity against the target. Unlimited guesses per round
— you play until you get the exact word (case-insensitive).

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/semantle` | public | Show current board or submit a word guess |
| `/semantle_new` | public | Abandon current round and start a fresh one |
| `/semantle_giveup` | public | Reveal the answer and end the round |
| `/semantle_stats` | public | Show wins / best count / averages |

Submit with `/semantle <word>` (e.g. `/semantle ocean`). Matching is
case-insensitive. Out-of-vocabulary words don't count toward the guess tally.

## Data source

Target words and similarity scores come from **[word2sim](https://word2sim.sg.miti99.com)**,
our hosted FastAPI service over the GoogleNews pretrained word2vec model
(3M tokens × 300 dims). Two endpoints are used:

- `GET /random` — round-start target pick, filtered to game-friendly words.
- `GET /similarity?a&b` — per-guess cosine similarity.

No local model — every guess is a network round-trip. Typical latency
~200–400ms; `api-client.js` enforces a 5s timeout and surfaces a
"Upstream hiccup" message on failure.

## Architecture

- `api-client.js` — word2sim HTTP wrapper (`randomWord`, `similarity`) plus
  `Word2SimError` with `{status, body, cause}` metadata.
- `state.js` — KV persistence for game + stats. Target stored lowercased.
- `lookup.js` — guess normalization and shape validation.
- `format.js` — warmth-percent and emoji-bucket formatters.
- `render.js` — Telegram HTML `<pre>` monospace board, sorted by similarity
  desc, capped at top 15 rows to stay under Telegram's message-length limit.
- `handlers.js` — subject resolution (user in DMs, chat in groups) + the
  four command entry points.

Subject resolution: private chats track per-user games; groups track
per-chat shared games. Mirrors `loldle`/`wordle`.

## Storage

KV namespace prefix: `semantle:`

| Key | Value |
|-----|-------|
| `game:<subject>` | `{ target, startedAt, solved, guesses[] }` — active round (TTL 7 days). `target` stored lowercased. |
| `stats:<subject>` | `{ played, solved, totalGuesses, bestGuessCount, lastResultAt }` |

Each `guesses[]` entry is `{ word, canonical, similarity }`. The canonical
form is lowercased on write so the solve check is a single string compare.

## Config

| Env var | Default | Meaning |
|---------|---------|---------|
| `WORD2SIM_API_URL` | `https://word2sim.sg.miti99.com` | word2sim base URL; override for local word2sim or self-hosted |

Set in `wrangler.toml` `[vars]`. For local `wrangler dev`, optionally add
to `.dev.vars` (gitignored).

## Why unlimited guesses?

Classic Semantle offers up to 100s of guesses per day, and the fun is in
the hunt — not the timer. We keep rounds open indefinitely (TTL 7 days on
KV) and measure skill via `bestGuessCount`, the fewest guesses to solve
across all rounds.

## Credits

- Embedding model: Google's pretrained word2vec (3M tokens, 300 dims, trained on Google News).
- Hosting layer: [tiennm99/word2sim](https://github.com/tiennm99/word2sim).
- Game concept: [Semantle](https://semantle.com/) by David Turner.
