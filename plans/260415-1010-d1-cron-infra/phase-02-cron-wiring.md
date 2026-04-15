# Phase 02 — Cron Wiring

**Priority:** P0
**Status:** Complete
**Depends on:** Phase 01

## Overview

Add Cloudflare Cron Triggers support to the module framework. Modules can declare `crons: [{ schedule, handler }]` alongside `commands`.

## Requirements

**Functional**
- Module contract extended with optional `crons[]`.
- Each cron entry: `{ schedule: string, name: string, handler: async (event, ctx) => void }`.
  - `schedule` is a cron expression (e.g. `"0 1 * * *"`).
  - `name` required for logging + conflict detection (unique within module).
  - `handler` receives `(event, { db, sql, env })`.
- `src/index.js` exports `scheduled(event, env, ctx)` in addition to `fetch`.
- `scheduled()` dispatches to all modules whose `schedule` matches `event.cron`.
- Multiple modules can share the same schedule — all their handlers fire.
- `wrangler.toml` requires `[triggers] crons = [...]` — populated by a build step OR manually (decision below).

**Non-functional**
- Errors in one cron handler do not block others (`Promise.allSettled`).
- Handler timeouts bounded by Workers cron execution limits (15min max).

## Architecture

```
Cron Trigger fires
        │
        ▼
src/index.js → scheduled(event, env, ctx)
        │
        ▼
getRegistry(env)  ◄── reuses existing memoized registry
        │
        ▼
for each module.crons[] where entry.schedule === event.cron:
    ctx.waitUntil(entry.handler(event, {
        db: createStore(module.name, env),
        sql: createSqlStore(module.name, env),
        env,
    }))
```

## Related Code Files

**Create**
- `src/modules/cron-dispatcher.js` — dispatches `event.cron` to matching handlers
- `tests/modules/cron-dispatcher.test.js`

**Modify**
- `src/index.js` — add `scheduled` export
- `src/modules/registry.js` — collect + validate `crons[]` per module; conflict check on `(module, cronName)` duplicates
- `src/modules/validate-command.js` → add `validate-cron.js` sibling
- `wrangler.toml` — add `[triggers] crons = ["0 1 * * *", ...]` (union of all schedules)
- `scripts/register.js` — no change (cron triggers are set by `wrangler deploy` from toml)
- Docs for module contract

## Open Decisions

- **`wrangler.toml` crons population:**
  - (a) manual — module author adds schedule to toml when adding cron.
  - (b) generated — prebuild script scans modules, writes toml triggers.
  - **Lean (a)** for simplicity — YAGNI. Document in `adding-a-module.md`.

## Todo List

- [x] `cron-dispatcher.js`
- [x] `validate-cron.js`
- [x] Extend `registry.js` to surface `crons[]`
- [x] Add `scheduled` export in `src/index.js`
- [x] Update module contract JSDoc typedef
- [x] Unit tests for dispatcher (schedule match, fan-out, error isolation)
- [ ] Document in `docs/using-cron.md` (done in Phase 06)

## Success Criteria

- A module declaring `crons: [{ schedule: "*/5 * * * *", name: "tick", handler }]` has `handler` invoked every 5 min locally via `wrangler dev --test-scheduled` and in prod.
- Error in one handler doesn't prevent others.
- `npm test` green.

## Risks

- `wrangler dev --test-scheduled` integration — document the `curl "http://localhost:8787/__scheduled?cron=..."` pattern.
- Cold start on cron: registry memoization across fetch+scheduled invocations — ensure single shared cache.

## Next Steps

- Phase 04 (retention cron) consumes this.
