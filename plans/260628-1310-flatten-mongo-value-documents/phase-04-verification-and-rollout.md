---
phase: 4
title: "Verification and Rollout"
status: pending
priority: P2
dependencies: [3]
effort: "M"
---

# Phase 4: Verification and Rollout

## Overview

Run the full validation suite, check raw Mongo documents, and define the safe
production rollout order. This phase is the gate before deploying a storage
schema change.

## Requirements

- Functional: app behavior remains unchanged for all Telegram commands.
- Functional: Mongo raw shape is verified for representative module documents.
- Functional: legacy docs remain readable before migration and absent after
  migration verify.
- Non-functional: rollout includes backup, deploy, migration, verify, and
  rollback notes.

## Architecture

Rollout order:

1. Backup/export current Atlas database or snapshot if available.
2. Deploy app version with dual-read + new-write root encoding.
3. Let app run; new/touched docs rewrite naturally.
4. Run `migrate-mongo-schema --dry-run`.
5. Run `migrate-mongo-schema`.
6. Run `migrate-mongo-schema --verify`.
7. Inspect sample docs in Atlas/Compass:
   - coin/gold/stock portfolio root fields;
   - wordle/loldle/twentyq game state root fields;
   - lolschedule scalar date guard under `_payload`;
   - no old `value` on migrated docs.

Rollback:

- If the new app fails before it handles writes, roll back binary; old `value`
  docs are still readable by the previous build.
- Once the new app has handled writes, rollback to the previous binary is unsafe
  for any touched flattened docs. Prefer fixing forward, or restore the backup.
- If migration ran, previous build may not read flattened docs at all. Rollback
  after migration requires restoring backup or a reverse migrator. Prefer fixing
  forward unless the migration corrupted data.

## Related Code Files

- Modify: tests only if verification gaps are found.
- Read: `Makefile`
- Read: `README.md`
- Read: `docs/deploy-coolify-selfhosted.md`
- Read: `cmd/migrate-mongo-schema/main.go`
- Read: `internal/storage/mongodb_kv_test.go`

## Tests Before

- Confirm Phase 1-3 tests run locally.
- Start local Mongo with `make mongo-local` for integration tests.

## Implementation Steps

1. Run focused storage tests.
2. Run module tests that rely on versioned storage flows:
   - coin portfolio;
   - gold portfolio;
   - lolschedule daily-push claim.
3. Run full hermetic suite.
4. Run Mongo integration suite.
5. Run DynamoDB tests only if the DynamoDB migrator or shared storage contracts
   changed.
6. Manually inspect raw docs or add a small test assertion report for:
   - no `value`;
   - expected root fields;
   - Date `updatedAt`;
   - stable `version`.
7. Document production run commands in the final completion report.

## Tests After

Required gates:

```sh
make vet
make test
MONGODB_TEST_URL=mongodb://127.0.0.1:27017 go test ./internal/storage ./cmd/migrate-mongo-schema
```

Conditional gate if DynamoDB migrator comments/tests changed:

```sh
make test-dynamodb
```

## Success Criteria

- [ ] `make vet` passes.
- [ ] `make test` passes.
- [ ] Mongo integration tests pass against local Mongo.
- [ ] New migrator e2e test passes.
- [ ] Raw docs verified no longer use `value` after migration.
- [ ] Production rollout notes include backup and rollback constraints.
- [ ] No user-visible Telegram command behavior changes.

## Risk Assessment

- Risk: previous binary cannot read newly-written or post-migration flattened
  docs. Mitigation: deploy dual-read binary first; backup before rollout;
  rollback old binary only before new writes happen; fix forward preferred.
- Risk: Atlas M0 resource limits during migration. Mitigation: tiny dataset
  expected; still scan collection-by-collection and report progress.
- Risk: hidden module stores non-object values. Mitigation: fallback `_payload`
  contract and migration tests for scalar/array docs.
- Risk: docs drift. Mitigation: grep for old `{ _id, value, version, updatedAt }`
  wording before finalizing.
