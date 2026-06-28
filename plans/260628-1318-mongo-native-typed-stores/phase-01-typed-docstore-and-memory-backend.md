---
phase: 1
title: "Typed DocStore contract + memory backend"
status: done
priority: P2
dependencies: []
effort: "M"
---

# Phase 1: Typed DocStore contract + memory backend

## Overview

Define the new typed storage contract and a memory implementation first, with
tests, before touching Mongo or any module. This nails the interface every
module and the Mongo backend will target.

## Requirements

- `DocStore[T]` generic interface: `Get`, `Put`, `PutVersioned`, `Delete`, `List`.
- `Provider.Collection(module)` returns an opaque per-module `Collection`.
- `Typed[T](Collection) DocStore[T]` free function (methods cannot be generic).
- Memory implementation: hermetic, concurrency-safe, version-CAS correct.
- Reserved-field name guard: payload BSON tags must not collide with `_id`,
  `version`, `updatedAt`.
- `ErrNotFound`, `ErrConflict` reused (move to this package if `kv_store.go` is deleted later).

## Architecture

```go
// doc_store.go
type DocStore[T any] interface {
    Get(ctx, id) (T, int64, error)
    Put(ctx, id, T) error
    PutVersioned(ctx, id, expectedVersion int64, T) error
    Delete(ctx, id) error
    List(ctx, prefix string) ([]string, error)
}

type Provider interface{ Collection(module string) Collection }
type Collection interface{ collection() } // sealed marker

func Typed[T any](c Collection) DocStore[T] // switch concrete type → mongo|memory store
```

Memory backend:

```go
// memory_provider.go (replaces kv_provider.go's MemoryProvider)
type MemoryProvider struct{ mu sync.Mutex; cols map[string]*memoryCollection }
type memoryCollection struct{ mu sync.Mutex; rows map[string]memoryRow }
type memoryRow struct{ version int64; val any } // val holds the typed value

type MemoryDocStore[T any] struct{ c *memoryCollection }
```

- `Get`: type-assert `row.val.(T)`; missing → `ErrNotFound`.
- `Put`: overwrite, `version++`.
- `PutVersioned`: compare stored version to expected; mismatch → `ErrConflict`;
  `expected==0` requires absent/zero-version row.
- `Delete`: idempotent.
- `List(prefix)`: sorted keys with prefix (reuse `validatePrefix`).

Keys: keep `validateKey`/`validatePrefix` from `keys.go`/`prefix.go`.

## Related Code Files

- Create: `internal/storage/doc_store.go` (interface, Provider, Collection, `Typed`, errors).
- Create: `internal/storage/memory_provider.go` (MemoryProvider + memoryCollection + MemoryDocStore).
- Create: `internal/storage/memory_doc_store_test.go`.
- Create: `internal/storage/reserved_fields.go` + test (reserved-name guard helper).
- Read: `internal/storage/keys.go`, `prefix.go`, `kv_store.go` (for error vars to migrate).

## Tests Before

Write failing tests first:

1. `TestMemoryDocStore_PutGetRoundTrip` — typed struct round-trips; version starts at 1.
2. `TestMemoryDocStore_PutBumpsVersion` — repeated Put increments version.
3. `TestMemoryDocStore_PutVersioned_CAS` — stale expected → `ErrConflict`; fresh succeeds.
4. `TestMemoryDocStore_PutVersioned_CreateZero` — `expected==0` creates; second create → `ErrConflict`.
5. `TestMemoryDocStore_Delete_Idempotent`.
6. `TestMemoryDocStore_List_Prefix` — disjoint key prefixes isolate types in one collection.
7. `TestReservedFields_RejectsCollision` — a payload type tagged `version` is flagged.
8. `TestMemoryProvider_CollectionIsolation` — two modules' collections don't share keys.

## Implementation Steps

1. Add `doc_store.go` with interface, sealed `Collection`, `Typed[T]`, and the
   `ErrNotFound`/`ErrConflict` vars (kept here so `kv_store.go` can be deleted).
2. Implement memory provider + store.
3. Implement reserved-field reflection guard (`bson` tag scan).
4. Make all Phase 1 tests pass.

## Success Criteria

- [ ] `DocStore[T]`, `Provider`, `Collection`, `Typed` compile and are documented.
- [ ] Memory store passes CAS/version/list/delete tests.
- [ ] Reserved-name guard catches `_id`/`version`/`updatedAt` collisions.
- [ ] No dependency on the soon-to-be-deleted byte `KVStore`.

## Risk Assessment

- Risk: generic free-function `Typed[T]` is unusual. Mitigation: document why
  (Go methods can't be generic); single switch point, well tested.
- Risk: memory `val any` type mismatch panics. Mitigation: `Get` returns a typed
  error on failed assertion, not a panic.
