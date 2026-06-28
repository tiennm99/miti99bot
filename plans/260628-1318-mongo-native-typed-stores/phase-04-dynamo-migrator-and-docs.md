---
phase: 4
title: "DynamoDB→Mongo migrator + docs"
status: done
priority: P2
dependencies: [3]
effort: "M"
---

# Phase 4: DynamoDB→Mongo migrator + docs

## Overview

Keep `cmd/migrate-dynamo-to-mongo` working, but make it write the new flattened
native shape directly. The migrator is schema-agnostic over modules, so it
cannot use the typed `DocStore[T]` (it has no compile-time `T` per row). It
writes documents with a small generic JSON→root-document encoder plus a tiny
explicit table for the only two non-object values.

## Requirements

- Migrator still Scans DynamoDB (read-only, `dynamodb:Scan` only) — unchanged.
- It writes `{ _id: sk, ...payloadFields, version:1, updatedAt(Date) }` per item.
- JSON object values flatten to root.
- The two known non-object DynamoDB values are wrapped to match the module's
  typed shape:
  - `lolschedule` subscribers (JSON array) → `{ subscribers: [...] }`
  - `lolschedule` last-push date (JSON/scalar string) → `{ date: "..." }`
- Idempotent (re-run overwrites by `_id`, never duplicates).
- `--dry-run` (counts only) and `--verify` (per-collection count parity) kept.
- No secrets read/printed; `MONGO_URL` stays secret.

## Architecture

Add a migrator-local encoder (NOT in `internal/storage`, to keep the runtime
typed and the generic JSON path confined to migration):

```go
// cmd/migrate-dynamo-to-mongo/encode.go
func rootDocForItem(module, key string, value []byte, now time.Time) (bson.M, error)
```

- Decode `value` with `json.Decoder.UseNumber` (int64 fidelity).
- If a `(module,key-prefix)` wrap rule matches → `{ wrapField: decoded }`.
- Else if decoded is a JSON object → spread its fields at root (reject keys
  colliding with `_id`/`version`/`updatedAt`; none expected — fail loud if seen).
- Else → error (an unexpected non-object value means a new wrap rule is needed;
  fail loud rather than guess).
- Always set `_id`, `version:int64(1)`, `updatedAt: now`.

Wrap rules table (explicit, documented):

```go
var wrapRules = []struct{ module, keyPrefix, field string }{
    {"lolschedule", "subscribers", "subscribers"},
    {"lolschedule", "last_push_date", "date"},
}
```

Write with `Collection(module).ReplaceOne({_id:key}, doc, upsert=true)` directly
via the driver (the migrator already holds `*mongo.Database`).

## Related Code Files

- Modify: `cmd/migrate-dynamo-to-mongo/main.go` — replace `provider.For(pk).Put`
  with `rootDocForItem` + `ReplaceOne` upsert; validate module/key names inline
  (the old `For` returned an erroring store for bad names — replicate that guard).
- Create: `cmd/migrate-dynamo-to-mongo/encode.go` + `encode_test.go`.
- Modify: `cmd/migrate-dynamo-to-mongo/main_test.go` — assert flattened shape +
  wrapped lolschedule docs.
- Modify: `cmd/migrate-dynamo-to-mongo/README.md` — new destination shape.
- Modify: `README.md` (storage layout summary), `docs/deploy-coolify-selfhosted.md`
  (Atlas layout + cutover note), `Makefile` if migrator target wording changed.

## Tests Before

1. `TestRootDocForItem_Object` — object JSON → root fields + meta; no `value`.
2. `TestRootDocForItem_Int64Fidelity` — large integral number stays int64.
3. `TestRootDocForItem_LolscheduleSubscribers` — array → `{subscribers:[...]}`.
4. `TestRootDocForItem_LolscheduleLastPush` — date string → `{date:"..."}`.
5. `TestRootDocForItem_UnknownScalar_Errors` — bare scalar w/o wrap rule fails loud.
6. Migrator e2e (`MONGODB_TEST_URL` + DynamoDB Local): seed DynamoDB rows for a
   few modules incl. lolschedule array+scalar → migrate → assert raw Mongo docs
   are flattened/wrapped, no `value`; re-run idempotent; `--verify` counts match;
   values readable by the module's `DocStore[T]`.

## Implementation Steps

1. Implement `rootDocForItem` + wrap rules.
2. Rewrite `runMigrate` to encode + `ReplaceOne` upsert; keep `--dry-run`.
3. Keep `runVerify` (count parity) — unchanged logic.
4. Add inline module/key validation (reuse `storage.ValidateKey` if exported, or
   replicate the rule) so bad rows fail loud.
5. Make migrator tests pass.
6. Update README/deploy docs to the flattened layout; remove BSON-string/binary
   destination wording.

## Success Criteria

- [ ] Migrator writes flattened native docs (no `value`) for all modules.
- [ ] lolschedule array + scalar land as `subscribers`/`date` root fields.
- [ ] Migrated docs are read back correctly by the typed module stores.
- [ ] Idempotent; `--dry-run` writes nothing; `--verify` exits non-zero on mismatch.
- [ ] Unknown non-object value fails loud (no silent guess).
- [ ] README + Coolify deploy docs show the flattened layout.

## Risk Assessment

- Risk: a module other than lolschedule stores a non-object (missed in survey).
  Mitigation: encoder fails loud on un-wrapped scalars/arrays; e2e covers the
  known set; the error names module/key so a new wrap rule is a one-liner.
- Risk: re-introducing a generic JSON codec invites runtime reuse. Mitigation:
  it lives only in `cmd/migrate-dynamo-to-mongo`, not `internal/storage`.
- Risk: DynamoDB source already gone at run time. Mitigation: migrator is the
  cutover step; if DynamoDB is empty/absent the operator skips it (documented).
