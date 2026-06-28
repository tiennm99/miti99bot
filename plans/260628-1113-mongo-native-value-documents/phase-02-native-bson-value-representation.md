---
phase: 2
title: Native BSON Value Representation
status: completed
priority: P1
dependencies:
  - 1
---

# Phase 2: Native BSON Value Representation

## Overview

Store the Mongo `value` field as a native BSON document (object/array) instead
of a stringified-JSON blob, with a string fallback for non-JSON values, so
values are expandable and queryable in Atlas/Compass. Depends on Phase 1's
version CAS (byte-exact compare is gone, so native storage is now safe).

## Requirements

- Functional: JSON object → native BSON object; JSON array → native array;
  non-JSON / bare scalar (e.g. lolschedule `daily_push:last_date`) → BSON string.
- Functional: `Get`/`GetJSON` reconstruct the caller's `[]byte`/struct from the
  native form with **no numeric type/precision loss**.
- Functional: dual-read — documents written by the prior string/binary build
  still read correctly.
- Non-functional: `KVStore` + `VersionedStore` signatures unchanged; the 12
  PutJSON/GetJSON/List consumers untouched. Conversion is confined to
  `mongodb_kv.go`.

## Architecture

### JSON ↔ BSON conversion (the fidelity-critical part)
- **Write** (`Put`/`PutVersioned` receive JSON `[]byte`): if `json.Valid` and
  the top level is `{` or `[`, decode with `json.Decoder` + `UseNumber()` into a
  generic value, then a recursive `jsonToBSON` converts `json.Number` →
  **int64 when integral and representable, else float64** (so int64 stays int64,
  not coerced to double). Store the result under `value`. Otherwise store
  `string(val)`.
- **Read** (`decodeValue`): inspect the decoded `value`:
  - `bson.M` / `bson.D` / `bson.A` (object/array) → recursive `bsonToJSON` →
    `json.Marshal`-compatible bytes (int64 → integer, double → number).
  - `string` → `[]byte(v)`.
  - `bson.Binary` / `[]byte` → legacy fallback (pre-refactor docs).
- Map-key ordering is NOT preserved on object re-serialization; acceptable —
  no byte-exact consumer remains (Phase 1 removed value-bytes CAS; all readers
  are `GetJSON`/unmarshal or the bare-string `last_date`).

### Codec decision (RESOLVED — Validation Session 1)
<!-- Updated: Validation Session 1 - int64-preserving json.Number codec chosen. -->
**Use the int64-preserving `json.Number` codec.** Verification confirmed no
persisted integer exceeds 2^53 today (all timestamps are `UnixMilli` ~1.7e12;
`CreatedAt int64` = UnixMilli; lolschedule cache `Ts` = UnixMilli), so float64
coercion would be safe *now* — but the `json.Number` codec is chosen anyway to
future-proof against any later large-int field, for trivial extra cost. Decode
with `json.Decoder.UseNumber()`; `jsonToBSON` maps `json.Number` → BSON int64
when integral and representable, else double.

## Related Code Files

- Modify: `internal/storage/mongodb_kv.go` — `jsonToBSON`/`bsonToJSON` helpers;
  `doc()` stores native-or-string; `decodeValue` handles object/array/string/
  binary; `GetVersioned` re-serializes value.
- Modify: `internal/storage/mongodb_kv_test.go` — fidelity + native-shape tests.
- Modify: `cmd/migrate-dynamo-to-mongo/` — no code change (writes through `Put`,
  now native); update `main_test.go` assertions if they inspect raw value type.
- Modify: `docs/deploy-coolify-selfhosted.md` + the self-host plan's
  `phase-01` note — value now native BSON, re-migration note.

## Implementation Steps (TDD)

1. Codec already chosen (int64-preserving `json.Number`; audit done in
   validation — no field >2^53). Proceed straight to tests.
2. **Tests first** — per-struct round-trip fidelity tests through
   `PutJSON`→Mongo→`GetJSON` for: coin/gold/stock Portfolio (balances, int qty,
   timestamps), wordle/loldle/twentyq state, stats counters, lolschedule
   subscribers (array) + `last_date` (bare string). Assert struct equality AND
   that the raw stored `value` is a native object/array (not a string) for the
   JSON cases, and a string for `last_date`. Add a dual-read test seeding a
   legacy string-value doc. These are red until the impl lands.
3. Implement `jsonToBSON`/`bsonToJSON` + wire into `doc()`/`decodeValue`/`GetVersioned`.
4. Make red tests green: `make test-mongo`; full `go test ./...`.
5. Re-run migrator e2e (DynamoDB Local → Mongo) — values land native, `--verify`
   counts match, byte round-trip via `Get` still decodes.
6. Docs: native representation + re-migration note.

## Success Criteria

- [ ] Raw stored `value` is a native object/array for JSON values, a string for
  `last_date`; verified by reading the raw doc in a test.
- [ ] Every persisted struct round-trips through `PutJSON`/`GetJSON` with no
  type/precision loss (fidelity tests green).
- [ ] Dual-read test: a seeded legacy string-value doc still decodes.
- [ ] Migrator e2e green; values native; `--verify` matches.
- [ ] `make vet` + full `go test ./...` (incl. Mongo + DynamoDB) green; 12
  non-CAS consumers untouched.
- [ ] In Atlas, a portfolio document is visibly expandable (manual confirm).

## Risk Assessment

- **Int/double fidelity (HIGH)** — see codec decision. Mitigation: int64-
  preserving `json.Number` codec + per-struct fidelity tests as the gate.
- **Map-key reordering on read** — harmless for unmarshal consumers; called out
  so no future code assumes byte-stable `Get` on objects.
- **Legacy docs** — dual-read fallback covers string/binary docs from the prior
  build; explicit test.
- **Re-migration** — if prod data was already migrated as strings, re-run the
  migrator (idempotent; overwrites to native). Atlas currently fresh → low.
