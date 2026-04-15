# Phase 04 — Retention Cron Implementation Report

**Date:** 2026-04-15
**Status:** DONE

## Files Modified

- `tests/fakes/fake-d1.js` — enhanced with SQL-semantic SELECT/DELETE support (DISTINCT, ORDER BY ts DESC, LIMIT/OFFSET with bind params, DELETE WHERE id IN)
- `src/modules/trading/retention.js` — created; per-user + global trim handler, overridable caps
- `src/modules/trading/index.js` — imported `trimTradesHandler`, added `crons` array
- `wrangler.toml` — `crons = ["0 17 * * *"]`
- `tests/modules/trading/retention.test.js` — created; 9 tests covering all scenarios
- `plans/260415-1010-d1-cron-infra/phase-04-retention-cron.md` — Status → Complete, todos ticked
- `plans/260415-1010-d1-cron-infra/plan.md` — phase 04 → Complete

## Tasks Completed

- [x] `retention.js` with per-user + global trim (hybrid SELECT-then-DELETE approach)
- [x] Caps exported as `PER_USER_CAP=1000`, `GLOBAL_CAP=10000`; overridable via optional arg for small-seed tests
- [x] Cron entry wired in trading module (`schedule: "0 17 * * *"`, `name: "trim-trades"`)
- [x] `wrangler.toml` schedule added
- [x] 9 retention tests: per-user trim, small-user no-op, exact-cap no-op, multi-user, global trim, combined pass, idempotence, sql=null guard

## Tests Status

- Unit tests: 169/169 passed (all files)
- Lint: clean (Biome + ESLint, 54 files)

## Key Decision

fake-d1 is SQL-less — DELETE naively cleared entire table. Chose option (b): hybrid SELECT-IDs-then-DELETE-by-id-list in `retention.js`. Enhanced fake-d1 to support targeted `DELETE WHERE id IN (?,...)` and `SELECT ... ORDER BY ts DESC LIMIT/OFFSET` with bind param resolution (`?` tokens resolved against binds iterator). This keeps tests meaningful without adding better-sqlite3 dependency.

## Concerns

None.
