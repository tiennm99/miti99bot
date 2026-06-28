# Brainstorm: Mongo-native documents + version-field CAS

**Date:** 2026-06-28
**Branch:** feature/selfhosted
**Modes:** (none)
**Outcome:** Approved — Option A (native value documents, keep KVStore interface, switch CAS to version field).

## Problem statement

Current Mongo storage wraps each value as a stringified-JSON blob under `value`
(`{ _id, value: "<json>", updatedAt }`). Readable but not native — can't expand
or query inside values in Atlas/Compass. User wants real, native, viewable
documents. Initial ask ("use Mongo directly, drop the KV interface") was
inverted (problem-first): the underlying goal is the native-document
*representation*, not removing the abstraction.

## Approaches evaluated

| Option | Summary | Verdict |
|---|---|---|
| **A. Native value + version CAS, keep interface** | Store value as native BSON object/array (string fallback for non-JSON). Switch CAS from value-bytes compare to a `version` field. `PutJSON`/`GetJSON`/`List` unchanged → 12 consumers untouched. | **Chosen** — ~80% of the benefit, contained cost |
| B. Typed per-module Mongo repositories | Real typed collections per module + thin interface seam. Idiomatic, fully queryable. | Rejected — big effort (~10 modules), not justified by current needs |
| C. Full Mongo-direct, no abstraction | Handlers call `*mongo.Collection`. | Rejected — loses memory/firestore/dynamodb, kills hermetic tests + no-DB local dev, needs Mongo in CI, largest rewrite |
| Keep current KV | Leave string-blob design as-is. | Rejected by user (wants native docs) |

Why not B/C: the KVStore seam is load-bearing — it gives hermetic millisecond
tests (memory backend, no DB) and the portability that made the AWS→Mongo
migration cheap. "No interface" is partly a mirage in Go: testable persistence
needs *some* seam, so removal just relocates it while sacrificing dev ergonomics.

## Chosen design (Option A)

- **Value representation (Mongo):** JSON object → native BSON object; JSON array
  → native array; bare/non-JSON (e.g. lolschedule `last_date` string) → BSON
  string fallback. `Get`/`GetJSON` reconstruct `[]byte` from the native form.
  Keep dual-read fallback for existing string/binary docs.
- **CAS → version field.** Replace value-bytes compare with load-value+version,
  swap-if-version-unchanged (standard optimistic lock). This removes the
  byte-exactness coupling that previously blocked native storage.
- **Interface:** bulk `KVStore` (Get/Put/PutJSON/GetJSON/Delete/List) signatures
  unchanged. Only the `CompareAndSwapStore` contract changes shape.

## Blast radius (verified against consumers)

- **Unchanged (12):** all `PutJSON`/`GetJSON`/`List`-only modules — conversion
  hides inside `Put`/`Get`.
- **Changed (CAS, 3):** coin/portfolio.go, gold/portfolio.go, lolschedule/cron.go
  (the last-push claim).
- **Storage layer:** `internal/storage/mongodb_kv.go` (native encode/decode +
  version), `CompareAndSwapStore` contract + 4 backend impls (memory + mongo
  live; dynamodb/firestore for parity), memory CAS.
- **Migrator:** writes native now; re-run needed if data already migrated (Atlas
  currently fresh → fine).

## Risks

1. **JSON↔BSON type fidelity (HIGH).** Native round-trip can drift int vs double
   (struct `int64` field returning as double). App data (float balances,
   in-range int timestamps < 2^53) is likely safe, but the plan MUST prove
   per-struct round-trip with tests or use a type-preserving codec. This is the
   real cost of leaving byte-blobs.
2. **CAS contract change** ripples to 4 backends + 3 callers + tests.
3. **Re-migration** of any already-migrated data (none yet → low now).
4. **Get-raw byte fidelity:** validated no consumer needs byte-identical `Get`
   of a JSON object (CAS path is being refactored; only bare-string `last_date`
   uses raw Get, and strings round-trip exact).

## Success criteria

- Values appear as native, expandable documents in Atlas/Compass.
- coin/gold/stock portfolios + game state round-trip through typed structs with
  no precision/type loss (tested).
- Concurrent CAS still single-winner (version-based); coin/gold concurrency tests
  pass.
- memory backend + hermetic `go test ./...` (no DB) still work.
- 12 PutJSON/GetJSON/List consumers compile + pass unchanged.

## Recommended next step

`/ck:plan --tdd` — this refactors critical, tested money-path code (portfolios +
concurrency). Lock current behavior with tests first, then change representation
underneath.

## Unresolved questions

- Exact JSON↔BSON codec choice (driver extJSON vs custom typed decode) — decide
  in plan after a fidelity spike on the real structs.
- Whether to keep CAS on dynamodb/firestore (parity) or drop it there (only
  memory + mongo are live) — plan decision.
