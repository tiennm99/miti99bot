# Twentyq Module

A reverse-Akinator yes/no guessing game. The bot picks a secret object from a
hand-curated seed list, gives an opening hint, then judges every user input
with a Workers AI LLM (`@cf/google/gemma-4-26b-a4b-it`) via function calling.
Each turn the model returns `{ is_guess, answer, hint }`. Round ends on a
correct guess (`is it an organ?` matches secret) or `/twentyq_giveup`.

**Visibility: `public`** — commands appear in both `/help` and Telegram's
native `/` autocomplete menu.

## Commands

| Command | Visibility | Description |
|---------|-----------|-------------|
| `/twentyq` | public | Show current board, or auto-start a round if none |
| `/twentyq <question>` | public | Submit a yes/no question OR a final guess (`is it ...?`) |
| `/twentyq_giveup` | public | Reveal the answer and end the round (next `/twentyq` starts fresh) |
| `/twentyq_stats` | public | Show per-subject stats |

## Example flow

```
/twentyq
🎯 I'm thinking of an instrument.
Hint: it uses wind through pipes to create sound.

/twentyq does it require hands to play?
✅ Yes. Hint: most players use both hands at once.

/twentyq is it made of wood?
❌ No. Hint: its body is mostly metal pipes.

/twentyq is it an organ?
🎉 Correct! It was an organ. Solved in 3 questions.
```

## Rules at a glance

- Yes/no questions only — open-ended forms (`what`, `how`, `why`, ...) are
  rejected client-side and don't count toward the turn tally.
- Repeat questions are deduped — same exact text replies `🔁 already asked`
  and skips the AI call.
- Unlimited turns. End with a correct guess or `/twentyq_giveup`.

## Data source

- **Secret pool:** `seeds.js` — flat array of keyword strings (~50+ nouns).
  No hand-curated categories or hints — drop in any noun, the LLM figures out
  the right category and produces a cryptic opening clue at round-start.
- **Round-start + per-turn responses:** Workers AI binding `env.AI` calling
  `@cf/google/gemma-4-26b-a4b-it`. Prompts instruct the model to emit one-line
  JSON that `parseJudgementJson` extracts even if wrapped in prose/fences.
  - Round start: `generateRoundStart(env, target)` → `{category, initialHint}`
  - Each turn:  `judge(env, state, userInput)`    → `{is_guess, answer, hint}`

The Gemma 4 26B A4B model runs on the Workers Free plan (10k Neurons/day).
Pricing: $0.10/M input + $0.30/M output. Each round is one start call (~1
AI request) + one request per turn, well under the cap for normal play volume.

## Architecture

- `seeds.js` — `SEEDS` array + `getRandomSeed(rng)`. Targets are lowercased.
- `state.js` — KV persistence for game + stats. Subject = user id (DM) or
  chat id (group). 7-day TTL on the active round.
- `prompts.js` — `buildSystemPrompt(state)` injects secret + history;
  `ANSWER_FUNCTION_SCHEMA` declares the `submit_answer` tool.
- `validate-input.js` — pre-AI regex check; rejects open-ended starters,
  empty/oversized input. Saves Neurons.
- `ai-client.js` — wraps `env.AI.run`, parses both Cloudflare-traditional and
  OpenAI-style tool-call shapes, normalizes payload, redacts the secret from
  hints (defense-in-depth). `UpstreamError` wraps any failure.
- `render.js` — five Telegram-HTML formatters; all user-derived text
  HTML-escaped.
- `handlers.js` — three command entry points + subject resolver + repeat
  dedup + round lifecycle.

## Storage

KV namespace prefix: `twentyq:`

| Key | Value |
|-----|-------|
| `game:<subject>` | `{ category, target, initialHint, startedAt, solved, turns[] }` (TTL 7 days) |
| `stats:<subject>` | `{ played, solved, totalTurns, bestTurnCount, lastResultAt }` |

Each `turns[]` entry is `{ text, isGuess, answer, hint, ts }`.

## Config

| Env var | Default | Purpose |
|---------|---------|---------|
| `MODULES` | (must include `twentyq`) | Activate the module at deploy time. |

No additional env vars. The module reads `env.AI` (Workers AI binding,
declared in `wrangler.toml [ai]`).

## Credits

- Game concept: classic *20 Questions* / Akinator-reversed.
- LLM judging: Cloudflare Workers AI `@cf/google/gemma-4-26b-a4b-it`.
