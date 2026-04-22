# Doantu Module

Vietnamese "đoán từ" (guess-the-word) — same core mechanic as `semantle`,
but targets come from a Vietnamese wordlist and similarity is computed
against ConceptNet's `/c/vi/<term>` concept URIs. Unlimited guesses per
round; solve on exact match.

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
with single spaces between them are accepted. Matching is case-insensitive
and diacritic-sensitive — `cá` and `ca` are different targets.

## Data source

**Target pool:** [duyet/vietnamese-wordlist](https://github.com/duyet/vietnamese-wordlist)'s
Viet22K list (~22k entries), sorted alphabetically. The raw list is
normalized (lowercase + deduped) but otherwise used verbatim — ConceptNet's
verify-and-fallback at round start rejects any pick that has no concept
edges. License: GPL-2.0 (Ho Ngoc Duc).

**Similarity + vocabulary:** [ConceptNet 5](https://conceptnet.io).
Multi-word Vietnamese terms are converted to underscore form (`con chó`
→ `/c/vi/con_chó`) only when building URIs — the board keeps the
space-separated display.

## Architecture

- `api-client.js` — ConceptNet wrapper (`randomWord`, `similarity`, and
  lower-level `concept` / `relatedness`). Hardcoded `LANG = "vi"`.
- `wordlist.js` — three-function API (`LINE_COUNT`, `randomLine`, `getLine`)
  over `words-data.js`.
- `words-data.js` — auto-generated (regenerate via `npm run build:doantu-words`).
- `state.js` — KV persistence for game + stats. Same shape as semantle.
- `lookup.js` — guess normalization + shape validation. Accepts Unicode
  letters + combining marks + single internal spaces.
- `format.js` — warmth-percent and emoji-bucket formatters (unchanged).
- `render.js` — Telegram HTML `<pre>` monospace board with a 🇻🇳 header.
- `handlers.js` — subject resolution + the three command entry points.

Near-clone of the semantle sibling — kept separate per the repo's
existing one-module-per-game convention rather than factoring out a
shared base. Diff your changes against `../semantle/` when fixing bugs
that apply to both.

## Storage

KV namespace prefix: `doantu:`

| Key | Value |
|-----|-------|
| `game:<subject>` | `{ target, startedAt, solved, guesses[] }` — active round (TTL 7 days). |
| `stats:<subject>` | `{ played, solved, totalGuesses, bestGuessCount, lastResultAt }` |

## Config

No env vars. ConceptNet base (`https://api.conceptnet.io`) is hardcoded;
pass an override to `createClient(url)` if you need a mirror or test double.

## Credits

- Wordlist: [duyet/vietnamese-wordlist](https://github.com/duyet/vietnamese-wordlist) by Ho Ngoc Duc (GPL-2.0).
- Similarity + vocabulary: [ConceptNet 5](https://conceptnet.io) by Robyn Speer et al.
- Game concept: [Semantle](https://semantle.com/) by David Turner.
