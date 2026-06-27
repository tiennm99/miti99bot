---
phase: 1
title: "MongoDB Storage Provider"
status: pending
priority: P1
dependencies: []
effort: "M"
---

# Phase 1: MongoDB Storage Provider

## Overview

Add `mongodb` as a 4th `KVProvider`, modeled exactly on the existing `firestore` provider (collection-per-module isolation). Wire it into `buildProvider` and config via `KV_PROVIDER=mongodb`, `MONGO_URL`, `MONGO_DATABASE`. No module code changes — the `KVStore` interface is the only contract.

## Requirements

- Functional: implement `KVStore` (Get/GetJSON/Put/PutJSON/Delete/List) + `CompareAndSwapStore` (CompareAndSwap) backed by MongoDB.
- Functional: one Mongo collection per module (mirrors `FirestoreProvider`); document `_id` = key, field `value` = raw bytes, field `updatedAt` = timestamp.
- Functional: `List(prefix)` = range/regex query on `_id` with `begins_with` semantics; empty prefix = whole collection.
- Non-functional: behavior parity with firestore/dynamodb — same `ErrNotFound`, `ErrConflict`, key validation (reuse `validateKey`/`validatePrefix`), same `collectionNameRe` module-name guard via `invalidStore`.
- Non-functional: bounded startup connect timeout (match `dynamodbInitTimeout = 5s` style), graceful `Close()`.

## Architecture

Reuse the shared helpers already in `internal/storage`:
- `validateKey` / `validatePrefix` (in `firestore_kv.go`) — key constraints. Mongo `_id` has no `/` restriction, but reusing keeps cross-backend parity and is harmless.
- `collectionNameRe` (in `firestore_provider.go`) — module-name alphabet.
- `invalidStore` (in `invalid_store.go`) — returned for bad module names.

Document shape (parity with firestore `value`/`updatedAt`):
```
{ "_id": "<key>", "value": <BinData>, "updatedAt": <int64 nanos> }
```
Store `value` as BSON binary (`bson.Binary`) so non-UTF-8 round-trips; on read accept both binary and string (firestore does the same dual-type handling). **Store `updatedAt` as int64 unix-nanos (NOT BSON datetime)** — matches DynamoDB exactly (`dynamodb_kv.go:101`), keeps migration byte-faithful, and avoids ms-truncation if a future TTL/sort ever reads it. The migrator (Phase 4) and the provider MUST share one encoding — see Phase 4 (migrator writes through `MongoKVStore.Put`, not raw `UpdateOne`).

`CompareAndSwap` mapping — the `expected == nil` branch is a LIVE path (first write of every new coin/gold portfolio, `coin/portfolio.go:81-83`, `gold/portfolio.go:62-80`), not an edge case. Map it to a plain **`InsertOne`** and rely SOLELY on the unique `_id` index for the conflict:
- `expected == nil` → `InsertOne({_id, value, updatedAt})`; `mongo.IsDuplicateKeyError(err)` → `ErrConflict`. Do NOT use a `{value:{$exists:false}}` upsert filter (it can false-conflict on a value-less doc and muddies the contract).
- `expected != nil` → `UpdateOne({_id, value: expected}, {$set:{value,updatedAt}})`; `MatchedCount == 0` → `ErrConflict`.
`_id` is unique by default, so the absent-insert race is linearizable: exactly one `InsertOne` wins, losers get duplicate-key → `ErrConflict` → caller retry loop reloads the winner's state. This must be proven by a **blocking** concurrent-writer test (see Success Criteria), not just asserted.

`List(prefix)`: `Find({_id: {$gte: prefix, $lt: prefixSuccessor(prefix)}}, projection={_id:1})` — reuse the existing `prefixSuccessor` helper from `firestore_kv.go`. Empty prefix → `Find({})`. Avoids regex injection and uses the `_id` index.

## Related Code Files

- Create: `internal/storage/mongodb_client.go` — `NewMongoClient(ctx, uri) (*mongo.Client, error)` with connect+ping timeout; `NewMongoDatabase`. Mirror `dynamodb_client.go`.
- Create: `internal/storage/mongodb_provider.go` — `MongoProvider{ db *mongo.Database }`, `For(module)` returns `invalidStore` on bad name else `NewMongoKVStore`. Mirror `firestore_provider.go`.
- Create: `internal/storage/mongodb_kv.go` — `MongoKVStore`, all methods. Mirror `firestore_kv.go`.
- Create: `internal/storage/mongodb_kv_test.go` + `mongodb_provider_test.go` — parity tests, gated on `MONGODB_TEST_URL` (skip when unset), mirroring `dynamodb_kv_test.go` gating on `DYNAMODB_LOCAL_URL`.
- Modify: `cmd/server/main.go` — add `mongodb` case to `buildProvider`; add `MongoURL`, `MongoDatabase` to `config` + `loadConfig` (`MONGO_URL`, `MONGO_DATABASE`); update `buildProvider` doc comment + auto-detect note (mongo is explicit-only).
- Modify: `go.mod` / `go.sum` — add `go.mongodb.org/mongo-driver/v2`.
- Modify: `Makefile` — add `mongo-local` (docker `mongo:7`) + `test-mongo` target gated by `MONGODB_TEST_URL`, mirroring `dynamodb-local`/`test-dynamodb`.
- Modify: `README.md` — add `mongodb` to the storage backend list + local-run snippet.

## Implementation Steps

1. `go get go.mongodb.org/mongo-driver/v2/mongo` (and `/bson`).
2. Write `mongodb_client.go`: connect with `options.Client().ApplyURI(uri)`, `client.Ping` under a 5s context, return client; helper to get `*mongo.Database` from `MONGO_DATABASE`.
3. Write `mongodb_kv.go`: implement methods per Architecture; reuse `validateKey`, `validatePrefix`, `prefixSuccessor`; constants `mongoValueField="value"`, `mongoUpdatedAtField="updatedAt"`.
4. Write `mongodb_provider.go`: `For` guards with `collectionNameRe`, returns `db.Collection(module)`-backed store.
5. Wire `buildProvider`: `case "mongodb"`: require `MONGO_URL` + `MONGO_DATABASE` (error if missing, mirror dynamodb's `DYNAMODB_TABLE` check); construct client under timeout; closer calls `client.Disconnect`. **The startup `log.Info("storage backend", …)` line MUST log only non-secret fields — `"backend","mongodb","database",cfg.MongoDatabase`. NEVER log `MONGO_URL`** (it is `mongodb+srv://user:pass@…`; the firestore/dynamodb cases at `main.go:234-253` log a benign identifier, but the mongo equivalent is a credential). If a host is wanted for diagnostics, parse and log only the host, never the userinfo.
6. Add config fields + env reads. Mongo is explicit-only (not in the auto-detect switch) to avoid surprising Lambda.
7. Tests: replicate the firestore/dynamodb test bodies against a real Mongo (`mongo-local`), covering Get/Put/Delete/List/prefix/CAS-absent/CAS-match/CAS-conflict/ErrNotFound + cross-module isolation.
8. `make vet && make test && MONGODB_TEST_URL=mongodb://localhost:27017 make test-mongo`.

## Success Criteria

- [ ] `internal/storage` exposes `MongoProvider`/`MongoKVStore` passing the same test matrix as `DynamoDBKVStore`.
- [ ] `KV_PROVIDER=mongodb` with `MONGO_URL`/`MONGO_DATABASE` boots; missing either errors clearly at startup.
- [ ] Cross-module isolation verified (collection-per-module).
- [ ] CompareAndSwap returns `ErrConflict` on stale expected + on absent-with-non-nil-expected; succeeds on nil-expected insert.
- [ ] **(blocking)** Concurrent-writer CAS test: N goroutines race a nil-expected insert + a stale-update on the same key against a real Mongo; assert exactly one winner, losers get `ErrConflict`. Plus a "doc exists without value field" edge case.
- [ ] Startup log shows `backend=mongodb database=<db>` and does NOT contain the connection string / any credential.
- [ ] `make vet` and `make test` pass; AWS/firestore paths untouched.

## Risk Assessment

- **CAS semantics drift**: Mongo upsert race differs from DynamoDB conditional put. Mitigation: unique `_id` + `IsDuplicateKeyError` for the absent case; assert with a concurrent-writer test.
- **Value type on read**: driver may decode as binary or string. Mitigation: dual-type switch like firestore's.
- **TLS to Atlas**: `mongodb+srv://` URIs need DNS SRV + TLS. Mitigation: driver handles it; document that `MONGO_URL` is the full Atlas SRV connection string incl. credentials.
- **Driver version**: v2 API differs from v1 (`mongo.Connect` signature). Mitigation: pin v2, follow current docs.
