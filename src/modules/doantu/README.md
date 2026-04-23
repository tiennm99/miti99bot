# Doantu Module

Vietnamese "đoán từ" (guess-the-word) — same core mechanic as `semantle`,
but targets + similarity come from a Vietnamese-tuned embedding service.
Unlimited guesses per round; solve on exact match (case-insensitive,
diacritic-sensitive).

**Visibility: `public`** — commands appear in both `/help` and Telegram's
native `/` autocomplete menu.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/doantu` | public | Show current board or submit a word guess |
| `/doantu_giveup` | public | Reveal the answer and end the round (next `/doantu` starts a fresh one) |
| `/doantu_stats` | public | Show per-subject stats |

Submit with `/doantu <word>` (e.g. `/doantu con chó`). Multi-syllable words
with single spaces between them are accepted. `cá` and `ca` are different
targets. Out-of-vocabulary words don't count toward the guess tally.
Repeating a prior guess replies with a `🔁 already guessed` notice and is
ignored (no cost, no stat inflation).

## Data source

Target words + similarity scores come from our self-hosted **phow2sim**
instance (default: `https://phow2sim.sg.miti99.com`). Wraps two endpoints:

- `GET /random` — pick a secret Vietnamese word at round start. Targets
  are filtered to the top-frequency band (`min_rank=100`, `max_rank=1000`)
  so rounds stay guessable for casual players.
- `GET /similarity?a=…&b=…` — cosine similarity + canonical forms +
  `in_vocab_a` / `in_vocab_b` flags.

Override the base URL for local dev via `PHOW2SIM_API_URL`.

## Architecture

- `api-client.js` — thin `fetch` wrapper around `/random` and
  `/similarity`. 5 s timeout; `UpstreamError` carries HTTP status + body
  snippet on failure.
- `state.js` — KV persistence for game + stats. Same shape as semantle.
- `lookup.js` — guess normalization + shape validation. Accepts Unicode
  letters + combining marks + single internal spaces.
- `format.js` — warmth-percent and emoji-bucket formatters (identical
  to semantle/format.js — score display is language-agnostic).
- `render.js` — Telegram HTML `<pre>` monospace board with a 🇻🇳 header.
- `handlers.js` — subject resolution + the three command entry points.
  Fast-path dedup (exact text OR prior canonical) skips wasted API calls
  on repeat guesses; post-API dedup catches different inputs that
  canonicalize to the same token.

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

Each `guesses[]` entry is `{ word, canonical, similarity }`.

## Config

| Env var | Default | Purpose |
|---------|---------|---------|
| `PHOW2SIM_API_URL` | `https://phow2sim.sg.miti99.com` | Base URL for the phow2sim service. |

## Credits

- Similarity backend: self-hosted `phow2sim` (Vietnamese word2vec/PhoBERT-style).
- Game concept: [Semantle](https://semantle.com/) by David Turner.
