# Semantle Module

Semantic-similarity guessing game. A secret word is picked from a local
curated pool and validated against ConceptNet; each guess is scored by
ConceptNet's relatedness API against the target. Unlimited guesses per
round — you play until you get the exact word (case-insensitive).

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/semantle` | public | Show current board or submit a word guess |
| `/semantle_giveup` | public | Reveal the answer and end the round (next `/semantle` starts a fresh one) |
| `/semantle_stats` | public | Show wins / best count / averages |

Submit with `/semantle <word>` (e.g. `/semantle ocean`). Matching is
case-insensitive. Out-of-vocabulary words don't count toward the guess tally.
Repeating a prior guess replies with a `🔁 already guessed` notice and is
ignored (no cost, no stat inflation).

## Data source

**[ConceptNet 5](https://api.conceptnet.io/)** — free public API, no auth,
~300k English concepts including multi-word phrases. Two endpoints:

- `GET /relatedness?node1=/c/en/X&node2=/c/en/Y` — per-guess similarity,
  returns `{ value: number ∈ [-1, 1] }`.
- `GET /c/en/{term}` — vocabulary check: term is in vocab iff the response
  carries at least one edge.

Because ConceptNet has no random-word endpoint, the target pool ships in
`words-data.js` — ~8k common English words (4–10 ASCII letters) derived
from Google Ngram frequency via the public
[google-10000-english](https://github.com/first20hours/google-10000-english)
list, with the top-200 most frequent function words stripped.
Each new round picks locally, verifies via the concept endpoint, and falls
back to an unverified pick after a few misses.

Regenerate the list with `npm run build:semantle-words` (chained into the
main `npm run build` that `npm run deploy` invokes).

Every guess costs **two** ConceptNet calls (concept edges + relatedness)
issued in parallel. Typical latency ~300–600ms round-trip from Cloudflare
Workers; `api-client.js` enforces a 5s timeout and surfaces a "Upstream
hiccup" message on failure.

## Architecture

- `api-client.js` — ConceptNet HTTP wrapper (`randomWord`, `similarity`,
  plus lower-level `concept` / `relatedness`) with `UpstreamError` metadata.
  Preserves the earlier word2sim response shape so the rest of the module
  didn't need rewriting.
- `words-data.js` — auto-generated dictionary (regenerate via `npm run build:semantle-words`).
- `wordlist.js` — thin wrapper exposing `TARGET_POOL` and `pickFromPool()` over the dictionary.
- `state.js` — KV persistence for game + stats. Target stored lowercased.
- `lookup.js` — guess normalization and shape validation.
- `format.js` — warmth-percent and emoji-bucket formatters.
- `render.js` — Telegram HTML `<pre>` monospace board, sorted by similarity
  desc, capped at top 15 rows to stay under Telegram's message-length limit.
- `handlers.js` — subject resolution (user in DMs, chat in groups) + the
  three command entry points.

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

No env vars. ConceptNet's public API base (`https://api.conceptnet.io`) is
hardcoded in `api-client.js`; pass an override to `createClient(url)` if you
need to point at a mirror or test double.

## Why unlimited guesses?

Classic Semantle offers up to 100s of guesses per day, and the fun is in
the hunt — not the timer. We keep rounds open indefinitely (TTL 7 days on
KV) and measure skill via `bestGuessCount`, the fewest guesses to solve
across all rounds.

## Credits

- Similarity + vocabulary: [ConceptNet 5](https://conceptnet.io) by Robyn Speer et al.
- Target dictionary: [google-10000-english](https://github.com/first20hours/google-10000-english) by Josh Kaufman, derived from Peter Norvig's Google Ngram analysis.
- Game concept: [Semantle](https://semantle.com/) by David Turner.
