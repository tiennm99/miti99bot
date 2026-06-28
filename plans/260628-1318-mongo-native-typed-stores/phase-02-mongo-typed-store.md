---
phase: 2
title: "Mongo typed store + integration tests"
status: done
priority: P2
dependencies: [1]
effort: "L"
---

# Phase 2: Mongo typed store + integration tests

## Overview

Implement `MongoDocStore[T]` and a `MongoProvider` that satisfies the Phase 1
contract, persisting typed payloads as flattened native root documents. Lock the
raw shape and CAS semantics with Mongo integration tests.

## Requirements

- `MongoProvider.Collection(module)` returns a Mongo-backed `Collection`.
- `Typed[T]` over a Mongo collection yields `MongoDocStore[T]`.
- Write shape: `{ _id, ...payload, version, updatedAt(Date) }`; no `value`, no `_payload`.
- Whole-document `ReplaceOne` (no `$set`) so overwrites drop stale fields.
- Version CAS identical in behavior to the current `MongoKVStore`.
- `updatedAt` stored as `time.Time`.
- Plain `Put` bumps version via a bounded versioned-write retry loop.

## Architecture

```go
type storedDoc[T any] struct {
    ID        string    `bson:"_id"`
    Version   int64     `bson:"version"`
    UpdatedAt time.Time `bson:"updatedAt"`
    Payload   T         `bson:",inline"`
}

type MongoDocStore[T any] struct{ coll *mongo.Collection }
```

- `Get`: `FindOne({_id})` → decode `storedDoc[T]`; `ErrNoDocuments`→`ErrNotFound`;
  return `Payload`, `Version`.
- `PutVersioned(expected>0)`: `ReplaceOne({_id, version:expected}, storedDoc{version:expected+1, now, payload})`;
  `MatchedCount==0`→`ErrConflict`.
- `PutVersioned(0)`: upsert filter `{_id, $or:[version absent, version:0]}` with
  replacement `version:1`; duplicate-key→`ErrConflict`. (Port existing logic.)
- `Put`: loop `Get`→`PutVersioned(version)` up to N attempts (reuse the small
  retry constant style already in coin/gold); first write uses `expected=0`.
- `Delete`: `DeleteOne` idempotent.
- `List(prefix)`: half-open `_id` range scan via `prefixSuccessor`, project `_id`
  only. (Port from current `MongoKVStore.List`.)
- Error messages name module + key + reason.

`MongoProvider` wraps `*mongo.Database`; `Collection(module)` validates the
module name (reuse existing validation) and returns `mongoCollection{db.Collection(module)}`.

## Related Code Files

- Create: `internal/storage/mongo_doc_store.go` (MongoDocStore + storedDoc).
- Modify: `internal/storage/mongodb_provider.go` (implement new `Provider`/`Collection`).
- Create: `internal/storage/mongo_doc_store_test.go` (integration, gated by `MONGODB_TEST_URL`).
- Delete (this phase): `mongodb_kv.go`, `mongodb_value_codec.go`, `mongodb_kv_test.go`.
- Read: current `mongodb_kv.go` (port CAS + List), `mongodb_provider.go`.

## Tests Before

Gated by `MONGODB_TEST_URL`:

1. `TestMongoDocStore_RootShape` — Put a portfolio-like struct; raw `bson.M` has
   root payload fields, `version`, `updatedAt`; **no** `value`, **no** `_payload`;
   `updatedAt` decodes as `time.Time`.
2. `TestMongoDocStore_OverwriteRemovesStaleFields` — write struct A with field
   set X, overwrite with struct B lacking some of X; raw doc has no stale field.
3. `TestMongoDocStore_PutVersioned_ConcurrentCreate` — N goroutines create same
   `_id`; exactly one wins, rest get `ErrConflict`.
4. `TestMongoDocStore_PutVersioned_Update` — stale expected → `ErrConflict`.
5. `TestMongoDocStore_Put_BumpsVersion` — plain Put increments version, survives
   a concurrent bump (retry loop succeeds or conflicts deterministically).
6. `TestMongoDocStore_List_Prefix`.
7. `TestMongoDocStore_WrappedScalar` — a `struct{ Date string }` payload stores
   `date` at root (proves the lolschedule wrapping works).
8. `TestMongoDocStore_WrappedArray` — a `struct{ Subscribers []sub }` payload
   stores `subscribers` array at root.

## Implementation Steps

1. Implement `MongoDocStore[T]` (port CAS + List from `MongoKVStore`, typed).
2. Reshape `MongoProvider` to the new `Provider`/`Collection`; wire `Typed[T]`.
3. Delete `mongodb_kv.go`, `mongodb_value_codec.go` and their tests.
4. `go build ./internal/storage` — expect module/server breakage deferred to
   Phase 3 (storage package itself must compile).
5. Make Phase 2 integration tests pass against `make mongo-local`.

## Tests After

```sh
go test ./internal/storage
MONGODB_TEST_URL=mongodb://127.0.0.1:27017 go test ./internal/storage -run 'MongoDocStore'
```

## Success Criteria

- [ ] Raw Mongo doc is flattened root payload + `version` + Date `updatedAt`; no `value`/`_payload`.
- [ ] Overwrite removes stale fields.
- [ ] Concurrent create/update keep single-winner CAS.
- [ ] Wrapped scalar/array payloads store as named root fields.
- [ ] Old `MongoKVStore`/codec deleted; storage package builds.

## Risk Assessment

- Risk: `bson:",inline"` edge cases (pointer payloads, embedded maps). Mitigation:
  require payload `T` to be a concrete struct; test scalar/array wrappers.
- Risk: bounded `Put` loop contention. Mitigation: small retry count; CAS users
  call `PutVersioned` directly anyway.
- Risk: driver decodes `version` as int32. Mitigation: `storedDoc.Version int64`
  + decode test.
