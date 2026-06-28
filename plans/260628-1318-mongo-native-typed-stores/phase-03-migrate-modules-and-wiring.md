---
phase: 3
title: "Migrate modules + wiring; delete KVStore"
status: done
priority: P2
dependencies: [2]
effort: "XL"
---

# Phase 3: Migrate modules + wiring; delete KVStore

## Overview

Switch every module from the byte `KVStore` to typed `DocStore[T]`, rewire
`Deps`/registry/server, and delete the old byte abstraction and DynamoDB runtime
backend. This is the largest phase; do it module-by-module with tests green
after each.

## Requirements

- `Deps.KV storage.KVStore` → `Deps.Store storage.Collection`.
- `registry.Build` takes `storage.Provider` instead of `storage.KVProvider`.
- Each module builds typed stores via `storage.Typed[T](deps.Store)`.
- CAS behavior preserved for coin, gold, lolschedule last-push.
- `lolschedule` subscribers + last-push wrapped in named structs.
- No remaining references to `KVStore`, `VersionedStore`, `PutJSON`, `GetJSON`,
  `kv.Put`/`kv.Get` byte calls, or the DynamoDB runtime backend.
- `cmd/server/main.go` `buildProvider` returns a Mongo `Provider` (or memory for
  `MODULES=`/no-`MONGO_URL` local runs); the `dynamodb` runtime branch is removed.

## Per-module migration map

| Module | Keys / payload types | Store ops | Notes |
|---|---|---|---|
| coin | `Portfolio` (per user) | Get/PutVersioned (CAS loop) | typed CAS via `DocStore[Portfolio]` |
| gold | `Portfolio` (per user) | Get/PutVersioned (CAS loop) | same |
| stock | `Portfolio` (per user) | Get/Put | no CAS today; keep plain Put |
| wordle | `GameState`, `Stats` | Get/Put | two typed views, disjoint key prefixes |
| loldle | `gameState`, `stats`, `roundConfig` | Get/Put/Delete | three typed views |
| twentyq | `GameState`, `Stats` | Get/Put/Delete | two typed views |
| lolschedule | `subscribers` (wrap `[]Subscriber`), `lastPush` (wrap date string) | List/Put + CAS on last-push | **wrap array+scalar in named structs** |
| stats | counter entry struct(s) | Get/Put/List | List-heavy (`views.go`) |
| misc | `lastPing` struct | Get/Put | |
| util | none | — | no storage |

`lolschedule` wrappers:

```go
type subscribersDoc struct { Subscribers []Subscriber `bson:"subscribers" json:"subscribers"` }
type lastPushDoc    struct { Date string `bson:"date" json:"date"` }
```

`claimDailyPush` uses `DocStore[lastPushDoc].Get`/`PutVersioned` for the
single-winner daily claim (replaces the current `VersionedStore` byte path).

## Related Code Files

- Modify: `internal/modules/module.go` (Deps.Store), `registry.go` (Build signature, `Collection(name)`).
- Modify: every module's storage-touching file (see map) + their factories
  (`coin.go`, `gold.go`, … `New(deps)` build typed stores).
- Modify: `cmd/server/main.go` `buildProvider` (Mongo|memory only).
- Modify: all module `*_test.go` that construct stores (use `MemoryProvider`/`Typed`).
- Delete: `kv_store.go`, `kv_provider.go`, `memory_kv.go`, `memory_kv_test.go`,
  `dynamodb_kv.go`, `dynamodb_provider.go`, `dynamodb_kv_test.go`,
  `dynamodb_provider_test.go`, `invalid_store.go`.

## Tests Before / After (per module)

For each module: update its tests to build a `MemoryProvider`, get
`deps.Store = provider.Collection(name)`, then run existing behavior assertions.
Run `go test ./internal/modules/<m>/...` green before moving on. Order:
misc → stock → wordle → loldle → twentyq → coin → gold → stats → lolschedule
(simplest first; CAS + List modules last).

After all modules:

```sh
go build ./...
go test ./internal/modules/...
grep -rn "KVStore\|VersionedStore\|PutJSON\|GetJSON\|\.For(" internal/ cmd/ # expect: none (except migrator handled in Phase 4)
```

## Implementation Steps

1. Add `Deps.Store storage.Collection`; change `registry.Build` to take
   `storage.Provider` and set `Deps.Store = provider.Collection(name)`.
2. Migrate modules one at a time per the order above; keep the suite green.
3. For coin/gold, replace the `kv.(VersionedStore)` assertion + byte CAS loop
   with `DocStore[Portfolio]` Get/PutVersioned (same retry count).
4. For lolschedule, introduce the wrapper structs and migrate subscribers (List +
   Put) and last-push (CAS).
5. Update `cmd/server/main.go`: drop the `dynamodb` runtime branch; auto-detect =
   Mongo when `MONGO_URL` set, else memory (warn). Keep secret-safe logging.
6. Delete the dead files listed above; ensure `go build ./...` is clean.
7. Run full module + storage suites.

## Success Criteria

- [ ] All modules compile and pass tests using `DocStore[T]`.
- [ ] No `KVStore`/`VersionedStore`/`PutJSON`/`GetJSON` references outside the migrator.
- [ ] DynamoDB runtime backend deleted; `dynamodb_client.go` retained for migrator.
- [ ] coin/gold/lolschedule CAS behavior unchanged (tests prove single-winner).
- [ ] lolschedule subscribers/last-push persist as named root fields.
- [ ] `cmd/server` builds a Mongo-only runtime (memory fallback for local/no-DB).

## Risk Assessment

- Risk: large blast radius. Mitigation: strict module-by-module order, suite
  green between each; no behavior changes, only the store type.
- Risk: a module silently relied on byte-identical round-trip. Mitigation: typed
  structs already define the JSON contract; assert decoded values, not bytes.
- Risk: stats List semantics differ. Mitigation: `List(prefix)` ported verbatim;
  reuse stats' existing key prefixes.
- Risk: cron Deps scoping regresses. Mitigation: keep registry's per-module Deps
  cloning; `Deps.Store` is already per-module.
