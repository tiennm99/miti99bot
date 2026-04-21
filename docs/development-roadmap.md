# Development Roadmap

Forward-looking plan for upcoming features and milestones. This document
tracks what's **next**, not what's done — for completed work, see git log and
`plans/*/` directories.

## Guiding Principles

- Each item lists the *future* value, not the past state.
- Cross-module infra lands before per-module features that need it.
- Keep items small enough to ship in one PR when possible.

## Modules

### Wordle

- **Daily mode.** Wire up `pickDaily(words, todayUtc())` so `/wordle` defaults
  to the shared puzzle of the day (one target per UTC date for every player).
  Give per-day stats a dedicated key so historical streaks aren't lost when
  random mode and daily mode are mixed. `pickDaily` already exists in
  `src/modules/wordle/daily.js` and has tests; the wiring in `handlers.js` is
  the missing piece.
  - Decide: should random-mode `/wordle_new` still work alongside daily, or
    should the flow be "one puzzle per day, no reroll"?

### Loldle

- **Daily mode.** Same shape as wordle's daily-mode plan — use `pickDaily`
  from `src/modules/loldle/daily.js` instead of `pickRandom`.

### Trading

- **Crypto support.** Add BTC / ETH price feed + asset category. Mirror the
  stock flow: dynamic symbol resolution, cache in KV, reuse `portfolio.assets`.
- **Gold support.** SJC / PNJ spot price feed, treated as a single asset.
- **Currency exchange.** `/trade_convert VND USD 1000000` — already scaffolded
  as "coming soon". Needs a forex buy/sell step that debits one currency and
  credits another.
- **Leaderboard.** Cross-user ranking by realized P&L over a time window.
  D1 query on `trading_trades` plus portfolio snapshots.

## Infrastructure

- **Shared `picker` util.** Extract the duplicated `todayUtc` / `pickDaily` /
  `pickRandom` / djb2 hash from `loldle/daily.js` and `wordle/daily.js` into a
  single `src/util/picker.js`. Consolidate tests.
- **Handler-level tests** for wordle + loldle mirroring
  `tests/modules/trading/handlers.test.js` (subject resolution, giveup flow,
  stats rendering, finished-round branch).
- **Coverage reporting.** Add vitest `--coverage` config and wire a threshold
  gate into `npm test`.
- **Staging environment.** Separate D1 database + KV namespace for a staging
  Worker, so migrations can be validated before prod.

## Unresolved Questions

- Daily-mode scope: global puzzle (everyone gets the same word / champion) vs
  per-chat daily (each group has its own seed)?
- Does daily mode count toward the existing `stats:<subject>` record, or live
  in a separate `daily-stats:<subject>:<date>` namespace?
- Do we keep `/wordle_new` in daily mode, or hide it until the next UTC
  rollover? (Loldle auto-starts a fresh round after solve/giveup, so the
  question only applies to wordle now.)
