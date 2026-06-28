# migrate-dynamo-to-mongo

One-off CLI that copies every item from the prod DynamoDB KV table
(`miti99bot-data`) into MongoDB Atlas using the exact document schema the live
app writes, then verifies per-module parity. Idempotent and re-runnable.

## What it does

- Full-table `Scan` of DynamoDB (the table is a small KV) → group by `pk`.
- Writes each item through `storage.MongoKVStore.Put`, so the `value` encoding
  is **byte-identical** to what the running bot writes (BSON binary). This is
  what keeps re-runs idempotent and future CompareAndSwap correct — a raw
  `UpdateOne` that stored `value` as a BSON string would silently break both.
- `Put` runs `validateKey` on each sort key and upserts by `_id`, so a re-run
  produces no duplicates and an invalid key fails loud (no key the app's read
  path can't load is ever written).

| DynamoDB | MongoDB |
|---|---|
| `pk` (module name) | collection name |
| `sk` (user key) | document `_id` |
| `value` (JSON string) | `value` field (BSON binary, same bytes) |
| `updatedAt` | restamped to migration time (write-only field; nothing reads it) |

## Usage

```sh
export MONGO_URL='mongodb+srv://botuser:PASS@cluster0.xxxxx.mongodb.net/?retryWrites=true&w=majority'
export MONGO_DATABASE=miti99bot
export AWS_PROFILE=miti99bot-migrate     # READ-ONLY profile (see IAM below)

# 1. Dry run — report per-module counts, write nothing.
go run ./cmd/migrate-dynamo-to-mongo --dry-run
# or: make migrate-dynamo-to-mongo DRY_RUN=1

# 2. Real migration.
go run ./cmd/migrate-dynamo-to-mongo
# or: make migrate-dynamo-to-mongo

# 3. Verify — per-module counts must match; exits non-zero on mismatch.
go run ./cmd/migrate-dynamo-to-mongo --verify
# or: make migrate-verify
```

Flags: `--dynamodb-table` (default `miti99bot-data`), `--dry-run`, `--verify`.

For a local end-to-end test, point at DynamoDB Local + a local Mongo:

```sh
DYNAMODB_LOCAL_URL=http://localhost:8001 \
MONGODB_TEST_URL=mongodb://127.0.0.1:27017 \
MONGO_DATABASE=migrate_test \
go test ./cmd/migrate-dynamo-to-mongo/ -run TestMigrateAndVerify
```

## IAM — least privilege

The runner needs **exactly** `dynamodb:Scan` on the table ARN and nothing else.
Verify uses a Scan tally (not Query), so no `dynamodb:Query` is needed; there
are **no write actions on the source**, enforcing the read-only requirement and
removing the destructive-credential foot-gun.

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": "dynamodb:Scan",
    "Resource": "arn:aws:dynamodb:ap-southeast-1:225603493174:table/miti99bot-data"
  }]
}
```

## Cutover runbook

The full zero-loss cutover (disable EventBridge → `deleteWebhook` → migrate →
verify → start the polling container) lives in
[`docs/deploy-coolify-selfhosted.md`](../../docs/deploy-coolify-selfhosted.md#cutover-runbook).
