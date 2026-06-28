---
phase: 1
title: "Schema Contract and Regression Tests"
status: pending
priority: P2
dependencies: []
effort: "M"
---

# Phase 1: Schema Contract and Regression Tests

## Overview

Define the exact Mongo document contract and lock it with failing tests before
changing the codec. This phase prevents a vague "flatten value" implementation
from preserving stale fields, losing type fidelity, or silently breaking legacy
reads.

## Requirements

- Functional: specify new raw document shapes for JSON object, array, scalar,
  non-JSON string, reserved-key collision, and legacy `value` docs.
- Functional: preserve current `KVStore` and `VersionedStore` method contracts.
- Functional: prove `updatedAt` is BSON Date / Go `time.Time`.
- Non-functional: tests must be explicit enough to fail against the current
  `{ _id, value, version, updatedAt int64 }` shape.

## Architecture

Target document examples:

```javascript
// Normal JSON object payload, flattened.
{
  _id: "user:7",
  usd: 1000.25,
  assets: { BTC: 1 },
  meta: { createdAt: 1782604800000 },
  version: 4,
  updatedAt: ISODate("2026-06-28T06:10:29Z"),
  schemaVersion: 2
}

// Non-object fallback. Mongo root must be a document, so array/scalar payloads
// need a reserved payload field. This is not the old generic `value` field.
{
  _id: "daily_push:last_date",
  _payload: "2026-06-28",
  payloadKind: "string",
  version: 1,
  updatedAt: ISODate("2026-06-28T06:10:29Z"),
  schemaVersion: 2
}
```

Reserved fields: `_id`, `value`, `version`, `updatedAt`, `schemaVersion`,
`payloadKind`, `_payload`. If an object payload contains one of these keys, the
codec must preserve data by storing the whole object in `_payload` with
`payloadKind: "object"` instead of flattening. Object payloads with keys that
contain null, contain `.`, or start with `$` also use `_payload`; MongoDB
permits some of these in modern versions but documents restrictions, so fallback
keeps query/index behavior predictable. <!-- Updated: Validation Session 1 - reserve legacy value + unsafe Mongo field names -->

## Related Code Files

- Modify: `internal/storage/mongodb_kv_test.go` — raw-shape, legacy-read,
  stale-field, updatedAt, and CAS tests.
- Modify: `internal/storage/mongodb_value_codec.go` — only if test helpers need
  exported/unexported constants clarified later.
- Read: `internal/storage/mongodb_kv.go`
- Read: `internal/storage/kv_store.go`
- Read: `plans/reports/research-260628-0605-mongodb-document-design-standards.md`

## Tests Before

Add/adjust Mongo integration tests gated by `MONGODB_TEST_URL`:

1. `TestMongoKVStore_RootObjectRepresentation`
   - `PutJSON` a coin-like portfolio.
   - Raw Mongo doc has root `usd`, `assets`, `meta`.
   - Raw Mongo doc has no `value`.
   - `updatedAt` decodes as `time.Time`.
2. `TestMongoKVStore_RootObjectOverwriteRemovesStaleFields`
   - Write `{a:1,b:2}` then overwrite `{a:3}`.
   - Raw doc has `a`, no stale `b`.
3. `TestMongoKVStore_NonObjectPayloadFallback`
   - Write bare string and JSON array.
   - Raw docs use `_payload` + `payloadKind`, no `value`.
   - `Get` round-trips.
4. `TestMongoKVStore_ReservedRootFieldCollision`
   - Write object with `value`, `version`, or `updatedAt` in payload.
   - Raw doc preserves payload under `_payload`; `GetJSON` round-trips.
5. `TestMongoKVStore_UnsafeMongoFieldNameFallback`
   - Write object payloads with keys containing null, containing `.`, and
     starting with `$`.
   - Raw docs preserve payload under `_payload`; `GetJSON` round-trips.
6. `TestMongoKVStore_LegacyValueDocsStillRead`
   - Seed old docs directly with `value` as string, `bson.Binary`, object,
     and array.
   - `Get`/`GetJSON` decode all.
7. Keep/strengthen `TestMongoKVStore_PutVersioned_ConcurrentCreate`.

## Implementation Steps

1. Add named constants in tests for reserved fields expected in the new shape.
2. Add raw Mongo assertions before changing implementation.
3. Audit current persisted structs for reserved top-level key collisions and
   document the result in test comments.
4. Confirm all new tests fail for the intended reason against current code.
5. Keep existing int64-fidelity and legacy-version tests in place.

## Tests After

- Re-run the new Mongo storage tests after Phase 2 implementation.
- Ensure old native-`value` tests are either rewritten to the new contract or
  retained only as legacy-read tests.

## Success Criteria

- [ ] New raw-shape tests exist and fail before implementation.
- [ ] Legacy-read coverage includes the old `value` field.
- [ ] Reserved-field coverage includes payload key `value`.
- [ ] Unsafe Mongo field-name coverage includes null, `.`, and `$` cases.
- [ ] Stale-field removal has a dedicated regression test.
- [ ] `updatedAt` BSON Date behavior has a dedicated assertion.
- [ ] Existing version-CAS tests remain in the suite.

## Risk Assessment

- Risk: tests assert driver-specific decoded types too tightly. Mitigation:
  assert behavior and accepted BSON-decoded Go types, not exact internal map
  ordering.
- Risk: reserved-field collision behavior gets forgotten because current structs
  do not collide. Mitigation: synthetic collision test is required.
- Risk: stale-field issue appears only on overwrite. Mitigation: explicit
  overwrite test before implementation.
