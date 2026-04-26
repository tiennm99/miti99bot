# Using MongoDB Atlas

Operational runbook for the MongoDB Atlas backend introduced by `plans/260425-1945-mongodb-atlas-migration/`.

## Cluster

| Field | Value |
|---|---|
| Provider | MongoDB Atlas |
| Tier | M0 Free |
| Region | `aws-ap-southeast-1` (Singapore) |
| Cluster name | `miti99bot-prod` (operator confirms) |
| Database | `miti99bot` |
| DB user | `miti99bot-worker` (`readWrite@miti99bot`) |

Connection string format:

```
mongodb+srv://miti99bot-worker:<pass>@<host>/miti99bot?retryWrites=true&w=majority
```

Stored in two places (must match):
1. CF Worker secret: `wrangler secret put MONGODB_URI`
2. `.env.deploy` (gitignored, used by local backfill / verify scripts)

Same secret-mirror protocol as `TELEGRAM_BOT_TOKEN`.

## Free-tier ceiling

- 512 MB storage (data + indexes)
- 500 max concurrent connections
- ~100 ops/sec sustained (no daily cap)
- No backups, single region, no PITR
- Auto-pauses after 30 days of zero ops

Upgrade path: **Flex Tier $8–$30/month** (M2/M5 deprecated as of 2026).

## Auto-pause

After 30 days idle the cluster pauses. First request after pause:
- Driver throws `MongoServerSelectionError` after `serverSelectionTimeoutMS` (5s).
- Worker code (see `src/db/mongo-client.js`, lands Phase 02) catches and returns 503 with `Retry-After: 30`.
- Cluster auto-wakes within 30–60s on attempted connection.

The bot has 6+ daily crons; any cron that writes Mongo prevents pause. Phase 08 confirms.

## Network access

`0.0.0.0/0` — Cloudflare Workers do NOT have static egress IPs on the Free or basic Paid plans. Only auth (SCRAM-SHA-256) + TLS gate connections.

**Permanent risk** unless upgrading to CF Workers paid static-egress IP add-on (~$10/mo).

Mitigations:
- DB user has `readWrite` on one db only (NOT `dbAdmin` / `clusterAdmin`).
- Password ≥32 chars random.
- Rotate quarterly.
- Atlas free-tier email alerts configured for cluster unavailability + connections > 400.

## Bundle gate (Phase 01 result)

Measured `npx wrangler deploy --dry-run` with a minimal probe importing `MongoClient`:

| Metric | Value | Cap (Free) | Cap (Paid) |
|---|---|---|---|
| Compressed (gzip) | **226 KiB** | 3 MiB | 10 MiB |
| Raw (minified) | 1.74 MiB | — | — |
| On-disk (uncompressed) | 3.9 MiB | — | — |

Pass on both plans with **>92% headroom**. nodejs_compat_v2 provides `node:net`/`node:tls`/`node:crypto` from the runtime, so the driver's transitive deps are not bundled.

## CPU-time gate (Phase 01 — operator-run)

Requires real Atlas + `wrangler dev`. Procedure:

1. Add a temporary `/__mongo-ping` route that connects + runs `db.runCommand({ping:1})` + returns `{wall_ms}`.
2. Run 5+ cold cycles (10-min spaced).
3. Inspect CF dashboard CPU column for each invocation.
4. **Hard gate**: if any cold-start CPU time approaches 50ms (Free plan limit), abort migration. Escalate to paid plan or pivot via `phase-07-alt-pivot.md`.
5. Record cold-ping P95 wall-clock as `BASELINE_COLD_PING_MS` here:

```
BASELINE_COLD_PING_MS = <fill after measurement>
```

Phase 06 derives the abort threshold from this value: `2.5 × BASELINE_COLD_PING_MS`.

## Auto-pause behavior gate (Phase 01 — operator-run)

In Atlas UI, manually pause the cluster, then hit `/__mongo-ping`. Confirm:
- Driver throws within 5s (does NOT hang indefinitely).
- Error class is `MongoServerSelectionError` (or driver subclass).
- Phase 02 `getDb()` catches this and surfaces a 503.

## Node API surface

`src/` (the Worker) imports zero `node:*` modules today. `nodejs_compat_v2` is enabled solely for the `mongodb` driver:

| Module | Used by Worker? | Used by scripts/? |
|---|---|---|
| `node:fs` | no | yes (build/scrape/migrate) |
| `node:path` | no | yes |
| `node:child_process` | no | yes (migrate.js) |
| `node:net` | indirectly (via mongodb) | no |
| `node:tls` | indirectly (via mongodb) | no |
| `node:crypto` | indirectly (via mongodb) | no |
| `process.env` | no | yes (register.js) |
| `Buffer` | no | no |

Risk: minimal. No existing module relies on the absence of these globals.

## Rollback

If migration is abandoned at any phase before cutover:

1. `wrangler secret delete MONGODB_URI`
2. Revert `wrangler.toml`: remove `compatibility_flags = ["nodejs_compat_v2"]`.
3. `npm uninstall mongodb`.
4. `npm run deploy` — bot continues on KV/D1 unchanged.
5. (Optional) Delete Atlas cluster from UI.

`scripts/check-secret-leaks.js` should stay — it covers other secrets too.

## Rotation

`MONGODB_URI` rotation cadence: every 90 days, owner = repo maintainer.

Procedure:
1. In Atlas UI → Database Access → edit `miti99bot-worker` → reset password.
2. Update `.env.deploy` with new URI.
3. `wrangler secret put MONGODB_URI` (paste new URI).
4. `npm run deploy` (re-runs register; no Worker restart needed since secret reads at request time via `env.MONGODB_URI`).

Mismatch between `.env.deploy` and CF secret causes register-script failure on next deploy — same fail-loud pattern as `TELEGRAM_WEBHOOK_SECRET`.

## Alerts

Configured in Atlas free-tier UI:

- **Cluster unavailable** → email maintainer.
- **Current connections > 400** (80% of cap) → email maintainer.

Plus CF Observability rule (Phase 06): >10 errors per 1 min window → email.
