# Code Review — Self-host (Coolify + MongoDB) Implementation

Reviewer: code-reviewer | Date: 2026-06-28 | Branch: `feature/selfhosted`
Plan: `plans/260627-1849-selfhost-coolify-mongodb/`

## Scope
- New: `internal/storage/mongodb_{client,kv,provider}.go` (+tests), `internal/cron/scheduler.go` (+test), `cmd/migrate-dynamo-to-mongo/` (+test), `compose.yml`, `.env.example`, 2 docs.
- Modified: `cmd/server/main.go`, `internal/server/router.go`, `internal/telegram/client.go`, `internal/modules/lolschedule/cron.go`, `internal/modules/module.go`, `internal/modules/stock/{stock,handlers}.go`, `Dockerfile`, `Makefile`, `go.mod/go.sum`, `aws/telegram-commands.json`.
- Deleted: `internal/telegram/webhook.go`(+test), `internal/modules/stock/income_events.go`(+test).
- Build/vet: `go vet ./...` clean (re-run). Author reports full `go test ./...` + integration suites pass.

## Overall Assessment
Solid, disciplined change set. The implementation matches the plan's validated decisions point-for-point, and the 15 red-team findings are all materially addressed in code (verified below). No Critical or High defects found. The CAS implementation, MONGO_URL secret handling, cron lifecycle, and deletion hygiene are correct. Findings are Low/informational.

## Verification of Acceptance Criteria

1. **mongodb auto-select + boot** — PASS. `buildProvider` (main.go:227-235) selects `mongodb` when `MONGO_URL!=""` and `KV_PROVIDER` empty; explicit `KV_PROVIDER` overrides; old `AWS_LAMBDA_FUNCTION_NAME` auto-detect gone. Cron runs unconditionally (main.go:146). No `/webhook` route, no `WebhookSecret` (router.go).
2. **CAS correctness** — PASS. nil-expected → `InsertOne` + `IsDuplicateKeyError`→`ErrConflict` (linearizable single-winner via unique `_id`); non-nil → `UpdateOne` filtered on `{_id, value:Binary(expected)}`, `MatchedCount==0`→`ErrConflict` (mongodb_kv.go:133-164). value stored as `bson.Binary` generic subtype; `updatedAt` int64 unix-nanos. Matches DynamoDB encoding intent.
3. **MONGO_URL never logged** — PASS (Finding-3). Only `database` logged (main.go:266); SECURITY comments at mongodb_client.go:28-29 and config field 334. No interpolation of `cfg.MongoURL` into any log/error string found.
4. **Cron scheduler** — PASS (Finding-4). 60s local `cronTimeout` const (scheduler.go:23, not imported from server), per-fire `context.WithTimeout`, `recover()` with stack, `cron.WithLocation(time.UTC)`, bad schedule returns error → `log.Fatal` at startup (main.go:147). Panic-recovery deferred before dispatch — correct ordering.
5. **lolschedule idempotent per UTC date** — PASS (Finding-6). `claimDailyPush` CAS on `daily_push:last_date` before fan-out; loser logs + returns nil. New test `TestRunDailyPush_IdempotentPerDate` proves one-message-per-subscriber across two same-date fires. CAS-less stores fall back to plain Put (test-only path) with documented loss of atomicity.
6. **Migrator** — PASS (Findings 9/15). Read-only `Scan` only (scanTable), writes through `provider.For(pk).Put` (byte-identical value + `validateKey` per sk + upsert-by-`_id` = idempotent), `--verify` tallies per-pk via Scan vs `CountDocuments`, exits non-zero on mismatch. e2e test asserts byte-identical round-trip through the app read path.
7. **No regression to other backends / modules / /cron** — PASS. memory/firestore/dynamodb switch arms unchanged; DispatchScheduled + `/cron` HTTP path intact (router + cron_dispatcher untouched in contract); `CronHandler` doc updated only.
8. **No new lint/vet/build errors; intentional contract changes** — PASS. `server.Config` dropped `WebhookSecret`; `storage` gained `MongoProvider`. `go.mod` correctly promotes `robfig/cron/v3` and `mongo-driver/v2` to direct requires; `go vet ./...` clean.

## Critical Issues
None.

## High Priority
None.

## Medium Priority
None blocking. (Finding dispositions all confirmed in code.)

## Low Priority / Informational

- **L1 — Stale env name in a user-facing string.** `internal/deploynotify/deploy_notify.go:97` returns `"no BOT_OWNER_ID configured"` as a skip reason, but the env key was renamed to `OWNER_ID` (Session 5). This is a log/diagnostic string only (not flow control), but it now points an operator at a key that no longer exists. Pre-existing line, surfaced by the rename. Suggest: `"no OWNER_ID configured"`. Not in review scope's modified files but directly caused by this change.

- **L2 — `claimDailyPush` runs the expensive fetch before claiming on every fire.** `runDailyPush` (cron.go:182-213) calls `listSubscribers` + `GetEventsCached` before `claimDailyPush`. A losing double-fire still does the subscriber read + match fetch before no-op'ing. This is intentional and documented (cron.go:202-204: "Placed after the fetch so a transient fetch failure does not consume the day's claim") — a deliberate correctness-over-efficiency trade so a fetch failure doesn't burn the day's claim. Correct call; noting only so it isn't mistaken for an oversight. The wasted fetch is cache-backed and rare (only on genuine double-fire).

- **L3 — `decodeValue` accepts `string`/`[]byte` in addition to `bson.Binary`.** mongodb_kv.go:42-57 handles three value types. Only `bson.Binary` is ever written by this code, so the `string`/`[]byte` arms are dead for app-written data. Documented as mirroring Firestore's dual-type handling for forward-compat. Harmless; flagging as minor YAGNI surface, not a defect (it does aid migrator/raw-write robustness).

## Targeted concerns from the request

- **DeleteWebhook-on-startup safety vs manual-cutover framing.** SAFE and consistent with the plan. The plan (Session 2, line 118) explicitly makes cutover = `deleteWebhook` → start polling container; adding `DeleteWebhook` to startup is the in-code realization of that step, not a divergence. `DropPendingUpdates:false` preserves the buffered queue (lossless). It is idempotent and harmless when no webhook is set. The go-telegram polling loop (`get_updates.go`) retries with exponential backoff on errors, so even a transient 409 (webhook deletion racing the first `getUpdates`) self-heals. The pre-cutover Lambda keeps its own webhook code until `sam delete`, so rollback in the window still works. No issue.

- **Goroutine/shutdown lifecycle of `cron.Run` + `b.Start`.** Correct. Both `b.Start(rootCtx)` and the in-process scheduler bind to `rootCtx` (SIGINT/SIGTERM). On signal: `b.Start` returns when ctx cancelled; `stopCron()` (deferred) calls `c.Stop()` and blocks on the returned context until in-flight jobs finish; `srv.Shutdown` gets a fresh 15s ctx. Order of defers (`stop` → `closeProvider` → `stopCron`) runs LIFO: stopCron waits for jobs, THEN closeProvider disconnects Mongo — correct (jobs that touch Mongo finish before disconnect). One nuance: `srv.Shutdown` runs in the main flow before the deferred `stopCron`/`closeProvider`, so an in-flight cron job during shutdown still has its Mongo client until stopCron completes. Fine.

- **CAS race edge cases.** The nil-expected `InsertOne` race is the right primitive (Finding-1): unique `_id` gives exactly one winner, losers get duplicate-key→`ErrConflict`. Author reports the blocking concurrent-CAS linearizability test passes against real MongoDB 7. The non-nil path's filter-on-value is a true CAS (no read-then-write gap). No TOCTOU window. The `claimDailyPush` `Get`-then-CAS in lolschedule has a benign read-ahead (the `Get` is an optimization; the CAS is the authority), so a concurrent claim between Get and CAS still resolves to a single winner via `ErrConflict`. Correct.

- **Dangling references after deletions.** None. Grep for `IncomeEventClient`, `handleIncomeEvents`, `stock_income_events`, `WebhookHandler`, `WebhookSecret`, `NewIncomeEventClientFromEnv` finds zero references in non-test and test Go code. `aws/telegram-commands.json` had the `stock_income_events` command removed (diff -4 lines). Only the L1 log-string mention of the old env name remains.

- **Divergence from plan's validated decisions.** None found. `updatedAt` is int64 nanos everywhere; CAS absent-case is `InsertOne` (no `$exists:false` upsert); migrator writes through `Put`; health is plain-text `miti99bot ok\n` (health.go:20) matching the compose note; `/cron` 404s when `CRON_SHARED_SECRET` unset; `WithAllowedUpdates{message,callback_query}` matches prior webhook allowed_updates; gitSHA injected via Dockerfile `ARG GIT_SHA`.

## Positive Observations (risk-relevant)
- The `b.Start` polling loop's built-in error backoff means the `DeleteWebhook`-before-`Start` ordering is robust even under a race — no custom retry needed.
- Migrator writing through `Put` (not raw driver) is the correct choice for encoding parity and is verified by the e2e byte-identity assertion.
- Provider `For()` re-validates module name against `collectionNameRe` (defense-in-depth identical to firestore/dynamodb), and `invalidStore` fails closed.

## Metrics
- Type/contract changes: intentional only (`server.Config -WebhookSecret`, `storage +MongoProvider`).
- Lint/vet: `go vet ./...` clean.
- Test coverage: new tests for scheduler (fire/stop/bad-schedule/panic-recovery), mongo CAS parity + concurrent linearizability, migrator e2e + decode/sort units, lolschedule idempotency. Adequate for the change.

## Recommended Actions
1. (Low) Update `internal/deploynotify/deploy_notify.go:97` skip-reason string `BOT_OWNER_ID` → `OWNER_ID` to match the renamed env key.
2. (Optional) Consider whether the `string`/`[]byte` arms in `MongoKVStore.decodeValue` earn their keep, or trim to `bson.Binary` only with a clear error for other types (YAGNI). Non-blocking.

## Unresolved Questions
None. Plan decisions are internally consistent and the implementation tracks them.

---
Status: DONE
Summary: Self-host implementation is production-ready; all 8 acceptance criteria and all 15 red-team findings verified satisfied in code. No Critical/High issues. One Low cosmetic fix (stale `BOT_OWNER_ID` string in deploynotify skip reason).
Concerns: Only L1 (stale env-name string in a diagnostic log line); cosmetic, non-blocking.
