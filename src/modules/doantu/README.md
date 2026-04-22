# Doantu Module

Vietnamese "đoán từ" (guess-the-word) — same core mechanic as `semantle`,
but targets come from a Vietnamese wordlist and similarity is computed
with a multilingual embedding model. Unlimited guesses per round; solve
on exact match (case-insensitive, diacritic-sensitive).

**Visibility: `protected`** — commands appear in `/help` but are hidden
from Telegram's native `/` autocomplete menu while the module is still
experimental.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/doantu` | protected | Show current board or submit a word guess |
| `/doantu_giveup` | protected | Reveal the answer and end the round (next `/doantu` starts a fresh one) |
| `/doantu_stats` | protected | Show per-subject stats |

Submit with `/doantu <word>` (e.g. `/doantu con chó`). Multi-syllable words
with single spaces between them are accepted. `cá` and `ca` are different
targets.

## Data source

**Target + vocabulary:** [duyet/vietnamese-wordlist](https://github.com/duyet/vietnamese-wordlist)'s
Viet22K list (~22k entries), lowercased and deduped. The same list is
both the target pool and the vocabulary — OOV detection is `Set.has()`
with no upstream call. License: GPL-2.0 (Ho Ngoc Duc).
Regenerate with `node scripts/build-doantu-words.js`.

**Similarity:** `@cf/baai/bge-m3` multilingual text embeddings via the
`env.AI` binding. Chosen over the English-only `bge-small-en-v1.5`
because that model's tokenizer shreds Vietnamese diacritics into noisy
byte-level subwords. Each in-vocab guess runs one inference call
batching target + guess (1024-dim vectors); the module scores them with
local cosine similarity.

## Architecture

- `api-client.js` — Workers AI wrapper: `randomWord()` picks from the
  local pool, `similarity(a, b)` calls `env.AI.run()` and returns
  `{ in_vocab_b, similarity }`. `UpstreamError` on inference failure.
- `words-data.js` — auto-generated Viet22K dictionary.
- `wordlist.js` — one-function module exposing `randomLine()`.
- `state.js` — KV persistence for game + stats. Same shape as semantle.
- `lookup.js` — guess normalization + shape validation. Accepts Unicode
  letters + combining marks + single internal spaces.
- `format.js` — warmth-percent and emoji-bucket formatters (identical
  to semantle/format.js — score display is language-agnostic).
- `render.js` — Telegram HTML `<pre>` monospace board with a 🇻🇳 header.
- `handlers.js` — subject resolution + the three command entry points.

Near-clone of the semantle sibling — kept separate per the repo's
one-module-per-game convention rather than factoring out a shared base.
Diff your changes against `../semantle/` when fixing bugs that apply to
both.

## Storage

KV namespace prefix: `doantu:`

| Key | Value |
|-----|-------|
| `game:<subject>` | `{ target, startedAt, solved, guesses[] }` — active round (TTL 7 days). |
| `stats:<subject>` | `{ played, solved, totalGuesses, bestGuessCount, lastResultAt }` |

## Config

No env vars. Model defaults to `@cf/baai/bge-m3`; override with
`createClient(env.AI, { model: "..." })` in a test or alternative deploy.

## Credits

- Embeddings: [`@cf/baai/bge-m3`](https://developers.cloudflare.com/workers-ai/models/bge-m3/) on Cloudflare Workers AI (multilingual).
- Wordlist: [duyet/vietnamese-wordlist](https://github.com/duyet/vietnamese-wordlist) by Ho Ngoc Duc (GPL-2.0).
- Game concept: [Semantle](https://semantle.com/) by David Turner.
