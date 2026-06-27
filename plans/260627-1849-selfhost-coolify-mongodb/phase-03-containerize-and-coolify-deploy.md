---
phase: 3
title: "Containerize and Coolify Deploy"
status: pending
priority: P2
dependencies: [1, 2]
effort: "M"
---

# Phase 3: Containerize and Coolify Deploy

## Overview

Ship the existing distroless image via a docker-compose stack on Coolify. MongoDB Atlas is external/managed, so compose runs only the bot service. Configuration (secrets + backend selection) is supplied as Coolify env vars. Document webhook registration against the Coolify public domain.

## Requirements

- Functional: `docker compose up` runs the bot on `:8080` with `KV_PROVIDER=mongodb` + `CRON_MODE=internal`, persistent storage in Atlas.
- Functional: Coolify exposes the service over HTTPS at a domain; `GET /` returns `text/plain` `miti99bot ok` (health, not JSON).
- Functional: Telegram webhook points at `https://<coolify-domain>/webhook` with the secret token.
- Non-functional: container healthcheck on `/`; restart policy `unless-stopped`; runs a single replica (cron correctness, see Phase 2).
- Non-functional: no secrets committed — all via Coolify env / `.env` (gitignored). Provide `.env.example`.

## Architecture

The existing `Dockerfile` (golang:1.25-alpine builder → distroless static nonroot, `ENTRYPOINT ["/server"]`, `EXPOSE 8080`) builds with `-ldflags="-s -w"` only — it does **not** inject `gitSHA`. Only the Makefile injects it (`Makefile:13`). `deploynotify` runs unconditionally at startup (`main.go:148`) and stays silent when `gitSHA` is empty (`main.go:40-41`), so as-is the "new version" owner DM is **silently dead on self-host** — a behavior regression from Lambda. Required (not optional): add `ARG GIT_SHA` + `-ldflags "-s -w -X main.gitSHA=$GIT_SHA"` to the Dockerfile and pass `--build-arg GIT_SHA=$(git rev-parse --short HEAD)` (Coolify exposes the commit SHA as a build var). If deploynotify parity is explicitly unwanted, instead document in acceptance criteria that the feature is disabled on self-host — do not leave it as undocumented breakage.

`docker-compose.yml` (committed; Coolify consumes it):
```yaml
services:
  bot:
    build: .                 # or image: ghcr.io/tiennm99/miti99bot:latest
    restart: unless-stopped
    environment:
      PORT: "8080"
      KV_PROVIDER: mongodb
      MONGO_URL: ${MONGO_URL}
      MONGO_DATABASE: ${MONGO_DATABASE}
      CRON_MODE: internal
      MODULES: ${MODULES}
      BOT_OWNER_ID: ${BOT_OWNER_ID}
      ADMIN_USER_IDS: ${ADMIN_USER_IDS}
      TELEGRAM_BOT_TOKEN: ${TELEGRAM_BOT_TOKEN}
      TELEGRAM_WEBHOOK_SECRET: ${TELEGRAM_WEBHOOK_SECRET}
      GEMINI_API_KEY: ${GEMINI_API_KEY}
      # NOTE: CRON_SHARED_SECRET intentionally OMITTED — with CRON_MODE=internal
      # the in-process scheduler owns cron timing, so leaving it unset makes the
      # public /cron/{name} route 404 (router.go:59,66-69) and removes a redundant
      # internet-reachable trigger surface. Set it only if you want manual curl triggers.
      # NOTE: do NOT set any *_PARAMETER_NAME vars (see Secrets below).
      # optional API overrides as needed
    expose: ["8080"]
    # No compose healthcheck: distroless has no shell/curl AND cmd/server has no
    # flag parsing (no -healthcheck flag exists). Use Coolify's native HTTP monitor
    # against GET / (returns text/plain "miti99bot ok", NOT JSON).
```

Healthcheck: distroless has no shell/curl/wget, and `cmd/server` has no flag parsing — `/server -healthcheck` does NOT exist and would just start the server. Use **Coolify's native HTTP monitor against `/`** (returns `text/plain` body `miti99bot ok`, per `internal/server/health.go` — not JSON). Only if a compose-level healthcheck is mandatory, first implement a real `-healthcheck` flag in `cmd/server` (localhost GET `/`, exit 0/1) — otherwise omit it. Note: a plain `/` check does NOT verify Mongo connectivity (see Risk Assessment).

Secrets: because `*_PARAMETER_NAME` env vars are NOT set, `cmd/server` reads `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, `CRON_SHARED_SECRET`, `GEMINI_API_KEY` directly (verified: `resolveSSMSecrets` returns early when no param names are set, `main.go:355-366`). **Hard requirement: `.env.example` must list all six `*_PARAMETER_NAME` vars explicitly as "leave UNSET for self-host."** `resolveSSMSecrets` is called unconditionally (`main.go:79`); if even one `*_PARAMETER_NAME` is set with an unset target, it calls `awsconfig.LoadDefaultConfig` + `ssm.GetParameters` (`main.go:371-378`) which fails with no AWS creds → `log.Fatal` (`main.go:80`), bricking startup. With all six unset, no AWS credentials are needed in the container.

## Related Code Files

- Create: `docker-compose.yml` — bot service as above.
- Create: `.env.example` — every env var with placeholder values + comments; real `.env` gitignored.
- Modify: `.gitignore` — ensure `.env` ignored (verify; add if missing).
- Modify (optional): `Dockerfile` — `ARG GIT_SHA` + `-ldflags "-X main.gitSHA=$GIT_SHA"` for deploynotify parity.
- Modify (optional): `cmd/server/main.go` — `-healthcheck` flag (only if compose healthcheck chosen over Coolify HTTP monitor).
- Create: `docs/deploy-coolify-selfhosted.md` — full onboarding: create Atlas M0 cluster, get SRV URL, create a least-privilege DB user (`readWrite` on one DB, strong unique password), set network access to `0.0.0.0/0` (accepted trade-off — record it), Coolify new resource from compose/Git, set env vars, deploy, grab public domain, register webhook + command menu.
- Modify: `README.md` — add "Self-host (Coolify + MongoDB Atlas)" deploy option alongside the AWS path.

## Implementation Steps

1. Write `docker-compose.yml` + `.env.example`; verify `.env` gitignored.
2. Decide healthcheck approach (Coolify HTTP monitor preferred); implement `-healthcheck` flag only if needed.
3. Local validation: `MONGO_URL=… MONGO_DATABASE=… docker compose up --build`; confirm boot logs show `storage backend backend=mongodb database=…` (NO connection string) and `internal cron scheduler started`; `curl localhost:8080/` returns `miti99bot ok` (plain text).
4. Atlas setup (M0): cluster, DB user, network access, copy `mongodb+srv://…` SRV URL.
5. Coolify: create resource (Git repo + compose, or prebuilt image), set env vars, deploy, note assigned HTTPS domain.
6. Register webhook + commands against the Coolify domain (reuse `make telegram-webhook`/`telegram-commands` with the URL, or the documented curl). Note: `make telegram-*` currently pulls token from SSM — for self-host, add env-var-based variants or document the direct curl.
7. Smoke test: send `/ping`, `/help`; confirm a persisted command (e.g. stock paper trade) survives a container restart.
8. Write `docs/deploy-coolify-selfhosted.md`; update README.

## Success Criteria

- [ ] `docker compose up --build` boots the bot with mongo + internal cron locally.
- [ ] Coolify deployment is reachable at its HTTPS domain; `/` health passes; restarts `unless-stopped`.
- [ ] Telegram webhook set to the Coolify domain; live commands respond.
- [ ] No secret committed; `.env.example` documents every variable, lists all six `*_PARAMETER_NAME` as "leave UNSET", and omits `CRON_SHARED_SECRET` by default.
- [ ] Container boots with zero `*_PARAMETER_NAME` set (no AWS creds present) and never attempts an SSM/AWS call.
- [ ] `deploynotify` parity decided: either `gitSHA` injected via Dockerfile build arg (owner DM works) or the feature is documented as disabled on self-host.
- [ ] `docs/deploy-coolify-selfhosted.md` is complete enough to redo from scratch.

## Risk Assessment

- **Distroless healthcheck**: no shell, and no `-healthcheck` flag exists. Mitigation: use Coolify's HTTP monitor against `/` (text body `miti99bot ok`), or implement the flag first. Do not ship the bogus `["CMD","/server","-healthcheck"]` line.
- **Atlas network access — `0.0.0.0/0` accepted (validated decision)**: the Coolify host has no stable egress IP, so the Atlas IP access list is `0.0.0.0/0` (internet-reachable). This widens the surface beyond the project's documented "no non-designed-surface public resource" boundary (`docs/deploy-aws-free-tier-guide.md:25,35`) — under DynamoDB the DB was IAM-gated and never internet-reachable — and is knowingly accepted for self-host. **Mandatory compensating controls (hard requirements):** (1) a strong unique DB password, (2) a least-privilege Atlas DB user scoped to `readWrite` on the single app database (never Atlas admin / cluster-wide), (3) the connection string is treated as a secret and never logged (see Phase 1 / Finding-3 constraint). Record the acceptance in `docs/deploy-coolify-selfhosted.md`. <!-- Updated: Validation Session 1 - 0.0.0.0/0 accepted; least-priv user + strong password required -->
- **Public ingress is now the bot's own process (Medium)**: a Function URL sat behind AWS edge (throttling, managed TLS, absorbed L3/4 floods); a Coolify domain exposes the single `:8080` Go process directly, and Phase 2 pins 1 replica. An unauthenticated flood saturates the box even though `/webhook` rejects via constant-time secret compare (`telegram/webhook.go:55-60`). Mitigation: front the service with Coolify's built-in Traefik rate-limit middleware (no extra cost); keep existing `ReadHeaderTimeout`/`ReadTimeout` (`main.go:165-166`).
- **Mongo runtime connection loss (Medium)**: unlike the per-call short-lived DynamoDB client (`dynamodb_client.go`), self-host holds one long-lived Mongo client for days; Atlas M0 idles/fails over. The v2 driver auto-reconnects on the next op (confirm against current driver docs) — set sane pool + server-selection timeout options. A plain `/` healthcheck reports healthy even when Mongo is unreachable, so Coolify won't restart a DB-wedged bot; either make the healthcheck DB-aware (lightweight `Ping`) or explicitly accept "stays up reporting healthy during DB outage" as the chosen trade-off.
- **Webhook tooling assumes SSM**: `make telegram-*` reads SSM. Mitigation: document env-var/curl variant for self-host; optionally add `telegram-webhook-url URL=…` Make target.
- **Single replica**: scaling >1 double-fires cron + duplicates webhook processing is fine but cron is not. Mitigation: pin 1 replica in Coolify; documented in Phase 2.
