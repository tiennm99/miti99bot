---
phase: 3
title: "Migration and Documentation"
status: pending
priority: P2
dependencies: [2]
effort: "M"
---

# Phase 3: Migration and Documentation

## Overview

Add an operator-safe path to rewrite existing Mongo docs from the old `value`
envelope into the new flattened root shape, and update docs that currently
describe the old layout.

## Requirements

- Functional: existing Atlas documents can be migrated in place without relying
  on DynamoDB still existing.
- Functional: migration is idempotent and reports per-collection counts.
- Functional: migration preserves logical `Get`/`GetJSON` values and bumps or
  normalizes `version` consistently.
- Non-functional: migration must not read or print secrets; `MONGO_URL` remains
  secret.
- Non-functional: docs must stop calling int64 `updatedAt` TTL-ready.

## Architecture

Add a small Mongo-to-Mongo schema migration command rather than overloading the
DynamoDB migrator:

```text
cmd/migrate-mongo-schema/
  main.go
  main_test.go
```

Tool behavior:

1. Connect using `MONGO_URL` + `MONGO_DATABASE`.
2. Iterate collections or an allow-list flag.
3. For each document:
   - skip if it already has `schemaVersion >= 2` and no `value`;
   - read through `MongoKVStore.GetVersioned` so legacy decode paths are used;
   - rewrite through `PutVersioned` so root encoding is shared with app writes;
   - on `ErrConflict`, re-read the document: skip it if a live app write
     already migrated it, otherwise retry up to a small bounded limit;
   - count migrated/skipped/errors.
4. `--dry-run` prints counts without writes.
5. `--verify` confirms no documents with `value` remain, except explicitly
   allowed malformed docs if any are reported.

Do not drop or rename collections. Keep one collection per module.

## Related Code Files

- Create: `cmd/migrate-mongo-schema/main.go`
- Create: `cmd/migrate-mongo-schema/main_test.go`
- Modify: `Makefile` — add a migration target, e.g. `migrate-mongo-schema`.
- Modify: `README.md` — update storage layout summary.
- Modify: `docs/deploy-coolify-selfhosted.md` — update Atlas storage layout
  and migration note.
- Modify: `cmd/migrate-dynamo-to-mongo/README.md` — update destination shape.
- Modify: `cmd/migrate-dynamo-to-mongo/main.go` comments only if stale.
- Read: `cmd/migrate-dynamo-to-mongo/main.go` — reuse connection/config style.

## Tests Before

- Add migrator e2e test that seeds old `value` docs directly, runs migration,
  then asserts:
  - logical values round-trip;
  - raw docs have no `value`;
  - `updatedAt` is `time.Time`;
  - counts match.

## Implementation Steps

1. Implement CLI flags:
   - `--dry-run`
   - `--verify`
   - `--collection` optional repeat/single collection selector.
2. Reuse `storage.NewMongoClient`, `storage.NewMongoDatabase`, and
   `storage.NewMongoProvider`.
3. Add collection listing using the Mongo driver, excluding system collections.
4. Rewrite docs through the storage layer, not raw BSON mutation.
5. Print a concise table: collection, total, migrated, skipped, errors.
6. Add Makefile target with env examples.
7. Update docs and remove stale wording:
   - no old `value` envelope in new writes;
   - `updatedAt` is BSON Date;
   - legacy `value` docs dual-read until migrated.
8. Add migration conflict tests: simulate a version change between read and
   write; assert the migrator re-reads and either skips already-migrated docs or
   retries legacy docs without clobbering the concurrent update.
   <!-- Updated: Validation Session 1 - migration must handle live-write conflicts -->

## Tests After

- `go test ./cmd/migrate-mongo-schema`
- `MONGODB_TEST_URL=mongodb://127.0.0.1:27017 go test ./cmd/migrate-mongo-schema ./internal/storage`
- `make test`

## Success Criteria

- [ ] In-place migration rewrites old `value` docs without changing logical app
  values.
- [ ] Migration is idempotent.
- [ ] `--dry-run` performs no writes.
- [ ] `--verify` exits non-zero when old `value` docs remain.
- [ ] Migration handles version conflicts by re-reading and never clobbers a
  concurrent app write.
- [ ] README and Coolify deploy docs show the new flattened layout.
- [ ] DynamoDB-to-Mongo migrator docs no longer mention BSON binary/string as
  the destination `value` representation.

## Risk Assessment

- Risk: migration touches live data. Mitigation: dry-run + verify + local e2e
  test; recommend backup/export before production run.
- Risk: old DynamoDB source may be gone. Mitigation: in-place Mongo migration is
  independent of DynamoDB.
- Risk: malformed documents stop migration. Mitigation: report collection/key
  and continue only if an explicit `--continue-on-error` is later accepted;
  default should fail loud.
