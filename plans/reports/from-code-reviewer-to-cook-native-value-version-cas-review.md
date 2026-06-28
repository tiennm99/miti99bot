# Code Review: Mongo native value + version CAS (Phase 1 & 2)

Branch `feature/selfhosted`, uncommitted. Review only — no files modified.

## Scope
- Files: kv_store.go, memory_kv.go, prefix.go, mongodb_kv.go, mongodb_value_codec.go (new), dynamodb_kv.go, keys.go (new), coin/gold portfolio.go, lolschedule/cron.go, main.go, CI/Makefile; 5 firestore files deleted.
- Verified independently: `go build ./...` clean, `go vet ./...` clean, hermetic `go test` for storage + coin/gold/lolschedule all PASS.
- Probed codec edge cases with a throwaway test (nested arrays, null/bool/float, big ints, empty {} / [], dup keys, unicode, trailing garbage, integral floats).

## Overall Assessment
Solid, well-reasoned implementation. The version-CAS contract and native-value codec are correct for the value shapes this bot actually stores, and the legacy dual-read / adopt-at-v0 path is sound. All 5 acceptance criteria hold. No blocking defects found. The findings below are correctness edge cases (low real-world risk for current data) and stale comments.

## Acceptance Criteria — Verdict

1. **Versioned CAS** — PASS. Concurrent create is single-winner: `expected==0` filter `{_id, $or[version absent, version==0]}` + upsert means N-1 racers fall through to an insert on the same `_id` and hit the unique `_id` index → duplicate-key → ErrConflict (`IsDuplicateKeyError`). Stale version → MatchedCount 0 → ErrConflict. Legacy version-less doc adopted at v0 with no spurious conflict. Tests `Versioned`, `AdoptsLegacyDoc`, `ConcurrentCreate` cover all three. No double-write path: an already-versioned doc (v>=1) fails the v0 filter and the upsert insert collides → ErrConflict, never a silent overwrite.
2. **Native value** — PASS. JSON object/array → native bson.M/bson.A; non-JSON (date string) → string; both round-trip; `decodeValue` dual-reads string / bson.M/A/D / bson.Binary / []byte legacy shapes.
3. **Int64 fidelity** — PASS for in-range int64. `json.Number` codec keeps integral numbers as BSON int64; `Int64Fidelity` test + my probe confirm `1719500000000` and `9223372036854775807` survive. See Finding 1 for the out-of-int64-range caveat.
4. **Bulk interface unchanged** — PASS. `KVStore` untouched; `VersionedStore` is a separate opt-in interface. GetJSON/PutJSON/List consumers compile and pass.
5. **No firestore refs / build / vet** — PASS for code; firestore only survives in comments + one stale doc-comment string (Finding 4). Build/vet clean.

## Findings

### Medium

**M1. Integers beyond int64 range silently lose precision (regression vs old byte-store).**
`numbersToBSON` falls back to `Float64()` when `Int64()` overflows. Probe: `{"toobig":9223372036854775808}` → stored/returned as `9223372036854776000`. The previous byte-blob store round-tripped such values exactly. No current consumer stores integers > 2^63-1 (portfolios use float64 amounts + int64 unix-nanos timestamps), so impact today is nil — but it is a real behavioral change worth a one-line note in the codec doc so a future caller isn't surprised. Not blocking.

### Low

**L1. `decodeValue`/round-trip drops byte-exactness for native JSON — confirmed no consumer depends on it.** Verified the only two raw `kv.Get` consumers: `lolschedule/cron.go` compares a bare date string (`2026-06-28`, not JSON → string-fallback path, byte-exact) and `lolschedule/subscribers.go` immediately `json.Unmarshal`s the bytes (semantic, not byte, comparison). Every other consumer uses `GetJSON`. Phase 1 removed the only byte-CAS user. So key reordering / whitespace loss from native re-serialization is safe. Noting it so the invariant ("no consumer byte-compares Get output") is recorded — re-check if a future feature stores a signature/hash blob and reads it back for exact compare.

**L2. Integral JSON floats normalize (`1.0` → `1`).** A JSON `1.0` is stored as int64 and returns as `1`. Harmless for the typed-struct consumers here (unmarshals into float64 fields fine; into `any` yields float64(1), same value). Only matters if a consumer type-asserts `float64` out of `map[string]any` and requires the source to have been a float — none do today.

**L3. Memory vs Mongo legacy-adopt semantics differ, but divergence is unobservable.** Mongo `GetVersioned` reports v0 for a version-less doc and `PutVersioned(0)` adopts it; memory `PutVersioned(0)` returns ErrConflict if the key exists at all. Memory never produces a version-less existing entry (Put/PutVersioned always set version>=1), so `GetVersioned` never returns v0 for an existing memory key — the legacy-adopt branch is unreachable in memory. Behaviorally consistent in practice; flagging only so the asymmetry is documented and a future memory-store change doesn't quietly break parity.

**L4. DynamoDB live mode now breaks versioned writes (by design, but still operator-selectable).** `KV_PROVIDER=dynamodb` remains a live switch case in `main.go`, but DynamoDB no longer implements `VersionedStore`. If selected at runtime, every `UpdatePortfolio` / `claimDailyPush` fails fast with "storage does not support versioned portfolio updates" / unsupported. This matches the stated "DynamoDB is migrate-only" intent and fails loudly (no data corruption), but the runtime path still lets an operator pick a backend that can't serve writes. Consider rejecting `dynamodb` in `buildProvider` outside the migrator, or documenting it as read/migrate-only. Not blocking.

### Stale comments / docs (cosmetic, non-blocking)

- `cmd/server/main.go:301` — `KVProvider` doc comment still lists `"firestore"` as a valid value; the switch no longer has that case.
- `cmd/migrate-dynamo-to-mongo/main.go:15,144-145` — comments claim Put writes are "byte-identical" to the source value. After Phase 2, JSON objects/arrays are re-encoded to native BSON (not byte-identical). Round-trip correctness is preserved; the wording is now wrong.
- `internal/modules/lolschedule/cron.go:138-141` and `gold/portfolio_test.go:223` — comments/names still say "CompareAndSwap" / "every real KV backend implements CompareAndSwapStore". The type is gone (now `VersionedStore`, and DynamoDB no longer implements it). Per the project rule against stale audit/contract labels in comments, refresh these to reference versioned CAS.

## Edge cases probed (codec)
Empty `{}`/`[]`, nested arrays, null/bool, unicode (`chào 日本語`), leading whitespace, `-9007199254740993` → all round-trip cleanly. Duplicate keys → last-wins (matches `encoding/json`). Trailing garbage (`{"a":1} trailing`), bare scalars (`123`, `true`, `null`, `"x"`), and truncated (`{`, `[1,2`) → all correctly fall to string-fallback (byte-exact preserved, and `GetJSON` on a malformed value would error identically to before).

## Unresolved Questions
1. M1: acceptable to accept >int64 precision loss permanently, or add an explicit codec note? (No current caller hits it.)
2. L4: should `buildProvider` refuse `KV_PROVIDER=dynamodb` for the live server, or is the fail-fast-at-write behavior intended to stay?

---

Status: DONE_WITH_CONCERNS
Summary: Phase 1/2 are correct and all 5 acceptance criteria hold; no blocking defects. Concerns are low-risk fidelity edges (>int64 precision, integral-float normalization) plus stale firestore/CompareAndSwap comments to refresh.
Concerns: M1 (>int64 precision loss vs old byte-store, no current caller); L4 (DynamoDB live mode breaks versioned writes by design but stays operator-selectable); stale comments in main.go, migrator, cron.go, gold test.
