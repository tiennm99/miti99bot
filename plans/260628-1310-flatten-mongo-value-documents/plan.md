---
title: "Flatten Mongo value documents"
description: "Remove Mongo's generic value envelope for JSON object values, keep KVStore callers stable, and store Mongo metadata in queryable root fields."
status: superseded
supersededBy: 260628-1318-mongo-native-typed-stores
priority: P2
branch: "feature/selfhosted"
tags: [database, mongodb, refactor, storage, tdd]
blockedBy: [260627-1849-selfhost-coolify-mongodb]
blocks: []
created: "2026-06-28T06:10:29.184Z"
createdBy: "ck:plan"
source: skill
---

# Flatten Mongo value documents

> **Superseded (2026-06-28)** by
> [`260628-1318-mongo-native-typed-stores`](../260628-1318-mongo-native-typed-stores/plan.md).
> User chose the full Mongo-native direction this plan deferred: delete the
> `KVStore` abstraction in favor of typed stores, MongoDB-only runtime (memory
> kept for tests), and drop the legacy `value` dual-read + in-place Mongo
> migrator (Atlas is empty until cutover, so the DynamoDB→Mongo migrator writes
> the final flattened shape directly). Kept for history; do not implement.

## Overview

Change MongoDB's on-disk KV document shape from a generic envelope:

```javascript
{ _id, value: { ... }, version, updatedAt }
```

to a more Mongo-native root-document shape for JSON object values:

```javascript
{ _id, ...payloadFields, version, updatedAt: ISODate(...), schemaVersion }
```

Keep the public `KVStore` / `VersionedStore` interfaces stable so modules keep
using `GetJSON`, `PutJSON`, `GetVersioned`, and `PutVersioned`. This plan is a
storage-representation refactor, not a full rewrite to typed repositories.

Non-object values cannot be Mongo root documents. New writes for arrays/scalars
use a small reserved `_payload` fallback with `payloadKind`, but **new writes
must not use the old `value` field**. Legacy docs with `value` remain readable
and are rewritten to the new shape by normal updates or by the in-place
migration tool.

## Scope Challenge

- Existing code: `MongoKVStore` already owns all Mongo encoding/decoding and
  version CAS; `mongodb_value_codec.go` already preserves `int64` via
  `json.Number`; modules do not need direct Mongo access.
- Minimum changes: storage codec + raw-shape tests + migrator/docs. Defer
  per-module typed repositories until there is a real query/report need.
- Complexity: touches storage, one migration command, docs, and tests. Avoid
  changing module handlers or public storage interfaces.
- Selected mode: HOLD SCOPE. Refactor the persisted Mongo shape; do not expand
  into a full domain repository rewrite.

## Design Decision

Chosen approach: **root-payload KV documents**.

Why not full typed repositories now: it would touch most modules, duplicate
test plumbing, and remove the memory backend value before there is a query need.
Why not keep `value`: it defeats the user's Mongo document-design goal and
keeps domain fields one level away from simple Compass/query/index use.

Reserved root fields:

| Field | Purpose |
|---|---|
| `_id` | Module-local KV key, unchanged |
| `value` | Legacy envelope field; forbidden in new root payloads |
| `version` | Optimistic-lock token, unchanged semantics |
| `updatedAt` | BSON Date (`time.Time`), not unix nanos |
| `schemaVersion` | Storage document shape version |
| `payloadKind` | Only needed for non-object fallback or collision fallback |
| `_payload` | Array/scalar payload fallback; never used for ordinary JSON objects |

Mongo field-name guard: object payloads also fall back to `_payload` if any
flattened key contains the null character, contains `.`, or starts with `$`.
MongoDB permits `.` and `$` in modern versions but documents restrictions around
them; fallback keeps normal query/index/schema behavior predictable.
Reference: https://www.mongodb.com/docs/manual/core/document/#field-names

Decoder rules:

1. If old `value` exists, decode using the legacy path.
2. Else if `_payload` exists, decode `_payload` according to `payloadKind`.
3. Else reconstruct the JSON object from all non-reserved root fields.

Writer rules:

1. JSON object without reserved-field collision writes payload fields at root.
2. JSON array, scalar, invalid JSON, or object with reserved-field collision
   or unsafe Mongo field names writes `_payload` + `payloadKind`.
3. Writes replace the whole document (except the `_id` key value is preserved in
   the replacement) so stale old payload fields cannot survive an overwrite.
4. `PutVersioned` keeps single-document CAS semantics by filtering on `_id` and
   `version`.
5. Plain `Put` must still bump `version`; implement as a bounded versioned
   write loop or a carefully tested aggregation update pipeline. Prefer the
   simpler loop unless tests prove unacceptable contention.

## Cross-Plan Dependencies

| Relationship | Plan | Status | Rationale |
|---|---|---|---|
| Blocked by | `260627-1849-selfhost-coolify-mongodb` | in-progress | Created the Mongo provider, migrator, and self-host runtime this refactor modifies. |
| Supersedes storage choice from | `260628-1113-mongo-native-value-documents` | completed | Keeps its version CAS + native BSON goal, but replaces the `value` envelope with root payload fields. |

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Schema Contract and Regression Tests](./phase-01-schema-contract-and-regression-tests.md) | Pending |
| 2 | [Root Document Storage Encoding](./phase-02-root-document-storage-encoding.md) | Pending |
| 3 | [Migration and Documentation](./phase-03-migration-and-documentation.md) | Pending |
| 4 | [Verification and Rollout](./phase-04-verification-and-rollout.md) | Pending |

## Dependencies

- Existing Mongo provider and version-CAS code from
  `plans/260628-1113-mongo-native-value-documents/`.
- Research report:
  `plans/reports/research-260628-0605-mongodb-document-design-standards.md`.
- MongoDB official guidance used in the research report:
  - https://www.mongodb.com/docs/manual/data-modeling/
  - https://www.mongodb.com/docs/manual/data-modeling/best-practices/
  - https://www.mongodb.com/docs/manual/core/write-operations-atomicity/
  - https://www.mongodb.com/docs/manual/core/index-ttl/

## Acceptance Criteria

- [ ] New JSON object writes contain no top-level `value` field; payload fields
  live at root with `_id`, `version`, `updatedAt`, and `schemaVersion`.
- [ ] Payload objects with top-level `value`, reserved metadata keys, null
  characters, `.`, or `$`-prefixed keys use `_payload` fallback and round-trip.
- [ ] New array/scalar/non-JSON writes contain no old `value` field; they use
  `_payload` + `payloadKind` and round-trip byte-equivalent where required.
- [ ] `updatedAt` stores as BSON Date / Go `time.Time`, not int64 nanos.
- [ ] Legacy docs with `value` as string, binary, object, or array still read.
- [ ] Overwriting a document removes stale payload fields from prior values.
- [ ] Versioned writes remain single-winner under concurrent create/update.
- [ ] A Mongo in-place migration can rewrite existing `value` docs to the new
  shape without changing logical values or document counts.
- [ ] `make test`, `make vet`, and Mongo integration tests pass.

## Not In Scope

- Replacing every module's `KVStore` usage with typed Mongo repositories.
- Adding Mongo schema validation/indexes for every module.
- Deleting DynamoDB migrator support.
- Changing Telegram bot behavior or command outputs.

## Validation Log

### Session 1 — 2026-06-28
**Trigger:** `/ck:plan validate /config/workspace/tiennm99/miti99bot/plans/260628-1310-flatten-mongo-value-documents/plan.md`
**Questions asked:** 0 — validation found technical plan gaps with clear repo/doc evidence; no user-facing trade-off needed.

#### Plan Recap

- Refactor Mongo on-disk KV documents from `{ _id, value, version, updatedAt }`
  to root payload fields.
- Keep `KVStore` / `VersionedStore` caller contracts stable.
- Keep legacy `value` docs readable.
- Use `_payload` only for non-object or unsafe/colliding object payloads.
- Store `updatedAt` as BSON Date.
- Add an in-place Mongo schema migration and rollout guardrails.

#### Verification Results

- **Tier:** Standard (4 phases)
- **Claims checked:** 32
- **Verified:** 29 | **Failed:** 3 | **Unverified:** 0

Failures fixed in this validation:

1. [Contract Verifier] Reserved fields omitted `value`. Evidence:
   `internal/storage/mongodb_kv.go:19` uses `mongoValueField = "value"` and
   `decodeValue` treats any top-level `value` as legacy payload
   (`internal/storage/mongodb_kv.go:46`). A flattened payload containing
   top-level `value` would violate the no-`value` contract and decode wrong.
2. [Fact Checker] Field-name guard omitted null/`.`/`$` cases. Evidence:
   MongoDB documents forbid null in field names and document restrictions for
   `.`/`$` field names: https://www.mongodb.com/docs/manual/core/document/#field-names.
   The plan now falls back to `_payload` for those keys.
3. [Contract Verifier] Migration rewrite via `PutVersioned` lacked conflict
   retry handling. Evidence: `PutVersioned` returns `ErrConflict` on version
   mismatch (`internal/storage/kv_store.go:35`) and concurrent app writes are
   possible after deploying the dual-read/new-write binary. Phase 3 now requires
   re-read/skip/retry behavior.

#### Confirmed Decisions

- Keep `value` reserved forever for backward-compatible legacy decode.
- Use `_payload` fallback for reserved keys and Mongo-awkward field names.
- Keep migration writes through storage encoding and handle conflicts by
  re-reading, skipping already-migrated docs, or retrying.

#### Impact on Phases

- Phase 1: add tests for payload key `value`, null/`.`/`$` field names, and
  migration conflict setup.
- Phase 2: update reserved-field and unsafe-field encoder rules.
- Phase 3: add conflict retry semantics to the migration command.

### Whole-Plan Consistency Sweep

- Files reread: `plan.md`, all four `phase-*.md` files.
- Decision deltas checked: 3 (`value` reserved, unsafe field fallback, migration conflict retry).
- Reconciled stale references: 6.
- Unresolved contradictions: 0.

## Open Questions

None.
