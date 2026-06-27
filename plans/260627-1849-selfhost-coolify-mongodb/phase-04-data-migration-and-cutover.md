---
phase: 4
title: "Data Migration and Cutover"
status: pending
priority: P1
dependencies: [1, 3]
effort: "M"
---

# Phase 4: Data Migration and Cutover

## Overview

Copy all existing items from the prod DynamoDB table (`miti99bot-data`) into MongoDB Atlas using the finalized Phase 1 document schema, verify parity, then switch Telegram from the Lambda webhook to the long-polling container (Phase 3). Idempotent and re-runnable.

## Requirements

- Functional: every DynamoDB item → one Mongo document in collection `pk`, `_id = sk`, `value` = decoded value, `updatedAt` preserved.
- Functional: idempotent (re-run overwrites by `_id`, no duplicates); resumable on failure.
- Functional: verification step compares per-module (per-`pk`) counts DynamoDB vs Mongo and spot-checks values.
- Non-functional: read-only on DynamoDB (Scan only); never mutates source.
- Non-functional: dry-run mode that reports counts without writing.

## Architecture

Mapping (confirmed from `dynamodb_kv.go` ↔ `firestore_kv.go`):

| DynamoDB | Mongo |
|---|---|
| `pk` (S) = module name | collection name |
| `sk` (S) = user key | document `_id` |
| `value` (S) = JSON string | `value` field (BSON binary, same bytes) |
| `updatedAt` (N) = unix nanos | `updatedAt` (int64 nanos, verbatim) |

A full-table `Scan` (table is small KV) yields all items across all partitions. Group by `pk` → write into the matching collection. Read with `storage.NewDynamoDBClient`; **write through the Phase 1 `MongoKVStore.Put` (NOT raw `UpdateOne`)** so `value`/`updatedAt` encoding is byte-identical to what the live app writes — otherwise the migrator could store `value` as a BSON string while the app writes binary, and a later CAS (`{value: expected}`) never matches → spurious `ErrConflict` and the "idempotent re-run" criterion silently breaks.

New one-off CLI `cmd/migrate-dynamo-to-mongo/main.go`:
- Flags/env: `--dynamodb-table` (default `miti99bot-data`), `MONGO_URL`, `MONGO_DATABASE`, `--dry-run`, `--verify`.
- Uses AWS default cred chain via a **dedicated read-only profile/role** (see IAM below) — NOT an admin profile.
- For each item: **`validateKey(sk)` before writing**; fail loudly on any `/`-containing or otherwise-invalid `sk`. The app's read path runs `validateKey` first (firestore/dynamodb pattern), so a key the migrator writes but `validateKey` rejects would be silently unreadable. No `/` keys exist today (all modules use `:` separators), so this is a guard, not a transform.
- Writes via `MongoKVStore.Put`; `Put` overwrites by `_id`, so re-runs are idempotent and produce no duplicates.
- `--verify`: per-`pk` count on DynamoDB via a **Scan tally** (so a single `dynamodb:Scan` permission suffices — do NOT use Query, which would need `dynamodb:Query` and over-grant) vs `CountDocuments` per collection; print a table + mismatches; exit non-zero on mismatch.

IAM (least privilege): exactly `dynamodb:Scan` on the specific table ARN (`arn:…:table/miti99bot-data`), nothing else. No write actions on the source — enforces the read-only requirement and removes the destructive-credential foot-gun.

Cutover sequence (documented runbook — zero-loss):
1. Deploy the Coolify container (Phase 3, polling — the only transport), but keep it **stopped/scaled-to-0** for now (a running poller would 409 against the live Lambda webhook, and would start serving before data is migrated, and its always-on scheduler would overlap EventBridge). Fresh/empty Atlas DB.
2. **Disable/delete the EventBridge `LolscheduleDailyPushSchedule`** (`template.yaml:271-289`) — it invokes the Lambda directly, independent of transport, so it must be stopped before any internal scheduler runs. (The Phase 2 last-push-date guard is the belt-and-braces backup.)
3. `deleteWebhook` (MANDATORY) — does double duty: (a) Telegram now buffers incoming updates up to 24h, making the cut lossless (do NOT take an "accept the gap" path — a `/buy` in the gap would write to DynamoDB only and be lost, `coin/portfolio.go:64-90`); (b) it releases the webhook so the polling container's `getUpdates` won't 409. After this the Lambda stops receiving updates.
4. Run migrator `--dry-run` → review counts. Then run for real; run `--verify` (counts equal per `pk`, exit 0). Keep this window short (target minutes).
5. Start the Coolify container (its in-process scheduler runs by default — safe now that EventBridge is disabled). Exactly 1 replica (single polling consumer).
6. The container's `getUpdates` loop drains Telegram's buffered queue automatically — no `setWebhook`, no public URL. Confirm in logs that updates are being received.
7. Verify `getWebhookInfo` shows `url` empty (webhook cleared) and `pending_update_count` draining toward 0 as the poller consumes the backlog (same queue signal as `docs/deploy-aws.md:77`).
8. Smoke `/ping`, `/stats`, a coin/stock balance command — confirm migrated state is visible from Atlas.
9. **Tear down AWS** (validated decision): after `--verify` passes and smoke tests are green, `sam delete` the stack (Lambda, DynamoDB, EventBridge, SQS, etc.) and disable the GitHub Actions `deploy.yml` workflow. The user coordinates users to avoid activity during the brief cutover window, so the simple cut is lossless; no reverse migrator is built. <!-- Updated: Validation Session 1 - tear down AWS after verify; no reverse migrator -->
10. **Last clean revert point:** rollback to AWS is only possible BEFORE `sam delete`. To revert during the observation window: stop the polling container, then re-`setWebhook` to the Lambda Function URL — the still-deployed Lambda runs its own already-built webhook code (this branch's webhook removal doesn't touch the live function until `sam delete`). Lossless only until the first post-cutover Mongo write. After `sam delete`, MongoDB/Coolify is the sole system of record. <!-- Updated: Validation Session 2 - polling cutover -->

## Related Code Files

- Create: `cmd/migrate-dynamo-to-mongo/main.go` — the migrator.
- Create: `cmd/migrate-dynamo-to-mongo/README.md` (or section in deploy doc) — usage, required IAM (`dynamodb:Scan` on the table), env vars, dry-run/verify, cutover runbook.
- Modify: `Makefile` — `migrate-dynamo-to-mongo` + `migrate-verify` targets wrapping the CLI with sensible defaults.
- Modify: `docs/deploy-coolify-selfhosted.md` — link the cutover runbook.
- Reference: prior `plans/260515-2250-cf-data-to-aws-migration/` — same migration shape (Firestore→DynamoDB); reuse its verification approach if present.

## Implementation Steps

1. Implement the migrator using existing storage clients; default table `miti99bot-data`.
2. Implement `--dry-run` (scan + group + report counts, no writes) and `--verify`.
3. Test locally: seed DynamoDB Local with a few items across 2+ modules, run against Mongo Local, assert documents match via the Phase 1 `MongoKVStore.Get`.
4. Add Make targets.
5. Dry-run against prod DynamoDB; review counts per module.
6. Execute migration + verify; record the count table in the cutover doc.
7. `deleteWebhook`, start the polling container, smoke-test, monitor.

## Success Criteria

- [ ] Dry-run reports per-module item counts without writing.
- [ ] Real run copies all items; `--verify` shows DynamoDB and Mongo counts equal for every `pk`, exit 0.
- [ ] Re-running the migrator produces no duplicates and no value changes (idempotent).
- [ ] Migrator runs `validateKey` per `sk` and writes through `MongoKVStore.Put`; a `Get` round-trip spot check returns byte-identical values.
- [ ] After cutover (polling live, webhook cleared), a previously-stored value (e.g. a user's paper-trade balance) is returned by the live bot from Atlas.
- [ ] EventBridge schedule disabled before the container starts; daily push fires exactly once on cutover day.
- [ ] `getWebhookInfo` shows the webhook URL empty and the polling container is the sole update consumer (no 409).
- [ ] Rollback truth documented: reverting to the Lambda webhook is lossless only before the first post-cutover Mongo write (RPO stated explicitly).

## Risk Assessment

- **Rollback bound — accepted (validated decision)**: AWS is torn down after verify, so there is no long-term fallback by design and no reverse migrator is built. The user coordinates users to pause around the brief cutover window, so the simple cut loses no writes. The only clean-revert opportunity is the short observation window BEFORE `sam delete` (stop the poller, re-`setWebhook` to Lambda), and even then it is lossless only until the first post-cutover Mongo write. This is an accepted RPO for a personal paper-trading bot, not an open risk. <!-- Updated: Validation Session 1 - accepted; teardown after verify -->
- **Migration window write-loss**: the user pauses user activity during cutover (coordinated), and the mandatory `deleteWebhook` buffers any stray updates (Telegram retains ~24h). The "accept the gap" option is removed. Keep the window short and confirm `pending_update_count` drains post-flip.
- **Cron double-fire during cutover**: EventBridge schedule is disabled BEFORE the container starts (runbook step 2); the Phase 2 last-push-date guard is the backup. Without both, subscribers get the daily push twice (`lolschedule/cron.go:128-191` is non-idempotent).
- **`updatedAt` type**: store as int64 unix-nanos in Mongo (matches DynamoDB `dynamodb_kv.go:101`) — exact parity, no truncation. No code reads `updatedAt` today (write-only), so this is cheap insurance against a future TTL/sort.
- **Value/encoding mismatch**: migrator writes through `MongoKVStore.Put` (same encoding as the app), not raw `UpdateOne` — guarantees byte-identical `value` and keeps re-runs + CAS correct. Spot-check with a `Get` round-trip.
- **Key validity asymmetry**: migrator runs `validateKey(sk)` before writing and fails loud on any rejected key, so it never writes data the app's read path can't load.
- **IAM for Scan**: dedicated read-only profile/role with exactly `dynamodb:Scan` on the table ARN. No admin profile, no write actions on the source.
- **Large value / 16MB BSON cap**: KV values are tiny JSON; far under limit. No action.
