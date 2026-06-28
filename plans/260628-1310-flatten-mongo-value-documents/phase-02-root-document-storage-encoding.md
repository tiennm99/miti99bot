---
phase: 2
title: "Root Document Storage Encoding"
status: pending
priority: P2
dependencies: [1]
effort: "L"
---

# Phase 2: Root Document Storage Encoding

## Overview

Implement the new root-payload Mongo encoding in `MongoKVStore` while preserving
all caller-facing storage interfaces. Writes should stop producing `value`,
reads should support both new root docs and legacy `value` docs.

## Requirements

- Functional: `Put`, `PutJSON`, `PutVersioned`, `Get`, `GetJSON`,
  `GetVersioned`, `Delete`, and `List` signatures remain unchanged.
- Functional: JSON object values flatten to root unless they collide with
  reserved metadata fields, the legacy `value` field, or unsafe Mongo field
  names.
- Functional: arrays/scalars/non-JSON values use `_payload` and `payloadKind`,
  never the old `value` field.
- Functional: `updatedAt` is stored as `time.Time`.
- Functional: `version` increments on plain and versioned writes.
- Non-functional: no stale payload fields after overwrite.

## Architecture

Split the codec into document-level helpers:

```go
type payloadKind string

const (
    payloadKindObject payloadKind = "object"
    payloadKindArray  payloadKind = "array"
    payloadKindString payloadKind = "string"
)

func encodeRootDocument(key string, val []byte, version int64, now time.Time) (bson.M, error)
func decodeRootDocument(key string, doc bson.M) ([]byte, error)
func payloadFieldsFromRoot(doc bson.M) bson.M
```

Decode order:

1. Legacy `value` field exists -> current `decodeValue` compatibility path.
2. `_payload` exists -> decode `_payload` by `payloadKind`.
3. Else -> collect all root fields except reserved metadata and marshal as JSON.

Write strategy:

- `PutVersioned(expectedVersion > 0)`: `ReplaceOne({_id:key, version:expected}, replacement(version=expected+1))`.
- `PutVersioned(expectedVersion == 0)`: replace/upsert with filter matching
  `{_id:key, version missing or 0}`, duplicate key -> `ErrConflict`.
- `Put`: implement as a bounded loop over `GetVersioned` + `PutVersioned` so it
  overwrites unconditionally while still removing stale fields and bumping
  version. Use existing retry style (`portfolioUpdateAttempts` is 5) or a
  small storage-local constant.

Use `ReplaceOne`, not partial `$set`, for root-payload writes. Partial updates
are the foot-gun: `{a:1,b:2}` overwritten by `{a:3}` would otherwise leave `b`.

## Related Code Files

- Modify: `internal/storage/mongodb_kv.go` — write/read methods and version
  replacement logic.
- Modify: `internal/storage/mongodb_value_codec.go` — convert byte payload to
  flattened root docs and reconstruct JSON.
- Modify: `internal/storage/mongodb_kv_test.go` — make Phase 1 tests pass.
- Read: `internal/storage/memory_kv.go` — keep interface behavior aligned.
- Read: `internal/storage/dynamodb_kv.go` — ensure migrate-only DynamoDB stays
  unaffected.

## Tests Before

- Run Phase 1 Mongo tests and confirm failure before implementation.
- Run hermetic storage tests to ensure no memory/DynamoDB API changes are
  required.

## Refactor

1. Rename legacy `decodeValue` to make compatibility explicit, e.g.
   `decodeLegacyValueField`.
2. Add reserved-field constants and helper `isMongoRootMetadataField`.
3. Add root-document encoder:
   - trim/decode JSON with `json.Decoder.UseNumber`;
   - object -> root fields if no reserved/legacy `value` collision and no
     unsafe key containing null, containing `.`, or starting with `$`;
   - array -> `_payload` + `payloadKind:"array"`;
   - scalar/non-JSON -> `_payload` + `payloadKind:"string"` or scalar kind;
   - always add `_id`, `version`, `updatedAt`, `schemaVersion`.
   <!-- Updated: Validation Session 1 - reserve legacy value + unsafe Mongo field names -->
4. Add root-document decoder:
   - legacy `value` first;
   - `_payload` fallback second;
   - root object reconstruction third.
5. Replace `$set`/`$inc` writes with whole-document replacement in
   `PutVersioned`.
6. Rework `Put` to bump versions and replace whole docs without exposing
   conflicts to ordinary callers unless retries are exhausted.
7. Keep `Delete` and `List` unchanged.
8. Tighten error messages so malformed root docs name the module/key and
   missing payload reason.

## Tests After

- `go test ./internal/storage`
- `MONGODB_TEST_URL=mongodb://127.0.0.1:27017 go test ./internal/storage -run 'MongoKVStore'`
- `go test ./internal/modules/coin ./internal/modules/gold ./internal/modules/lolschedule`

## Success Criteria

- [ ] All Phase 1 tests pass.
- [ ] No new write path emits `value`.
- [ ] Payload objects with key `value` use `_payload` fallback, not root
  flattening.
- [ ] Payload objects with null/`.`/`$` field names use `_payload` fallback.
- [ ] Legacy `value` docs continue to read.
- [ ] Plain `Put` and `PutVersioned` both bump `version`.
- [ ] Concurrent create/update tests keep exactly-one-winner semantics.
- [ ] Stale payload fields are removed on overwrite.

## Risk Assessment

- Risk: bounded `Put` loop can conflict under extreme contention. Mitigation:
  keep retries small but adequate; if tests show trouble, switch to a
  single-write aggregation pipeline with `$replaceRoot` and `$literal` payload
  expressions.
- Risk: root reconstruction accidentally includes metadata fields. Mitigation:
  centralize reserved field list and test it.
- Risk: `time.Time` changes raw `updatedAt` type. Mitigation: no app reader uses
  it today; integration tests assert new date behavior.
- Risk: malformed existing docs without `value` or root payload become harder
  to diagnose. Mitigation: descriptive errors in decoder.
