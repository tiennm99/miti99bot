# Project Changelog

All significant changes, features, and fixes to miti99bot.

## [2026-04-25] MongoDB Atlas Migration Complete

**Status:** Phases 01–08 committed on `dev` branch. Dual-write ready; cutover pending operator execution (Phase 07 binding deletion).

**Summary:**
Research, planning, and 7 phases of implementation + documentation to migrate from Cloudflare KV/D1 to MongoDB Atlas M0 free tier. Achieved zero-downtime dual-write architecture with automated drift detection, backfill/reverse-backfill scripts, and comprehensive telemetry. All 733 unit tests passing.

**Plan:** `plans/260425-1945-mongodb-atlas-migration/`

**Phases:**
- Phase 01: Wrangler config, secret-leak lint, bundle-size gate
- Phase 02: MongoKVStore + memoized client + fake-mongo + tests
- Phase 03: MongoTradesStore + trading refactor + MongoSqlStore shim
- Phase 04: Dual-write wrappers + factories + retry queue + drift verifier + e2e
- Phase 05: Backfill scripts (local node, CF KV REST + wrangler d1 export)
- Phase 06: Telemetry instrumentation + soak runbook
- Phase 07: Cutover scripts + reverse-backfill + decommission helpers
- Phase 08: Final docs pass + alternatives section update

**Key artifacts:**
- `src/db/mongo-kv-store.js` — MongoDB implementation of KVStore interface
- `src/db/mongo-trades-store.js` — MongoDB trading ledger store
- `src/db/dual-kv-store.js` — dual-write wrapper for safety during migration
- `src/db/mongo-client.js` — shared memoized connection with retry logic
- `docs/using-mongodb.md` — operational runbook
- `docs/cost-tracking.md` — cost monitoring and upgrade triggers

**Reviewers noted** (not adopted):
- Maintenance-window cutover instead of dual-write (saves ~6h engineering)
- Defer trading migration (D1 free handles ~100 writes/day indefinitely)
- Single shared `kv` collection vs 12 per-module collections
- Pivot to Upstash before starting (smaller bundle, HTTP-native)

User chose full cold-start Atlas validation. If bundle size trips, `phase-07-alt-pivot.md` is ready.

**Post-cutover simplification** (Phase 07 Stage 3 — operator-driven):
- Delete dual-write layer
- Delete `cf-kv-store.js`, `cf-sql-store.js`, and D1 bindings from `wrangler.toml`
- `createStore()` returns pure `MongoKVStore`
- Cost tracking shifts to Mongo-only projections
