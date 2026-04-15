# Phase 01 — D1 Setup

**Priority:** P0 (blocker for 02/03/04)
**Status:** Complete

## Overview

Wire Cloudflare D1 into the framework: binding, per-module migrations, `SqlStore` factory mirroring the `KVStore` shape, Miniflare-backed tests.

## Key Insights

- D1 is SQLite at the edge; prepared statements + `db.prepare().bind().all()/first()/run()`.
- `vitest-pool-workers` (or plain Miniflare) exposes D1 in tests without real Cloudflare calls.
- Per-module table prefixing mirrors the existing KV prefixing — module authors never touch raw `env.DB`.

## Requirements

**Functional**
- Module init receives `sql` alongside `db` in `init({ db, sql, env })`.
- `sql.prepare(query, ...binds)` / `sql.run(query, ...binds)` / `sql.all(query, ...binds)` / `sql.first(query, ...binds)`.
- Table names referenced in queries are left literal — authors write `trading_trades` directly (prefix is convention, not rewriting).
- `sql.tablePrefix` exposed for authors who want to interpolate.
- Migrations auto-discovered from `src/modules/*/migrations/*.sql`, applied via `wrangler d1 migrations apply` in deploy script.

**Non-functional**
- Zero overhead when a module does not use SQL.
- Tests run fully offline against Miniflare.

## Architecture

```
src/db/
├── kv-store-interface.js   # existing
├── cf-kv-store.js          # existing
├── create-store.js         # existing (KV)
├── sql-store-interface.js  # NEW — JSDoc typedef
├── cf-sql-store.js         # NEW — wraps env.DB
└── create-sql-store.js     # NEW — factory, sets tablePrefix = `${moduleName}_`
```

`createSqlStore(moduleName, env)` returns an object exposing `prepare`, `run`, `all`, `first`, `batch`, `tablePrefix`.

## Related Code Files

**Create**
- `src/db/sql-store-interface.js`
- `src/db/cf-sql-store.js`
- `src/db/create-sql-store.js`
- `tests/db/create-sql-store.test.js`
- `tests/fakes/fake-d1.js` (Miniflare D1 helper for tests)

**Modify**
- `src/modules/registry.js` — pass `sql` into `init({ db, sql, env })`
- `src/modules/dispatcher.js` — same
- `wrangler.toml` — add `[[d1_databases]]` block, `migrations_dir` optional
- `package.json` — add `db:migrate` script: `wrangler d1 migrations apply miti99bot-db --remote`; chain into `deploy`
- `scripts/register.js` — no change needed, but verify no breakage

## Implementation Steps

1. Create D1 database: `npx wrangler d1 create miti99bot-db`. Record UUID in `wrangler.toml`.
2. Author `sql-store-interface.js` with JSDoc `@typedef` for `SqlStore`.
3. Implement `cf-sql-store.js` — thin wrapper around `env.DB.prepare()`.
4. Implement `create-sql-store.js` — returns wrapper + `tablePrefix`.
5. Update `registry.js` + `dispatcher.js` to pass `sql` into module `init` + command handler contexts (via `ctx.sql`? **decision below**).
6. Add migration discovery: walk `src/modules/*/migrations/` at deploy time, consolidate into a central `migrations/` or use wrangler's default per-dir.
7. Wire `db:migrate` into `npm run deploy`: `wrangler deploy && npm run db:migrate && npm run register`.
8. Add `fake-d1.js` using `@miniflare/d1` or `better-sqlite3`-backed fake.
9. Tests: `create-sql-store.test.js` verifying prefix exposure + basic CRUD.

## Open Decisions

- **Command handler access to `sql`:** expose via `ctx.sql` (grammY context extension in dispatcher) or require modules to close over `sql` captured in `init`? Lean **close over in init** — matches how `db` is currently used.
- **Migration runner:** wrangler's native `d1 migrations apply` requires a single `migrations_dir`. Options:
  - (a) consolidate all per-module SQL into root `migrations/` at build time via a prebuild script.
  - (b) custom runner script that applies each `src/modules/*/migrations/*.sql` in order.
  - **Lean (b)** — keeps per-module locality.

## Todo List

- [x] Create D1 database + update `wrangler.toml`
- [x] `sql-store-interface.js` with typedefs
- [x] `cf-sql-store.js` implementation
- [x] `create-sql-store.js` factory
- [x] Update `registry.js` init signature
- [x] Update `dispatcher.js` to pass `sql` (no change needed — delegates to buildRegistry)
- [x] Write custom migration runner at `scripts/migrate.js`
- [x] Wire into `npm run deploy`
- [x] `fake-d1.js` test helper
- [x] Unit tests for `create-sql-store`

## Success Criteria

- A module can define `init({ sql }) => sql.run("INSERT INTO mymod_foo VALUES (?)", "x")` and it works in dev + test + prod.
- `npm test` green.
- No regression in existing KV-only modules.

## Risks

- Wrangler migration tooling may not support per-module layout → fallback to custom runner.
- D1 read-after-write consistency in eventually-consistent replicas — document for module authors.

## Next Steps

- Phase 02 can start once `sql` threads through `init`.
