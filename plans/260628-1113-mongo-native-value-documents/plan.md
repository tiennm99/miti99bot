---
title: Mongo-native value documents + version-field CAS
description: >-
  Store Mongo KV values as native BSON documents (object/array, string fallback)
  and switch CompareAndSwap to a version-field optimistic lock, keeping the
  KVStore interface.
status: completed
priority: P2
branch: feature/selfhosted
tags:
  - storage
  - mongodb
  - refactor
  - cas
  - tdd
blockedBy:
  - 260627-1849-selfhost-coolify-mongodb
blocks: []
created: '2026-06-28T04:51:16.659Z'
createdBy: 'ck:plan'
source: skill
---

# Mongo-native value documents + version-field CAS

## Overview

Make MongoDB store each KV value as a **native BSON document** (object/array
natively; string fallback for non-JSON like the lolschedule date guard) instead
of a stringified-JSON blob, so values are expandable and queryable in
Atlas/Compass. This requires replacing the value-bytes `CompareAndSwap` with a
**version-field optimistic lock** first — byte-exact compare is the only thing
that makes native storage impossible. The generic `KVStore` interface
(Get/Put/PutJSON/GetJSON/Delete/List) is preserved, so the 12 PutJSON/GetJSON/
List consumers stay unchanged; only the 3 CAS callers (coin, gold, lolschedule)
and the storage layer change.

Approved design: `plans/reports/brainstorm-260628-1113-mongo-native-documents-report.md` (Option A).

Sequencing rationale: Phase 1 (version CAS) is representation-neutral and
independently shippable — it de-risks the concurrency change while values are
still strings. Phase 2 flips the representation to native on top of the
already-safe version lock.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Version-Field Optimistic Locking](./phase-01-version-field-optimistic-locking.md) | Completed |
| 2 | [Native BSON Value Representation](./phase-02-native-bson-value-representation.md) | Completed |

## Dependencies

- **blockedBy `260627-1849-selfhost-coolify-mongodb`** — that plan created the
  `mongodb` KVProvider, the migrator, and the deployed self-host runtime this
  refactor modifies. Its code is already committed on `feature/selfhosted`.
- Phase 2 depends on Phase 1 (native storage requires version CAS, not
  value-bytes CAS).

## Acceptance Criteria

- [ ] In Atlas/Compass, a coin/gold/stock portfolio and a game-state value
  render as **native, expandable BSON documents** (not a quoted JSON string).
- [ ] Non-JSON values (lolschedule `daily_push:last_date`) stored as a plain
  BSON string; round-trip byte-exact.
- [ ] CAS is version-based: concurrent updates yield exactly one winner, losers
  get `ErrConflict` (proven by the existing-style concurrent test against real
  Mongo, plus memory).
- [ ] coin/gold/stock portfolios + each game-state struct round-trip through
  `PutJSON`/`GetJSON` with **no numeric precision/type loss** (TDD fidelity
  tests; explicit check that no int field exceeds 2^53 or the codec preserves
  int64).
- [ ] The 12 PutJSON/GetJSON/List-only consumers compile and pass unchanged.
- [ ] memory backend + hermetic `go test ./...` (no DB) still pass; full suite
  (incl. Mongo + DynamoDB integration) green.
- [ ] Migrator writes native documents; `--verify` still matches per-module
  counts; re-migration path documented (Atlas fresh → low impact now).
- [ ] Dual-read fallback: documents written by the previous string/binary build
  still read correctly.

## Risks (carried into phases)

1. **JSON↔BSON int/double fidelity (HIGH)** — `json.Unmarshal` to a generic
   coerces all numbers to float64 → BSON double; large int64 (>2^53) would lose
   precision. Phase 2 step 1 audits struct numeric fields and picks the codec
   (accept float64 if all safe, else a `json.Number`-preserving decode). Gated
   by TDD fidelity tests.
2. **CAS contract change ripple** — `CompareAndSwapStore` → `VersionedStore`
   (memory + mongodb implement; dynamodb drops CAS; firestore backend deleted)
   + 3 callers + tests. Phase 1 isolates this.
3. **Re-migration** — migrator output representation changes; document re-run
   (Atlas currently fresh).
4. **Legacy version-less docs (HIGH, live rollout)** — docs from the deployed
   build have no `version` field; Phase 1 treats absent version as v0 +
   match-missing-or-equal so existing users' updates don't fail.

## Open Questions

None — all resolved in the Validation Log below.

## Validation Log

### Session 1 — 2026-06-28

**Verification pass (Light tier, Fact Checker, 2 phases):** Claims checked
against code. Key result — **no persisted integer exceeds 2^53**: all timestamps
are `UnixMilli` (~1.7e12) / `Unix()` seconds; `coin|gold.Portfolio.Meta.CreatedAt int64`
= UnixMilli; lolschedule cache `Ts` = UnixMilli. CAS interface
(`CompareAndSwapStore`), its 3 callers, and the backend files confirmed present.
Failures: 0.

| # | Question | Decision | Affects |
|---|----------|----------|---------|
| 1 | JSON↔BSON codec | **int64-preserving `json.Number`.** float64 is safe today (no int >2^53) but json.Number future-proofs cheaply. | Completed |
| 2 | Versioned CAS backend scope | **memory + mongodb only.** Drop the **firestore backend entirely** (legacy/unused); **dynamodb** keeps base `KVStore` only (migrate-only; drop its CAS). | Completed |
| 3 | Pre-existing version-less docs | **Treat absent `version` as v0, match missing-or-equal (upsert).** No backfill; live docs keep working through the Phase 1 deploy. | Phase 1 |

Consequence surfaced: dropping firestore requires relocating the shared helpers
it hosts (`validateKey`/`validatePrefix`/`prefixSuccessor`/`collectionNameRe`)
to a backend-neutral file — added to Phase 1.

### Whole-Plan Consistency Sweep
Re-read `plan.md` + both phase files after propagation. Reconciled: backend
scope is now "memory + mongodb implement versioned CAS; dynamodb base-only;
firestore deleted" consistently in plan risks, Phase 1 architecture/files/steps/
criteria. Codec is "int64-preserving json.Number" in plan + Phase 2. Version-less
doc handling consistent (Phase 1 architecture + risk + criteria). No unresolved
contradictions.
