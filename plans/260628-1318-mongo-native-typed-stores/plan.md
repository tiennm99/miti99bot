---
title: "MongoDB-native typed stores (replace KVStore)"
description: "Delete the generic byte-oriented KVStore abstraction; give every module a typed, Mongo-native versioned store. MongoDB is the only runtime backend (memory kept for tests). The DynamoDB→Mongo migrator stays and writes the new flattened shape directly."
status: completed
priority: P2
branch: "feature/selfhosted"
tags: [database, mongodb, refactor, storage, typed-repositories, tdd]
blockedBy: [260627-1849-selfhost-coolify-mongodb]
blocks: []
supersedes: [260628-1113-mongo-native-value-documents, 260628-1310-flatten-mongo-value-documents]
created: "2026-06-28T13:18:00.000Z"
createdBy: "manual"
source: user-direction
---

# MongoDB-native typed stores (replace KVStore)

## Overview

Replace the generic, byte-oriented `KVStore` / `VersionedStore` abstraction with
a small **typed versioned store** that persists each module's value as a native
MongoDB root document:

```javascript
{ _id, ...payloadFields, version, updatedAt: ISODate(...), schemaVersion }
```

There is no `value` envelope, no JSON-bytes round-trip, and no `_payload` /
`payloadKind` fallback. Module payload structs map straight to BSON via the
driver; the two non-object values (`lolschedule` subscribers array and last-push
date) are wrapped in named structs so they too become ordinary root fields.

This is the **full Mongo-native** direction the earlier flatten plan
deliberately deferred. Per user direction (2026-06-28):

- **Drop `KVStore`.** Modules use typed stores; the generic byte interface is removed.
- **MongoDB is the only runtime backend.** The DynamoDB *runtime* store and
  provider are deleted. The memory store is kept **only** as a test/local double.
- **Keep the DynamoDB→Mongo migrator.** It still Scans DynamoDB and now writes
  the flattened native shape directly. No in-place Mongo schema migrator and no
  legacy `value` dual-read — Atlas is empty until cutover, so neither is needed.

## Why this is now correct (and was not before)

The blocking plan `260627-1849-selfhost-coolify-mongodb` Phase 4 is
`code-complete-operator-pending`: the live cutover has **not** run, so MongoDB
Atlas holds **no production data**. The prior flatten plan's legacy-`value`
dual-read and standalone in-place migrator existed only to convert pre-existing
Mongo docs — docs that do not exist. The real and only migration is
DynamoDB → Mongo, which writes the final shape in one pass.

## Design Decision

Chosen approach: **one generic typed store, two backends.**

A single generic `DocStore[T]` (Mongo + memory implementations) instead of
hand-written per-module repositories. This honors "typed Mongo repos" without
duplicating store/CAS/list logic ten times (DRY/KISS).

```go
// internal/storage/doc_store.go
type DocStore[T any] interface {
    Get(ctx context.Context, id string) (val T, version int64, err error) // ErrNotFound
    Put(ctx context.Context, id string, val T) error                       // unconditional; bumps version
    PutVersioned(ctx context.Context, id string, expectedVersion int64, val T) error // CAS; ErrConflict
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, prefix string) ([]string, error)
}
```

Backend wiring stays backend-agnostic for modules. A provider hands each module
an opaque per-module `Collection` handle; a package-level generic constructor
turns it into a typed store:

```go
type Provider interface{ Collection(module string) Collection }
type Collection interface{ /* opaque: mongo or memory */ }
func Typed[T any](c Collection) DocStore[T] // type-switches on the concrete Collection
```

Module factory usage:

```go
portfolios := storage.Typed[Portfolio](deps.Store) // deps.Store = provider.Collection("coin")
games      := storage.Typed[GameState](deps.Store)  // same collection, disjoint key prefixes
```

Go methods cannot be generic, so the typed view is a free function, not a
`Provider` method. Multiple types per module share one collection but use
disjoint `_id` key prefixes (existing `gameKey`/`statsKey`/… helpers), so reads
never decode a doc into the wrong type.

### Stored document encoding (Mongo)

```go
type storedDoc[T any] struct {
    ID        string    `bson:"_id"`
    Version   int64     `bson:"version"`
    UpdatedAt time.Time `bson:"updatedAt"`
    Payload   T         `bson:",inline"` // payload fields hoisted to root
}
```

- `bson:",inline"` requires `T` to be a struct (or map). Module payloads already
  are, except the two `lolschedule` values, which Phase 3 wraps in named structs.
- Writes use whole-document `ReplaceOne` (never `$set`), so overwriting a value
  cannot leave stale fields from a previous value.
- `version` CAS: `PutVersioned(expected>0)` filters `{_id, version:expected}`;
  `MatchedCount==0` → `ErrConflict`. `expected==0` upserts on `{_id, version
  absent/0}`; duplicate-key → `ErrConflict`. (Same proven semantics as today's
  `MongoKVStore`, just typed.)
- Plain `Put` overwrites unconditionally and bumps `version` via a small bounded
  get-version → put-versioned retry loop (reuse existing retry-count style).
- `updatedAt` is `time.Time` (BSON Date), not int64 nanos.
- `schemaVersion` is omitted for now (single shape). Add later only if a real
  migration need appears — YAGNI.

### Reserved root field names

`_id`, `version`, `updatedAt` are reserved. A payload struct must not define BSON
tags colliding with these. This is enforced once, at compile/review time, per
payload type — not at runtime — because types are known. Phase 1 adds a
reserved-name check helper + test; no runtime `_payload` fallback exists.

## Scope Challenge

- This is intentionally a **large** refactor (user-selected maximal scope):
  storage layer rewrite + all ~10 modules + all module tests + server wiring +
  migrator + docs.
- Held back from going further: no `schemaVersion` machinery, no per-module
  bespoke repositories, no Mongo schema validators/indexes beyond the existing
  `_id` index, no change to Telegram behavior or command output.

## Backends after this change

| Concern | Before | After |
|---|---|---|
| Runtime store | memory \| dynamodb \| mongodb | **mongodb only** |
| Test / local-no-DB store | memory | memory (typed double) |
| DynamoDB | runtime backend + migrator | **migrator only** (Scan + write) |
| Generic byte `KVStore` | yes | **deleted** |

## Files removed

- `internal/storage/kv_store.go` (KVStore, VersionedStore interfaces)
- `internal/storage/kv_provider.go` (old KVProvider/MemoryProvider) — replaced
- `internal/storage/memory_kv.go`
- `internal/storage/mongodb_kv.go`, `mongodb_value_codec.go`
- `internal/storage/dynamodb_kv.go`, `dynamodb_provider.go`, `dynamodb_provider_test.go`, `dynamodb_kv_test.go`
- `internal/storage/invalid_store.go`
- `internal/storage/mongodb_kv_test.go`, `memory_kv_test.go` (rewritten as doc-store tests)

## Files kept / reused

- `internal/storage/dynamodb_client.go` (`NewDynamoDBClient`, `DynamoDBEndpointFromEnv`) — migrator only
- `internal/storage/mongodb_client.go`, `mongodb_provider.go` (reshaped to the new Provider)
- `internal/storage/keys.go`, `prefix.go` (key/prefix validation + range scan reused)

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Typed DocStore contract + memory backend](./phase-01-typed-docstore-and-memory-backend.md) | Done |
| 2 | [Mongo typed store + integration tests](./phase-02-mongo-typed-store.md) | Done |
| 3 | [Migrate modules + wiring; delete KVStore](./phase-03-migrate-modules-and-wiring.md) | Done |
| 4 | [DynamoDB→Mongo migrator + docs](./phase-04-dynamo-migrator-and-docs.md) | Done |
| 5 | [Verification + Mongo-only rollout](./phase-05-verification-and-rollout.md) | Done (code; operator runs live cutover) |

## Implementation Log

### Session — 2026-06-28 (/cook)
All 5 phases implemented. Deleted byte `KVStore`/`VersionedStore` + DynamoDB/memory
KV impls; added generic typed `DocStore[T]` (`Provider`/`Collection`/`Typed`) with
Mongo (flattened native `storedDoc[T]` via `bson:",inline"`) + memory backends.
Migrated all 10 modules + `internal/deploynotify` to typed stores; every persisted
struct (incl. nested `lolschedule` `ScheduleEvent` tree and `wordle` `LetterScore`)
carries `bson` tags == `json` names. lolschedule wraps its array/scalar values
(`subscribers`, `daily_push:last_date`) in named structs. MongoDB is the only
runtime backend (memory kept for tests/local); DynamoDB survives only as the
migrator source. Migrator rewritten to write the flattened shape via
`storage.Typed[bson.M]` + wrap rules (`cmd/migrate-dynamo-to-mongo/encode.go`).

Verification: `go vet`/`go build` clean; full `go test ./...` green hermetically
**and** in-container against real Mongo 7 + DynamoDB Local (storage integration +
migrator e2e `TestMigrateAndVerify`). code-reviewer: CHANGES_REQUESTED →
addressed (H1 nested bson tags, L1 LetterScore tags, L2 stale comment, M1
camelCase-fidelity test); CAS/parity/migrator/security verified correct.

Operator-pending (out of code scope): the live DynamoDB→Mongo cutover
(backup → migrate → verify → start Mongo-only container).

## Cross-Plan Dependencies

| Relationship | Plan | Status | Rationale |
|---|---|---|---|
| Blocked by | `260627-1849-selfhost-coolify-mongodb` | in-progress (cutover pending) | Provides Mongo provider, dynamo migrator, self-host runtime. Cutover-pending state is *why* legacy dual-read is unnecessary. |
| Supersedes | `260628-1113-mongo-native-value-documents` | completed | Keeps its version-CAS goal; replaces byte-codec values with typed root docs. |
| Supersedes | `260628-1310-flatten-mongo-value-documents` | pending | Same flatten goal; takes the full typed-repo path instead of preserving `KVStore`. |

## Acceptance Criteria

- [ ] `internal/storage` exposes a typed `DocStore[T]` with Mongo + memory impls; no `KVStore`/`VersionedStore` remain.
- [ ] All modules persist/read through `DocStore[T]`; no module imports a byte KV API.
- [ ] New Mongo docs are `{ _id, ...payloadFields, version, updatedAt(Date), }` with no `value`, no `_payload`.
- [ ] `lolschedule` subscribers + last-push are named-struct root fields (`subscribers`, `date`), not bare array/scalar.
- [ ] Overwriting a value removes stale prior fields (ReplaceOne).
- [ ] Versioned writes remain exactly-one-winner under concurrent create/update (coin/gold/lolschedule).
- [ ] MongoDB is the only runtime backend; memory store works for hermetic tests + `MODULES=` local run.
- [ ] DynamoDB→Mongo migrator writes the new flattened shape, is idempotent, supports `--dry-run`/`--verify`.
- [ ] `make vet`, `make test`, and Mongo integration tests pass; migrator e2e passes.

## Not In Scope

- Deleting the DynamoDB→Mongo migrator or its dynamo Scan path.
- Mongo schema validators / per-field indexes beyond the existing `_id` index.
- `schemaVersion` evolution machinery.
- Any change to Telegram command behavior or output.

## Open Questions

None. (Scope confirmed by user 2026-06-28: delete KVStore → typed repos; drop
legacy dual-read + in-place migrator; Mongo-only runtime, memory for tests;
keep dynamo migrate.)
