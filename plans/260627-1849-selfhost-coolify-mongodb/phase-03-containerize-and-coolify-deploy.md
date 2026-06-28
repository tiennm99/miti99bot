---
phase: 3
title: "Long-Polling Runtime + Containerize + Coolify Deploy"
status: code-complete-operator-pending
priority: P2
dependencies: [1, 2]
effort: "M"
---

# Phase 3: Long-Polling Runtime + Containerize + Coolify Deploy

## Overview

Two coupled changes: (1) switch the Telegram transport from webhook to **long polling** so the self-hosted container needs NO public inbound ingress, and (2) ship the existing distroless image via a docker-compose stack on Coolify. MongoDB Atlas is external/managed, so compose runs only the bot service. Config is supplied as Coolify env vars.

Long polling is the key self-host simplification: the bot opens an OUTBOUND connection to `api.telegram.org` and pulls updates (`getUpdates`), instead of Telegram POSTing to a public URL. That removes the public HTTPS domain, TLS/Traefik routing for inbound, the `/webhook` route, the webhook secret, and the entire unauthenticated-ingress attack surface. The same library (`go-telegram/bot v1.20.0`) does this via `b.Start(ctx)` — no library change, handlers unchanged.

**Polling is the ONLY transport — no toggle.** Webhook mode existed solely for Lambda, which is being decommissioned (Phase 5), so the webhook code path is removed entirely (YAGNI). The currently-deployed Lambda keeps running its own already-built webhook code until `sam delete`, so the short rollback window in Phase 4 is unaffected by deleting webhook code from this branch.

## Requirements

- Functional: `docker compose up` runs the bot with persistent storage in Atlas — mongodb is auto-selected from `MONGO_URL` and the cron scheduler runs by default (no `KV_PROVIDER`/`CRON_MODE`).
- Functional: bot consumes updates via long polling (`b.Start(ctx)`) as the sole transport; no `/webhook` route, no webhook secret, no public domain. Only OUTBOUND HTTPS to Telegram is needed.
- Functional: the webhook code path (`internal/telegram/webhook.go`, the `/webhook` route, `TELEGRAM_WEBHOOK_SECRET` config + its startup fatal) is **deleted**, not gated.
- Non-functional: container restart policy `unless-stopped`; **exactly one replica** (Telegram allows only ONE polling consumer per bot — a second poller gets HTTP 409; also cron correctness, Phase 2).
- Non-functional: health server stays available internally (`GET /` → `text/plain` `miti99bot ok`) for Coolify's container healthcheck, but is NOT publicly routed.
- Non-functional: no secrets committed — all via Coolify env / `.env` (gitignored). Provide `.env.example`.

## Architecture

### Telegram transport: long polling (only mode)
`go-telegram/bot` runs long polling when you call `b.Start(ctx)` (its `getUpdates` loop). In `cmd/server/main.go`, after `modules.Install`, run `b.Start(rootCtx)` in a goroutine; it returns when `rootCtx` is cancelled (graceful shutdown). Remove the webhook wiring:
- Delete `internal/telegram/webhook.go` + `webhook_test.go`, the `/webhook` route in `internal/server/router.go`, and the `WebhookSecret` config field + its `if cfg.WebhookSecret == ""` fatal (`main.go:85-88`).
- The HTTP server still runs for the `/` health route only (Coolify healthcheck) — bind it, do not publicly route it. (`/cron` stays mountable but is unused in polling+internal-cron mode and isn't exposed.)
- A one-time `deleteWebhook` is required before first poll (else `getUpdates` 409s if a webhook is still set — Phase 4 cutover). Only ONE polling process per bot token (second → 409) → single replica.
- `WithSkipGetMe()` (currently set for fast Lambda cold start) is harmless to keep. `WithNotAsyncHandlers` rationale was webhook-specific — re-evaluate, but handlers take `ctx` (not `r.Context()`) so leaving them as-is is safe.

### Container + Coolify
The existing `Dockerfile` (golang:1.25-alpine builder → distroless static nonroot, `ENTRYPOINT ["/server"]`, `EXPOSE 8080`) builds with `-ldflags="-s -w"` only — it does **not** inject `gitSHA`. Only the Makefile injects it (`Makefile:13`). `deploynotify` runs unconditionally at startup (`main.go:148`) and stays silent when `gitSHA` is empty (`main.go:40-41`), so as-is the "new version" owner DM is **silently dead on self-host** — a behavior regression from Lambda. Required (not optional): add `ARG GIT_SHA` + `-ldflags "-s -w -X main.gitSHA=$GIT_SHA"` to the Dockerfile and pass `--build-arg GIT_SHA=$(git rev-parse --short HEAD)` (Coolify exposes the commit SHA as a build var). If deploynotify parity is explicitly unwanted, instead document in acceptance criteria that the feature is disabled on self-host — do not leave it as undocumented breakage.

`compose.yml` (committed; Coolify consumes it):
```yaml
services:
  bot:
    build: .                 # or image: ghcr.io/tiennm99/miti99bot:latest
    restart: unless-stopped
    environment:
      MONGO_URL: ${MONGO_URL}
      MONGO_DATABASE: ${MONGO_DATABASE}
      MODULES: ${MODULES}
      OWNER_ID: ${OWNER_ID}
      ADMIN_IDS: ${ADMIN_IDS}
      TELEGRAM_BOT_TOKEN: ${TELEGRAM_BOT_TOKEN}
      GEMINI_API_KEY: ${GEMINI_API_KEY}
      # Storage auto-selects mongodb because MONGO_URL is set (no KV_PROVIDER needed).
      # The in-process cron scheduler runs by default (no CRON_MODE needed).
      # PORT defaults to 8080 (health server) — omit unless overriding.
      # Long polling = no TELEGRAM_WEBHOOK_SECRET, no /webhook, no CRON_SHARED_SECRET.
      # Do NOT set any *_PARAMETER_NAME vars (see Secrets below).
      # No stock/coin/gold *_API_URL overrides — modules use their coded default
      # providers (stock: SSI/FireAnt; coin: Binance→Coinbase→CoinGecko; gold: VNAppMob→spot).
    # No published ports / domain: polling is outbound-only, nothing inbound to route.
    # `expose` (internal-only) is enough for Coolify's container healthcheck on /.
    expose: ["8080"]
    # No compose healthcheck: distroless has no shell/curl AND cmd/server has no
    # flag parsing (no -healthcheck flag exists). Use Coolify's container/HTTP monitor
    # against GET / (returns text/plain "miti99bot ok", NOT JSON).
```

Healthcheck: distroless has no shell/curl/wget, and `cmd/server` has no flag parsing — `/server -healthcheck` does NOT exist and would just start the server. Use **Coolify's native HTTP monitor against `/`** (returns `text/plain` body `miti99bot ok`, per `internal/server/health.go` — not JSON). Only if a compose-level healthcheck is mandatory, first implement a real `-healthcheck` flag in `cmd/server` (localhost GET `/`, exit 0/1) — otherwise omit it. Note: a plain `/` check does NOT verify Mongo connectivity (see Risk Assessment).

Secrets: because `*_PARAMETER_NAME` env vars are NOT set, `cmd/server` reads `TELEGRAM_BOT_TOKEN`, `GEMINI_API_KEY` directly (verified: `resolveSSMSecrets` returns early when no param names are set, `main.go:355-366`). `TELEGRAM_WEBHOOK_SECRET` is removed with the webhook code (no longer read), so the `if cfg.WebhookSecret == ""` fatal (`main.go:85-88`) is deleted — the bot boots without it. `CRON_SHARED_SECRET` is unused (internal cron) and left unset. **Hard requirement: `.env.example` must list all six `*_PARAMETER_NAME` vars explicitly as "leave UNSET for self-host."** `resolveSSMSecrets` is called unconditionally (`main.go:79`); if even one `*_PARAMETER_NAME` is set with an unset target, it calls `awsconfig.LoadDefaultConfig` + `ssm.GetParameters` (`main.go:371-378`) which fails with no AWS creds → `log.Fatal` (`main.go:80`), bricking startup. With all six unset, no AWS credentials are needed in the container.

## Related Code Files

- Modify: `cmd/server/main.go` — `go b.Start(rootCtx)` after `modules.Install`; delete the `WebhookSecret` config field, its load, and the `if cfg.WebhookSecret == ""` fatal (`main.go:85-88`). Rename the env keys read in `loadConfig`: `BOT_OWNER_ID` → `OWNER_ID`, `ADMIN_USER_IDS` → `ADMIN_IDS` (no backward-compat — AWS template that set the old names is being deleted).
- Delete: `internal/telegram/webhook.go` + `internal/telegram/webhook_test.go`.
- Modify: `internal/server/router.go` — remove the `/webhook` route; keep `/` health (and `/cron`, unused/unexposed).
- Modify: `internal/telegram/client.go` — update the "configured for webhook mode" doc to polling; keep `WithSkipGetMe`, re-evaluate `WithNotAsyncHandlers`.
- Create: `compose.yml` — bot service as above.
- Create: `.env.example` — every env var with placeholder values + comments; real `.env` gitignored.
- Modify: `.gitignore` — ensure `.env` ignored (verify; add if missing).
- Modify (optional): `Dockerfile` — `ARG GIT_SHA` + `-ldflags "-X main.gitSHA=$GIT_SHA"` for deploynotify parity.
- Modify (optional): `cmd/server/main.go` — `-healthcheck` flag (only if compose healthcheck chosen over Coolify HTTP monitor).
- Create: `docs/deploy-coolify-selfhosted.md` — full onboarding: create Atlas M0 cluster, get SRV URL, create a least-privilege DB user (`readWrite` on one DB, strong unique password), set network access to `0.0.0.0/0` (accepted trade-off — record it), Coolify new resource from compose/Git, set env vars, deploy (no public domain needed — polling is outbound-only), one-time `deleteWebhook`, register the command menu via `setMyCommands`.
- Modify: `README.md` — add "Self-host (Coolify + MongoDB Atlas)" deploy option alongside the AWS path; drop the stock income-events row from the module table.
- Delete: `internal/modules/stock/income_events.go` + `income_events_test.go`; remove the `stock_income_events` command registration (`stock/stock.go:45-48`), the `incomeEvents *IncomeEventClient` field + `NewIncomeEventClientFromEnv` wiring (`stock/handlers.go:25,43`), `handleIncomeEvents`, and the `STOCK_INCOME_EVENTS_API_URL`/`_TOKEN` config + `exportOptionalEnv` lines in `main.go`. Keep `stock_income_stock` / `stock_income_vnd`.
- Modify: `aws/telegram-commands.json` (the `setMyCommands` source) — remove the `stock_income_events` entry so the command menu matches.
- No change for gold: `gold/vnappmob_client.go` already auto-fetches + caches the VNAppMob key in KV (`vnappmob:api_key`); just leave `GOLD_VNAPP_API_KEY` unset.

## Implementation Steps

1. Write `compose.yml` + `.env.example`; verify `.env` gitignored.
2. Decide healthcheck approach (Coolify HTTP monitor preferred); implement `-healthcheck` flag only if needed.
1b. Switch `cmd/server/main.go` to `b.Start(rootCtx)`; delete the webhook handler, `/webhook` route, and `WebhookSecret` config + fatal.
3. Local validation: `MONGO_URL=… MONGO_DATABASE=… docker compose up --build`; confirm boot logs show `storage backend backend=mongodb database=…` (NO connection string), `internal cron scheduler started`, and the polling loop started (getUpdates); `curl localhost:8080/` returns `miti99bot ok`. (Requires the bot's webhook to be unset — see Phase 4 / `deleteWebhook`, else getUpdates 409s.)
4. Atlas setup (M0): cluster, DB user, network access, copy `mongodb+srv://…` SRV URL.
5. Coolify: create resource (Git repo + compose, or prebuilt image), set env vars, deploy. No public domain/route needed (outbound-only); ensure exactly 1 replica.
6. Command menu only: `setMyCommands` (still an HTTPS POST to Telegram). `make telegram-commands` currently pulls the token from SSM — for self-host, add an env-var/token-arg variant or document the direct curl. No webhook registration step exists in polling mode.
7. Smoke test: send `/ping`, `/help`; confirm a persisted command (e.g. stock paper trade) survives a container restart.
8. Write `docs/deploy-coolify-selfhosted.md`; update README.

## Success Criteria

- [ ] `docker compose up --build` boots the bot with mongo + internal cron + polling locally; logs show the getUpdates loop running.
- [ ] Coolify deployment runs with NO public domain/port (outbound-only); `/` health passes internally; restarts `unless-stopped`; exactly 1 replica.
- [ ] Bot receives messages via long polling (no webhook set); live commands respond.
- [ ] Webhook code removed (`webhook.go`, `/webhook` route, `WebhookSecret` config/fatal gone); bot boots with no `TELEGRAM_WEBHOOK_SECRET`.
- [ ] No secret committed; `.env.example` documents every variable, lists all six `*_PARAMETER_NAME` as "leave UNSET", and omits `CRON_SHARED_SECRET` by default.
- [ ] Container boots with zero `*_PARAMETER_NAME` set (no AWS creds present) and never attempts an SSM/AWS call.
- [ ] `deploynotify` parity decided: either `gitSHA` injected via Dockerfile build arg (owner DM works) or the feature is documented as disabled on self-host.
- [ ] `docs/deploy-coolify-selfhosted.md` is complete enough to redo from scratch.
- [ ] No stock/coin/gold `*_API_URL` overrides set; modules work on coded defaults. `stock_income_events` command + FireAnt client removed; command catalog updated; `make vet`/`make test` green after removal.
- [ ] Gold works with `GOLD_VNAPP_API_KEY` unset (auto-fetches + caches the VNAppMob key to Mongo at `vnappmob:api_key`).

## Risk Assessment

- **Distroless healthcheck**: no shell, and no `-healthcheck` flag exists. Mitigation: use Coolify's HTTP monitor against `/` (text body `miti99bot ok`), or implement the flag first. Do not ship the bogus `["CMD","/server","-healthcheck"]` line.
- **Atlas network access — `0.0.0.0/0` accepted (validated decision)**: the Coolify host has no stable egress IP, so the Atlas IP access list is `0.0.0.0/0` (internet-reachable). This widens the surface beyond the project's documented "no non-designed-surface public resource" boundary (`docs/deploy-aws-free-tier-guide.md:25,35`) — under DynamoDB the DB was IAM-gated and never internet-reachable — and is knowingly accepted for self-host. **Mandatory compensating controls (hard requirements):** (1) a strong unique DB password, (2) a least-privilege Atlas DB user scoped to `readWrite` on the single app database (never Atlas admin / cluster-wide), (3) the connection string is treated as a secret and never logged (see Phase 1 / Finding-3 constraint). Record the acceptance in `docs/deploy-coolify-selfhosted.md`. <!-- Updated: Validation Session 1 - 0.0.0.0/0 accepted; least-priv user + strong password required -->
- **No public ingress (resolved by polling)**: long polling is outbound-only, so there is NO public domain or `/webhook` to flood — the entire unauthenticated-ingress attack surface that webhook mode created is gone. (This supersedes the earlier "public ingress is the bot's own process" risk.) Residual: the `/` health server should stay internal (Coolify `expose`, not `ports`); never publish it.
- **Single polling consumer (409 conflict)**: Telegram permits only ONE `getUpdates` consumer per bot token. Two replicas, OR a leftover webhook still set, OR an overlapping old instance during redeploy → HTTP 409 / lost updates. Mitigation: exactly 1 replica; `deleteWebhook` before first poll (Phase 4); prefer stop-first redeploy (also covers the Phase 2 cron-overlap concern).
- **Mongo runtime connection loss (Medium)**: unlike the per-call short-lived DynamoDB client (`dynamodb_client.go`), self-host holds one long-lived Mongo client for days; Atlas M0 idles/fails over. The v2 driver auto-reconnects on the next op (confirm against current driver docs) — set sane pool + server-selection timeout options. A plain `/` healthcheck reports healthy even when Mongo is unreachable, so Coolify won't restart a DB-wedged bot; either make the healthcheck DB-aware (lightweight `Ping`) or explicitly accept "stays up reporting healthy during DB outage" as the chosen trade-off.
- **Telegram tooling assumes SSM**: `make telegram-commands` reads the token from SSM. Mitigation: document an env-var/token-arg variant for self-host (only `setMyCommands` is needed in polling mode; no webhook registration).
