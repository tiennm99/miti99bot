---
phase: 1
title: Version-Field Optimistic Locking
status: completed
priority: P1
dependencies: []
---

# Phase 1: Version-Field Optimistic Locking

## Overview

Replace the value-bytes `CompareAndSwap` with a version-field optimistic lock,
across all backends + the 3 callers, while values are still stored as strings.
Representation-neutral and independently shippable; unblocks Phase 2 (native
storage can't use byte-exact CAS).

## Requirements

- Functional: a versioned read+swap contract — read returns value + a version
  token; write succeeds only if the stored version is unchanged, else
  `ErrConflict`. Absent key = version 0; first write must be create-only.
- Functional: concurrent writers → exactly one winner, losers `ErrConflict`
  (same guarantee as today's CAS).
- Non-functional: `KVStore` bulk interface (Get/Put/PutJSON/GetJSON/Delete/List)
  unchanged. <!-- Updated: Validation Session 1 - versioned CAS only on memory + mongodb; firestore backend dropped; dynamodb keeps base KVStore for the migrator (no CAS). -->
  Only memory + mongodb implement the versioned lock (the live backends).
  dynamodb keeps only the base `KVStore` (migrator uses Scan/Put, never CAS).
  The firestore backend is **removed entirely** (legacy, unused since self-host).

## Architecture

Replace `CompareAndSwapStore` with a versioned contract in
`internal/storage/kv_store.go`:

```go
type VersionedStore interface {
    // GetVersioned returns the value and its current version; ErrNotFound if absent.
    GetVersioned(ctx context.Context, key string) (val []byte, version int64, err error)
    // PutVersioned writes val iff the stored version still equals expectedVersion.
    // expectedVersion == 0 means "must not exist yet". ErrConflict on mismatch.
    PutVersioned(ctx context.Context, key string, expectedVersion int64, val []byte) error
}
```

Per-backend version source:
- **memory** (`memory_kv.go`): per-key `version int64` alongside the bytes;
  `PutVersioned` checks+increments under the existing mutex. `prefixedStore`
  (`prefix.go`) forwards both methods with the key prefix.
- **mongodb** (`mongodb_kv.go`): add a `version` int field to the doc.
  `GetVersioned` reads value+version; **a doc with no `version` field (written
  by the pre-refactor build) reports version 0 but still "exists"**.
  `PutVersioned` with `expectedVersion==0` → `UpdateOne(filter={_id, version:
  {$in:[0,null]} OR $exists:false}, {$set:{value,version:1,updatedAt}}, upsert:true)`
  so it upserts a new key AND adopts a legacy version-less doc without a
  spurious duplicate-key conflict; `expectedVersion>0` →
  `UpdateOne(filter={_id, version:expectedVersion}, {$set:{value,updatedAt}, $inc:{version:1}})`,
  `MatchedCount==0` → `ErrConflict`. Keeps the linearizable single-winner
  property. <!-- Updated: Validation Session 1 - legacy version-less docs treated as v0 + match missing-or-equal so existing users' updates don't fail during rollout. -->
- **dynamodb** (`dynamodb_kv.go`): **drop `CompareAndSwap`** — the migrator only
  Scans/Puts, never locks. Keeps the base `KVStore` only.
- **firestore**: **delete the backend** (`firestore_client.go`,
  `firestore_provider.go`, `firestore_kv.go` + tests), the `firestore` case in
  `buildProvider`, and the firestore go.mod deps. Legacy, unused since self-host.
  Keep the shared helpers it currently hosts (`validateKey`/`validatePrefix`/
  `prefixSuccessor`/`collectionNameRe`) by relocating them to a backend-neutral
  storage file so memory/mongodb/dynamodb keep compiling.

Callers (load → mutate → swap-by-version, retry on conflict):
- `coin/portfolio.go` `UpdatePortfolio` + `loadPortfolioForUpdate`: capture
  `version` instead of `expected []byte`; call `PutVersioned`.
- `gold/portfolio.go`: same shape.
- `lolschedule/cron.go` `claimDailyPush`: `GetVersioned` the date key, claim via
  `PutVersioned(key, version, today)`; conflict → already claimed.

## Related Code Files

- Modify: `internal/storage/kv_store.go` — replace `CompareAndSwapStore` with `VersionedStore`.
- Modify: `internal/storage/memory_kv.go`, `prefix.go`, `mongodb_kv.go` —
  implement versioned methods; drop old CAS.
- Modify: `internal/storage/dynamodb_kv.go` — drop `CompareAndSwap` (migrate-only).
- Create: `internal/storage/keys.go` (or similar) — relocate shared helpers
  `validateKey`/`validatePrefix` (from `firestore_kv.go`), `prefixSuccessor`
  (from `firestore_kv.go`), `collectionNameRe` (from `firestore_provider.go`)
  to a backend-neutral file so they survive the firestore deletion.
- Delete: `internal/storage/firestore_client.go`, `firestore_provider.go`,
  `firestore_kv.go`, `firestore_kv_test.go`, `firestore_provider_test.go`.
- Modify: `cmd/server/main.go` — remove the `firestore` case + `FirestoreProject`/
  `FirestoreEmulatorHost` config and the firestore init-timeout const.
- Modify: `go.mod`/`go.sum` — `go mod tidy` drops `cloud.google.com/go/firestore`
  + now-unused google.golang.org/api deps.
- Modify: `internal/modules/coin/portfolio.go`, `internal/modules/gold/portfolio.go`,
  `internal/modules/lolschedule/cron.go` — switch to version flow.
- Modify tests: `memory_kv_test.go`, `mongodb_kv_test.go`, `dynamodb_kv_test.go`,
  `coin/portfolio_test.go`, `gold/portfolio_test.go`, `lolschedule/cron_test.go`.
- Check: `Makefile` (drop `firestore-emulator`/`test-emulator` targets),
  `.github/workflows/ci.yml` (firestore-emulator note), `README.md` storage list.

## Implementation Steps (TDD)

1. **Tests first — lock current behavior, expressed in the new contract.** Before
   changing impls, write/convert backend tests asserting: create-only on
   version 0, conflict on stale version, success on current version, and the
   N-goroutine concurrent single-winner test (memory + mongodb). Convert the
   coin/gold concurrent-update tests to assert the same observable outcome
   (exactly one mutation wins; balances never double-applied). These fail to
   compile/pass until the impl lands — that's the red state.
2. Relocate shared helpers (`validateKey`/`validatePrefix`/`prefixSuccessor`/
   `collectionNameRe`) to a backend-neutral file; delete the firestore backend +
   its `buildProvider` case + config; `go mod tidy`. Confirm build still green.
3. Define `VersionedStore`; implement in memory + `prefixedStore`.
4. Implement in mongodb (`version` field; legacy version-less doc = v0 via
   match-missing-or-equal + upsert).
5. Drop `CompareAndSwap` from dynamodb (keeps base `KVStore`).
6. Migrate the 3 callers to load-version → mutate → `PutVersioned`.
7. Make red tests green: `make test`; `make test-mongo`; `make test-dynamodb`.

## Success Criteria

- [ ] `VersionedStore` implemented + tested on memory + mongodb.
- [ ] Concurrent version-CAS test: exactly one winner, losers `ErrConflict` (memory + real Mongo).
- [ ] A legacy doc with no `version` field is updated successfully (treated as v0); test seeds one and asserts no spurious conflict.
- [ ] dynamodb no longer implements CAS; migrator (Scan/Put) + its e2e test still pass.
- [ ] firestore backend deleted; shared helpers relocated; `go mod tidy` drops firestore deps; build green.
- [ ] coin/gold portfolio updates + lolschedule daily-push claim use version flow; their tests pass.
- [ ] Values still stored as strings (representation unchanged this phase).
- [ ] `make vet` + full `go test ./...` green; 12 non-CAS consumers untouched.

## Risk Assessment

- **Legacy version-less docs (HIGH for live rollout)** — docs from the deployed
  build have no `version` field; naive create-only CAS would conflict and fail
  existing users' coin/gold updates. Mitigation: treat absent version as v0 and
  match missing-or-equal (upsert) — explicit seeded test.
- **Helper relocation regression** — `validateKey`/`prefixSuccessor`/
  `collectionNameRe` currently live in firestore files; deleting firestore
  without relocating them breaks memory/mongodb/dynamodb. Mitigation: relocate
  first (step 2), build before proceeding.
- **Contract ripple** — interface + 3 callers + memory/mongodb. Mitigation:
  tests-first per backend; phase is representation-neutral, ships independently.
- **Conflict-loop regression** — a bug in version compare could exhaust the
  bounded retry. Mitigation: explicit conflict + success unit tests before wiring callers.
