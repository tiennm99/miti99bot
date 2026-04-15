# D1 + Cron Infra + JSDoc + Trading History

**Created:** 2026-04-15
**Status:** Complete
**Branch:** main

## Goal

Add Cloudflare D1 + Cron Triggers to the plug-n-play module framework, document them for future module authors, add JSDoc tooling via ESLint, and ship one real D1-backed feature: trading trade history.

## Non-Goals

- No module renames (`wordle`, `loldle` stay as-is).
- No demo D1/cron features for wordle/loldle — infra + docs only.
- No TypeScript migration, no `tsc`, no `jsconfig.json`.
- No preview D1 — single production DB; Miniflare for tests.
- No inline retention — cleanup is a separate cron.

## Locked Decisions

| # | Decision |
|---|---|
| D1 scope | Single prod DB. Tests use Miniflare. |
| Table prefix | `{module}_{table}` (e.g. `trading_trades`). Enforced by `SqlStore`. |
| Migrations | Per-module at `src/modules/<name>/migrations/*.sql`. Applied on `npm run deploy`. |
| Cron contract | `crons: [{ schedule, handler }]`; handler signature `(event, { db, sql, env })`. |
| Trade retention | 1000/user + 10000/global, FIFO. Enforced by daily cleanup cron. |
| JSDoc tooling | ESLint + `eslint-plugin-jsdoc`. Runs alongside Biome. |

## Phases

| # | File | Status |
|---|---|---|
| 01 | [phase-01-d1-setup.md](phase-01-d1-setup.md) | Complete |
| 02 | [phase-02-cron-wiring.md](phase-02-cron-wiring.md) | Complete |
| 03 | [phase-03-trading-history.md](phase-03-trading-history.md) | Complete |
| 04 | [phase-04-retention-cron.md](phase-04-retention-cron.md) | Complete |
| 05 | [phase-05-jsdoc-pass.md](phase-05-jsdoc-pass.md) | Complete |
| 06 | [phase-06-docs.md](phase-06-docs.md) | Complete |

## Key Dependencies

- Phase 02 depends on Phase 01 (needs `SqlStore` available in cron handler context).
- Phase 03 depends on Phase 01 (needs D1 + `SqlStore`).
- Phase 04 depends on Phases 02 + 03 (needs cron wiring + trades table).
- Phase 05 can run in parallel with 01–04.
- Phase 06 last — documents final state.

## Success Criteria

- `npm test` green (Miniflare-backed D1 tests pass).
- `npm run deploy` applies pending migrations + deploys worker + registers webhook/commands.
- A module author can add a D1-backed feature + a cron by following `docs/using-d1.md` + `docs/using-cron.md` without reading framework internals.
- Trading `/history` returns last N trades for caller.
- Daily cleanup cron trims trades to caps.
- `npm run lint` runs Biome + ESLint (JSDoc rules) clean.
