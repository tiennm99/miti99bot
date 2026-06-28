---
title: "Self-host miti99bot on Coolify with MongoDB Atlas"
description: "Add a MongoDB Atlas storage backend + in-process cron, containerize for Coolify docker-compose, migrate existing DynamoDB data."
status: in-progress
priority: P2
branch: "feature/selfhosted"
tags: [selfhost, coolify, mongodb, migration]
blockedBy: []
blocks: []
created: "2026-06-27T12:01:53.894Z"
createdBy: "ck:plan"
source: skill
---

# Self-host miti99bot on Coolify with MongoDB Atlas

## Overview

Run miti99bot as a long-lived container on Coolify (docker-compose) instead of AWS Lambda, using MongoDB Atlas (`MONGO_URL` + `MONGO_DATABASE`) instead of DynamoDB. Existing DynamoDB data is migrated into Atlas. At the **code** level this is additive — a 4th KV backend + a self-host run mode; the DynamoDB/Lambda code path is NOT ripped out (kept for portability and to run the migrator). At the **infrastructure** level, the deployed AWS stack is fully decommissioned after a verified cutover (Phase 5, validated decision).

Why this is low-risk: storage is already a pluggable `KVProvider` interface with 3 backends (`memory`, `firestore`, `dynamodb`); adding `mongodb` follows the exact `firestore` collection-per-module pattern. Secrets already fall back to plain env vars when `*_PARAMETER_NAME` is unset, so Coolify env vars need no code change. Two gaps to close, both with minimal code: (1) **cron** — EventBridge Scheduler triggers `/cron/{name}` today, replaced by an in-process scheduler (Phase 2); (2) **transport** — the bot runs webhook-only today, switched to **long polling** (Phase 3) via the same `go-telegram/bot` library's built-in polling mode, which removes the need for any public inbound ingress on the self-hosted box.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [MongoDB Storage Provider](./phase-01-mongodb-storage-provider.md) | Done (code + tests) |
| 2 | [In-Process Cron Scheduler](./phase-02-in-process-cron-scheduler.md) | Done (code + tests) |
| 3 | [Long-Polling Runtime + Containerize + Coolify Deploy](./phase-03-containerize-and-coolify-deploy.md) | Done (code + compose + docs); operator runs Atlas/Coolify setup |
| 4 | [Data Migration and Cutover](./phase-04-data-migration-and-cutover.md) | Migrator done (code + e2e test); operator runs the live cutover |
| 5 | [AWS Full Decommission](./phase-05-aws-decommission.md) | Runbook delivered + deploy.yml disabled; operator runs teardown (admin creds) |

## Implementation Log

### Session — 2026-06-28 (/cook full code until deployable)
All code for Phases 1-4 implemented + Phase 5 runbook delivered. `go build`, `go vet`, full `go test ./...` green. Storage Mongo parity tests (incl. the blocking concurrent-CAS linearizability gate), DynamoDB parity tests, and the migrator e2e (migrate→idempotent re-run→verify→byte-identical round-trip) all PASS against real MongoDB 7 + DynamoDB Local (run in-container; `-race` skipped only because the alpine image lacks cgo/gcc). Code review (code-reviewer subagent): 0 Critical/High, all 8 acceptance criteria verified; one Low cosmetic (stale `BOT_OWNER_ID` log string) fixed.

**Implementation decision (within plan intent):** the container calls `DeleteWebhook(drop_pending_updates=false)` on startup before `b.Start`, making the "deleteWebhook before first poll" requirement automatic + idempotent (the manual `make telegram-deletewebhook-selfhost` target remains for the explicit cutover step). Reviewer confirmed safe vs the manual-cutover framing.

**Remaining (operator-run, need live creds/infra, out of code scope):** create Atlas M0 + DB user + network access; create the Coolify resource + env vars + deploy; run the live cutover (disable EventBridge → deleteWebhook → migrate → verify → start container); execute the AWS teardown runbook with the admin profile.

## Dependencies

- Phase 2 is independent of Phase 1 (cron touches no storage).
- Phase 3 depends on 1 + 2 (container must boot with mongo + internal cron).
- Phase 4 depends on Phase 1 (Mongo document schema must be final before copying data) and Phase 3 (the polling container must exist before cutover); it is the last step before switching Telegram to the polling container.
- Phase 5 (AWS decommission) depends on Phase 4 `--verify` passing — it destroys DynamoDB, so it runs only after data is migrated and the bot is confirmed live on Coolify.

Suggested order: 1 → 2 → 3 → 4 → 5. Phases 1 and 2 can be done in parallel by separate developers (disjoint files).

## Architecture Summary

```
                 BEFORE (AWS)                         AFTER (Coolify self-host)
   Telegram ──webhook──> Lambda Function URL    Telegram <──long poll── container (outbound only)
   EventBridge Scheduler ──> /cron/{name}        in-process scheduler ──> cron.Handler
   DynamoDB (pk=module, sk=key)                  MongoDB Atlas (db / collection-per-module)
   SSM Parameter Store secrets                   Coolify env vars (plain)
   public HTTPS ingress (Function URL)           NO public ingress (polling = outbound only)
```

Same Go binary (`cmd/server`), same HTTP server on `:8080`, same module framework. Backend + cron + secret source are all selected by env vars at startup.

## Acceptance Criteria

- [ ] `MONGO_URL=… MONGO_DATABASE=…` (mongodb auto-selected, no `KV_PROVIDER`) boots, runs the cron scheduler by default, and consumes updates via long polling (the only transport) with persistent storage; no `/webhook` route, no webhook secret.
- [ ] `make test` green; new mongo provider has parity tests with the firestore/dynamodb suites.
- [ ] Self-hosted container fires the lolschedule daily push at 01:00 UTC (08:00 ICT) without EventBridge.
- [ ] `docker compose up` (Coolify) brings the bot live via long polling with NO public domain/ingress (outbound-only); `/` health check passes internally; exactly 1 replica.
- [ ] All existing prod DynamoDB items present in Atlas with identical values; migration is idempotent + verifiable by per-module counts.
- [ ] After cutover, ALL miti99bot AWS resources are deleted — CloudFormation stack AND the manually-created SSM secrets, GitHub OIDC role, and SAM S3 bucket; `app=miti99bot` tag sweep is empty and Cost Explorer trends to $0.
- [ ] `.github/workflows/deploy.yml` is removed/disabled so `main` pushes no longer recreate the AWS stack.

## Red Team Review

### Session — 2026-06-27
**Findings:** 15 (15 accepted, 0 rejected) — 3 reviewers (security/secrets, assumptions, failure-modes), all findings carried `file:line` evidence.
**Severity breakdown:** 2 Critical, 6 High, 7 Medium.

| # | Finding | Severity | Disposition | Applied To |
|---|---------|----------|-------------|------------|
| 1 | nil-expected CAS is a live first-write path; `$exists:false` upsert is wrong primitive → use `InsertOne` + unique-`_id` + blocking concurrent test | Critical | Accept | Phase 1 |
| 2 | Rollback loses ALL post-cutover writes; no reverse path → state true RPO / reverse migrator | Critical | Accept | Phase 4 |
| 3 | `buildProvider` log line would leak `MONGO_URL` creds → log only `database`, never URL | High | Accept | Phase 1 |
| 4 | Cron dispatcher symbol misdescribed (`DispatchScheduled(ctx,name,reg)`, no timeout/recover/log) → scheduler adds own | High | Accept | Phase 2 |
| 5 | Dockerfile omits `gitSHA` → `deploynotify` silently dead → inject build arg or document disabled | High | Accept | Phase 3 |
| 6 | Cron double-fire (EventBridge-live + rolling deploy; push non-idempotent) → disable schedule before internal cron + last-push-date guard | High | Accept | Phase 2, Phase 4 |
| 7 | Atlas `0.0.0.0/0` = public DB surface vs project boundary → egress-IP allowlist + least-priv user default | High | Accept | Phase 3 |
| 8 | Cutover write-loss gap; ambiguous webhook-unset → mandatory `deleteWebhook`→migrate→verify→`setWebhook` + `pending_update_count` | High | Accept | Phase 4 |
| 9 | Migrator raw `UpdateOne` diverges from `Put` encoding → write through `Put`; store `updatedAt` int64 | Medium | Accept | Phase 1, Phase 4 |
| 10 | Stray `*_PARAMETER_NAME` fatal off-AWS → `.env.example` lists all six "leave UNSET" + boot criterion | Medium | Accept | Phase 3 |
| 11 | `/` is plain text not JSON; `-healthcheck` flag doesn't exist → Coolify HTTP monitor, drop bogus CMD | Medium | Accept | Phase 3 |
| 12 | `/cron/` redundant double-trigger when internal → leave `CRON_SHARED_SECRET` unset (route 404) | Medium | Accept | Phase 3 |
| 13 | Migrator IAM under-specified / admin profile → exact least-priv `Scan`-only + Scan-based verify, read-only profile | Medium | Accept | Phase 4 |
| 14 | No Mongo reconnect/health story → document auto-reconnect + pool opts; DB-aware healthcheck or accept trade-off | Medium | Accept | Phase 3 |
| 15 | `validateKey` rejects `/` but migrator bypasses it → migrator `validateKey` each `sk`, fail loud | Medium | Accept | Phase 4 |

Verified non-issues (no change needed): `List()` `$gte/$lt` range avoids regex injection (sound, mirrors firestore); secrets fallback to plain env is verified correct; `.env` already gitignored.

### Whole-Plan Consistency Sweep
Re-read all phase files after applying findings. Reconciled:
- `updatedAt` storage type now consistently **int64 nanos** in Phase 1 doc-shape and Phase 4 migrator/risk (was "BSON datetime").
- CAS absent-case is **`InsertOne`** everywhere (Phase 1 architecture + Phase 4 encoding note); the `$exists:false` upsert is removed.
- Migrator writes **through `MongoKVStore.Put`** in Phase 4 architecture, risk, and success criteria (no raw `UpdateOne`).
- Health endpoint described as **plain text `miti99bot ok`** in Phase 3 requirements, architecture, and risk (was "JSON"); bogus `-healthcheck` CMD removed from the compose snippet.
- Cron dispatcher named **`DispatchScheduled`** in Phase 2 with scheduler-local timeout/recover.
- Cutover ordering (disable EventBridge → `deleteWebhook` → migrate → start polling container, whose scheduler runs by default) consistent across Phase 2 risk and Phase 4 runbook. <!-- Session 2: polling cutover supersedes setWebhook -->
No unresolved contradictions remain.

## Validation Log

### Session — 2026-06-27
Verification pass skipped: `## Red Team Review` already carries full `file:line` evidence and no `[UNVERIFIED]` tags remain. Interview resolved all 8 open questions.

| # | Question | Decision | Affects |
|---|----------|----------|---------|
| 1 | Rollback RPO | **Simple short-window cutover; NO reverse migrator.** User coordinates users to pause around cutover, so no writes occur mid-migration. | Phase 4 |
| 2 | Decommission AWS | **Tear down the SAM stack after `--verify` passes.** No long-term fallback. EventBridge schedule still disabled as an explicit pre-cutover step. Stop the GitHub Actions deploy workflow. | Phase 4 |
| 3 | Atlas network access | **`0.0.0.0/0` (no stable Coolify egress IP).** Accepted trade-off: mandatory strong unique password + least-privilege DB user (`readWrite` on one DB, not admin). Documented as a knowing widening vs DynamoDB's IAM-gated posture. | Phase 3 |
| 4 | deploynotify | **Keep it** — inject `gitSHA` via Dockerfile `ARG GIT_SHA` + Coolify build-arg. | Phase 3 |
| 5 | `/cron/` HTTP route | **Disable in prod** — leave `CRON_SHARED_SECRET` unset (route 404s); internal scheduler is the sole trigger. | Phase 2, Phase 3 |
| 6 | Mongo driver | `go.mongodb.org/mongo-driver/v2` (current stable); `robfig/cron/v3` for the scheduler. | Phase 1, Phase 2 |
| 7 | Atlas tier | Free **M0** (512 MB) — sufficient for the tiny paper-trading KV. | Phase 3 |
| 8 | `updatedAt` reader | Confirmed write-only today; store as int64 for cheap parity. No TTL/sort planned. | Phase 1, Phase 4 |

### Session 2 — 2026-06-27 (Telegram transport)
User directive: use long polling, **as the only mode — no env toggle**. Decision: keep the existing `go-telegram/bot` library (native polling via `b.Start(ctx)` — no library swap, handlers unchanged) and **delete the webhook code path entirely** (webhook existed only for Lambda, which is being decommissioned — YAGNI). Removed: `internal/telegram/webhook.go` + test, the `/webhook` route, and the `WebhookSecret` config + its startup fatal. Consequences propagated to Phase 3 (no public ingress/domain/TLS/webhook secret; health server internal-only; single replica = single polling consumer) and Phase 4 (cutover = `deleteWebhook` → start polling container, which drains the buffered queue; no `setWebhook`). Rollback during the pre-`sam delete` window still works because the live Lambda keeps its own already-built webhook code until teardown. This eliminated the earlier "public ingress DDoS" risk entirely.

### Session 3 — 2026-06-27 (minimize env surface)
User directive: don't require `KV_PROVIDER`/`CRON_MODE`; mongo + in-process cron are defaults; remove unneeded system envs. Decisions: (1) `buildProvider` auto-selects `mongodb` when `MONGO_URL` is set (else `memory`); `KV_PROVIDER` is an optional override; the old `AWS_LAMBDA_FUNCTION_NAME → dynamodb` auto-detect is removed. (2) The cron scheduler runs unconditionally at container start — `CRON_MODE` env removed (cutover safety is from ordering + the idempotency guard, not a gate). (3) Dropped from required env: `KV_PROVIDER`, `CRON_MODE`, `PORT` (defaults 8080). Final required env = `TELEGRAM_BOT_TOKEN`, `MONGO_URL`, `MONGO_DATABASE`; operational = `MODULES`, `OWNER_ID`, `ADMIN_IDS`; optional = `GEMINI_API_KEY`. Propagated to Phases 1-4.

### Session 4 — 2026-06-27 (drop per-module API URL overrides + two credentials)
User directive: remove the stock/coin/gold URL envs; use the providers configured in code. Decisions:
- **URL overrides dropped** — self-host sets NONE of `STOCK_INCOME_EVENTS_API_URL`, `GOLD_PRICE_API_URL`, `GOLD_FX_API_URL`, `GOLD_VNAPP_API_URL`, `COIN_BINANCE_API_URL`, `COIN_COINBASE_API_URL`, `COIN_COINGECKO_API_URL`; modules use coded default endpoints. Override plumbing stays (dormant) unless later cleaned.
- **`GOLD_VNAPP_API_KEY` dropped** — already unnecessary: `gold/vnappmob_client.go` auto-fetches a VNAppMob API key via the refresh endpoint and caches it in KV (`vnappmob:api_key`, 24h refresh buffer). With Mongo as the KV the key auto-caches to the DB. The env var was only an optional override. No code change — just don't set it.
- **`STOCK_INCOME_EVENTS_API_TOKEN` dropped AND the income-events feature removed** — delete the `stock_income_events` command + its FireAnt `IncomeEventClient` (`internal/modules/stock/income_events.go` + test), the token env, and any user-facing notice/description. The other stock commands (`stock_income_stock`, `stock_income_vnd`) stay. Removing a public command requires updating the command catalog (`aws/telegram-commands.json` → the self-host `setMyCommands` source). Code task added to Phase 3.

Final optional env reduces to `GEMINI_API_KEY` only.

### Session 5 — 2026-06-27 (rename owner/admin envs)
User directive: cleaner names. `BOT_OWNER_ID` → `OWNER_ID`, `ADMIN_USER_IDS` → `ADMIN_IDS`. Rename the env keys in `cmd/server/main.go` `loadConfig`; no backward-compat (the AWS template + workflow that set the old names are being deleted). The Go config field names + the `Auth{BotOwnerID, AdminUserIDs}` struct can keep their internal names — only the env keys change.

### Whole-Plan Consistency Sweep (post-validation)
- Phase 4 rollback/teardown rewritten: AWS torn down after verify; reverse-migrator option removed; rollback framed as "coordinate users, short window" not "keep Lambda N days."
- Phase 3 Atlas networking: `0.0.0.0/0` is now the chosen path (was "egress-IP default") with password + least-priv user as hard requirements.
- deploynotify `gitSHA` injection and `/cron/` disabled (`CRON_SHARED_SECRET` unset) were already the recommended defaults in Phases 2/3 — now confirmed, no contradiction.
- No unresolved contradictions remain.

## Open Questions

None — all resolved in the Validation Log above.
