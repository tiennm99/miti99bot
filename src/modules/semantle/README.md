# Semantle Module

Semantic-similarity guessing game. A secret word is picked from a local
curated pool and each guess is scored by cosine similarity between
embedding vectors produced by Cloudflare Workers AI. Unlimited guesses
per round — you play until you get the exact word (case-insensitive).

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

**Target + vocabulary:** `words-data.js` ships the full
[google-10000-english](https://github.com/first20hours/google-10000-english)
list (~9.9k entries), ordered by Google Ngram frequency, normalized to
lowercase and deduped but otherwise unfiltered. The **same list is both
the target pool and the vocabulary** — so every legal guess could itself
have been the answer, and OOV detection is an O(1) `Set.has()` with no
upstream round-trip. Regenerate with `node scripts/build-semantle-words.js`.

**Similarity:** `@cf/baai/bge-m3` multilingual text embeddings via the
`env.AI` binding. Each in-vocab guess runs one inference call batching
target + guess (1024-dim vectors) and the module scores them with local
cosine similarity. At 1075 Neurons per M input tokens (~0.002 N/guess
for short words), the Workers Free plan cap of 10k Neurons/day covers
~4.6M guesses/day. Same model as `doantu` so both share the binding.

**Score calibration:** BGE embeddings live in a narrow cone, so raw
cosine for unrelated words already clusters at ~0.40–0.55 — reading as
misleadingly "warm". `format.js` applies a normalized sigmoid (FLOOR
0.40, CENTER 0.60, SCALE 8) to remap raw cosine → displayed 0-100.
Resulting curve: raw 0.40 → 0, 0.50 → 18, 0.60 → 42, 0.70 → 66,
0.80 → 84, 0.90 → 94, 1.00 → 100. Retune those three constants if you
swap models.

OOV guesses short-circuit before inference — the player sees
"isn't in the vocabulary" instead of a noisy subword-based score.

## Architecture

- `api-client.js` — Workers AI wrapper: `randomWord()` picks from the
  local pool, `similarity(a, b)` runs `env.AI.run()` and returns
  `{ in_vocab_b, similarity }` along with canonical forms.
  `UpstreamError` carries status/body metadata when inference fails.
- `words-data.js` — auto-generated dictionary (~9.9k entries).
- `wordlist.js` — one-function module exposing `randomLine()`.
- `state.js` — KV persistence for game + stats. Target stored lowercased.
- `lookup.js` — guess normalization (`trim + lowercase + collapse spaces`)
  and shape validation (`/^[a-z]+$/`, max 64 chars).
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

Each `guesses[]` entry is `{ word, canonical, similarity }`.

## Config

No env vars. Model defaults to `@cf/baai/bge-m3`; override with
`createClient(env.AI, { model: "@cf/baai/bge-small-en-v1.5" })` in a test
or alternative deploy.

## Why unlimited guesses?

Classic Semantle offers up to 100s of guesses per day, and the fun is in
the hunt — not the timer. Rounds stay open (TTL 7 days on KV) and skill is
tracked via `bestGuessCount` — fewest guesses to solve across all rounds.

## Credits

- Embeddings: [`@cf/baai/bge-m3`](https://developers.cloudflare.com/workers-ai/models/bge-m3/) on Cloudflare Workers AI (multilingual).
- Target dictionary: [google-10000-english](https://github.com/first20hours/google-10000-english) by Josh Kaufman, derived from Peter Norvig's Google Ngram analysis.
- Game concept: [Semantle](https://semantle.com/) by David Turner.
