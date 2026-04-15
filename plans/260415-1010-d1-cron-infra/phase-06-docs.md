# Phase 06 — Docs

**Priority:** P1 (last)
**Status:** Complete
**Depends on:** All prior phases (✓ complete)

## Overview

Document D1 + Cron usage for module authors. Update existing architecture + module-authoring docs. Update `CLAUDE.md` module contract snippet.

## Deliverables

### Create

- `docs/using-d1.md` — how to add a D1-backed feature to a module
  - When to choose D1 vs KV (query patterns, relational needs, scans)
  - Writing migrations (`src/modules/<name>/migrations/NNNN_name.sql`)
  - `sql.prepare/run/all/first/batch` API reference
  - Table naming convention (`{module}_{table}`)
  - Accessing `sql` from `init` and command handlers
  - Testing with Miniflare D1 (`tests/fakes/fake-d1.js`)
  - Running migrations: `npm run db:migrate`

- `docs/using-cron.md` — how to add a scheduled job to a module
  - Declaring `crons: [{ schedule, name, handler }]`
  - Cron expression syntax + Cloudflare limits
  - Registering schedule in `wrangler.toml` `[triggers] crons`
  - Handler signature `(event, { db, sql, env })`
  - Local testing: `curl "http://localhost:8787/__scheduled?cron=0+17+*+*+*"`
  - Error isolation (one handler fail ≠ others fail)
  - Execution time limits

### Update

- `docs/adding-a-module.md` — add sections:
  - Optional `crons[]` field
  - Optional `init({ sql })` for D1
  - Migration file placement
  - Link to `using-d1.md` + `using-cron.md`

- `docs/architecture.md` — add:
  - `scheduled()` flow diagram alongside existing fetch flow
  - D1 store layer in storage section
  - Migration runner role in deploy

- `docs/codebase-summary.md` — reflect new `src/db/cf-sql-store.js`, `src/modules/cron-dispatcher.js`, `src/types.js`

- `docs/code-standards.md` — JSDoc expectations (when to add, ESLint rules enforced)

- `CLAUDE.md` (project) — update module contract code block:

  ```js
  {
    name: "mymod",
    init: async ({ db, sql, env }) => { ... },
    commands: [ /* ... */ ],
    crons: [{ schedule: "0 1 * * *", name: "daily", handler: async (event, { sql }) => { ... } }],
  }
  ```

- `CLAUDE.md` — add D1/cron bullets in "Commands" (npm run db:migrate)

- `README.md` — light touch: mention D1 + cron in "Why" bullets + update architecture snapshot tree

### Plan Output Docs

- Update `plans/260415-1010-d1-cron-infra/plan.md` — mark all phases Complete

## Todo List

- [x] Draft `docs/using-d1.md`
- [x] Draft `docs/using-cron.md`
- [x] Update `docs/adding-a-module.md`
- [x] Update `docs/architecture.md`
- [x] Update `docs/codebase-summary.md`
- [x] Update `docs/code-standards.md`
- [x] Update project `CLAUDE.md`
- [x] Update `README.md`
- [x] Mark plan phases complete

## Success Criteria

- A dev unfamiliar with Cloudflare Workers can follow `using-d1.md` + `using-cron.md` to add a persistent scheduled feature without reading framework internals.
- Internal docs reflect shipped state (no stale references to KV-only).
