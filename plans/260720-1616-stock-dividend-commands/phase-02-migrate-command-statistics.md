---
phase: 2
title: Migrate Command Statistics
status: completed
effort: ''
priority: P1
dependencies:
  - 1
---

# Phase 2: Migrate Command Statistics

<!-- Updated: Validation Session 1 - prepared-checkpoint retry and retention -->

## Context Links

- [Approved compatibility decision](../reports/260720-1612-stock-dividend-command-brainstorm.md#compatibility-and-touchpoints)
- [Project stats compatibility rules](../../AGENTS.md#stats-compatibility)

## Overview

Move historical usage to the commands that retain the old meanings, then leave
`stock_dividend` empty for new combined-event usage. Guard the one-time move in
the shared `system` collection and run it during server startup.

## Requirements

- Functional: migrate `stock_dividend -> stock_cash_dividend` and
  `stock_bonus -> stock_share_dividend`.
- Functional: include command-total rows (`uid=0`) and every per-user row;
  merge source counts into existing target rows rather than overwrite them.
- Functional: preserve user ID/username metadata, remove migrated source rows,
  and write the completion marker only after both mappings finish.
- Functional: a completed marker makes subsequent startups a no-op; tests cover
  target merging and repeated invocation for memory and MongoDB stores.
- Functional: an incomplete migration resumes both remaining source rows and
  `prepared` row checkpoints, including a checkpoint whose source was deleted.
- Functional: retain global and per-row markers permanently as migration history.
- Non-functional: migration errors fail startup before module registration, so
  new command meanings never run against unmigrated history.

## Architecture

Extend stats startup maintenance to receive both `stats` and `system`
collections. Use the existing typed stats documents and `systemstate.Store`.
A stable marker such as `migration:stock-dividend-command-stats-v1` records
global completion. For each source row, first persist a stable per-row
`prepared` checkpoint in `system` containing the exact final target count.
Retries set the target to that checkpointed count instead of adding again,
then delete the source and mark the row complete. Preserve the best available
username deterministically. Mark global completion only after every row is
complete. On startup without a global completion marker, enumerate both source
rows and prepared checkpoints so deletion-before-row-complete can recover.
Retain all markers as migration history; add no cleanup path.

## Related Code Files

- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stats\startup.go` — startup signature, marker guard, and migration orchestration.
- Create: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stats\startup_test.go` — memory migration, merges, anonymous/users, failures, idempotency.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stats\startup_mongo_test.go` — Mongo indexes plus migration parity and rerun checks.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\modules\stats\stats_test.go` — visible stats attribution after migration if needed.
- Modify: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\cmd\server\main.go` — pass `stats` and shared `system` collections before module build.
- Reference: `C:\Users\miti99\Workspaces\tiennm99\miti99bot\internal\systemstate\systemstate.go` — existing marker store; no schema change expected.

## Implementation Steps

1. Add memory tests seeding source-only, target-only, merged anonymous, merged
   per-user, multiple users, and both command mappings.
2. Assert exact post-migration keys/counts/metadata, source removal, marker
   content, and no change after a second startup call.
3. Add injected-failure tests at prepare, target-write, source-delete, and
   row-complete boundaries. Retry each case—including source already deleted—
   and prove counts never duplicate.
4. Implement deterministic global and per-row marker keys. A prepared marker
   stores the exact merged target count; retries overwrite with that value,
   never increment an already migrated target again.
5. Implement the mapping helper using deterministic `usageKey` values and the
   existing typed store. Keep mapping data declarative and local to stats.
6. Extend `InitStore` to create indexes and run the guarded migration; wrap
   errors with migration/command context.
7. Wire `provider.Collection(systemstate.CollectionName)` from server startup
   and preserve fail-fast logging before command registration.
8. Mirror critical merge, partial-retry, and idempotency cases in Mongo tests.
9. Run `gofmt`, focused stats tests, and focused server tests.

## Success Criteria

- [x] Old cash usage appears only under `stock_cash_dividend`.
- [x] Old bonus usage appears only under `stock_share_dividend`.
- [x] Existing target counts merge for anonymous and per-user rows without loss.
- [x] Retries after every partial-write boundary cannot duplicate counts.
- [x] Prepared checkpoints resume even when their source rows no longer exist.
- [x] Completed migration is idempotent and marker-backed in memory and MongoDB.
- [x] Global and per-row migration markers remain after completion.
- [x] Startup aborts on migration failure and `go test ./internal/modules/stats ./cmd/server` passes.

## Risk Assessment

- Reusing `stock_dividend` before migration would mislabel history. Keep the
  migration before module construction and treat errors as fatal.
- A mid-migration process crash may leave partial work. Prepare each row's exact
  final count before touching the target; retry by setting that value, not by
  adding again. Scan prepared checkpoints independently from source rows. Do
  not mark row/global completion early or delete historical markers.

## Security Considerations

The migration touches counts and public usernames only. Never log full records;
log marker/mapping names and aggregate counts.

## Next Steps

Phase 3 aligns all user-facing surfaces and runs the repository-wide gate.
