---
phase: 5
title: "Verification + Mongo-only rollout"
status: done
priority: P2
dependencies: [4]
effort: "M"
---

# Phase 5: Verification + Mongo-only rollout

## Overview

Full validation and a simple cutover. Because Atlas is empty until cutover and
there is no legacy Mongo data, rollout is one-way and clean: migrate from
DynamoDB, then run Mongo-only. No in-place Mongo migration, no dual-read.

## Requirements

- All test gates pass (`make vet`, `make test`, Mongo integration, migrator e2e).
- Raw Mongo docs verified flattened for representative modules.
- App behavior unchanged for all Telegram commands.
- Cutover + rollback notes recorded.

## Validation gates

```sh
make vet
make test
MONGODB_TEST_URL=mongodb://127.0.0.1:27017 go test ./internal/storage ./cmd/migrate-dynamo-to-mongo
# migrator e2e additionally needs DynamoDB Local (DYNAMODB_LOCAL_URL)
```

Manual raw-doc inspection (Atlas/Compass or a small test assertion) for:

- coin/gold/stock portfolio: root fields, `version`, Date `updatedAt`, no `value`.
- wordle/loldle/twentyq game + stats: root fields.
- lolschedule: `subscribers` array field; last-push `date` field.
- stats counters: root fields + List works.

## Cutover order (operator)

1. Backup/export current DynamoDB table (source of truth) before cutover.
2. Disable EventBridge / stop the old AWS app so DynamoDB writes stop.
3. `migrate-dynamo-to-mongo --dry-run` → review per-module counts.
4. `migrate-dynamo-to-mongo` → writes flattened native docs to Atlas.
5. `migrate-dynamo-to-mongo --verify` → per-collection count parity (exit 0).
6. Start the Mongo-only container (`MONGO_URL`+`MONGO_DATABASE`); confirm health.
7. Spot-check a few commands (coin balance, wordle state, lolschedule subscribe).

## Rollback

- Before step 6 (no Mongo writes yet): re-enable the old AWS app on DynamoDB.
  DynamoDB is untouched (migrator is Scan-only), so this is safe.
- After step 6 (Mongo has taken live writes): forward-fix preferred. To revert to
  DynamoDB you would need a reverse Mongo→DynamoDB export of anything written
  after cutover. Keep the DynamoDB backup from step 1 as the floor.

## Related Code Files

- Read: `Makefile`, `README.md`, `docs/deploy-coolify-selfhosted.md`,
  `docs/aws-decommission-runbook.md`.
- Modify: tests only if verification finds gaps.

## Implementation Steps

1. Run all gates above; fix regressions (do not weaken tests).
2. Run the migrator e2e against local Mongo + DynamoDB Local.
3. Inspect raw docs per the list; optionally add a small assertion test.
4. Record the cutover commands + rollback constraints in the completion report
   under `plans/reports/`.
5. Grep docs for stale `{ _id, value, version, updatedAt }` wording; fix.

## Success Criteria

- [ ] `make vet`, `make test` pass.
- [ ] Mongo integration + migrator e2e pass.
- [ ] Raw docs verified flattened (no `value`/`_payload`) for representative modules.
- [ ] No Telegram command behavior change.
- [ ] Cutover + rollback documented in the completion report.

## Risk Assessment

- Risk: a module's data didn't round-trip through migration. Mitigation: e2e
  reads migrated docs back through the typed store; spot-check live commands.
- Risk: Atlas M0 limits during migrate. Mitigation: tiny dataset; collection-by-
  collection counts in `--verify`.
- Risk: one-way cutover. Mitigation: DynamoDB backup + Scan-only migrator means
  pre-write rollback is safe; post-write rollback documented as forward-fix.
