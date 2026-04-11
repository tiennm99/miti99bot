# Phase 02 — Webhook entrypoint

## Context Links
- Plan: [plan.md](plan.md)
- Reports: [grammY on CF Workers](../reports/researcher-260411-0853-grammy-on-cloudflare-workers.md)

## Overview
- **Priority:** P1
- **Status:** pending
- **Description:** fetch handler with URL routing (`/webhook`, `GET /` health, 404 otherwise), memoized `Bot` instance, grammY webhook secret-token validation wired through `webhookCallback`. Webhook + command-menu registration with Telegram is handled OUT OF BAND via a post-deploy node script (phase-07) — the Worker itself exposes no admin surface.

## Key Insights
- Use **`"cloudflare-mod"`** adapter (NOT `"cloudflare"` — that's the legacy service-worker variant).
- `webhookCallback(bot, "cloudflare-mod", { secretToken })` delegates `X-Telegram-Bot-Api-Secret-Token` validation to grammY — no manual header parsing.
- Bot instance must be memoized at module scope but lazily constructed (env not available at import time).
- No admin HTTP surface on the Worker — `setWebhook` + `setMyCommands` run from a local node script at deploy time, not via the Worker.

## Requirements
### Functional
- `POST /webhook` → delegate to `webhookCallback`. Wrong/missing secret → 401 (handled by grammY).
- `GET /` → 200 `"miti99bot ok"` (health check, unauthenticated).
- Anything else → 404.

### Non-functional
- Single `fetch` function, <80 LOC.
- No top-level await.
- No global state besides memoized Bot.

## Architecture
```
Request
  │
  ▼
fetch(req, env, ctx)
  │
  ├── GET /            → 200 "ok"
  ├── POST /webhook    → webhookCallback(bot, "cloudflare-mod", {secretToken})(req)
  └── *                → 404
```

`getBot(env)` lazily constructs and memoizes the `Bot`, installs dispatcher middleware (from phase-04), and returns the instance.

## Related Code Files
### Create
- `src/index.js` (fetch handler + URL router)
- `src/bot.js` (memoized `getBot(env)` factory — wires grammY middleware from registry/dispatcher)

### Modify
- none

### Delete
- none

## Implementation Steps
1. Create `src/index.js`:
   - Import `getBot` from `./bot.js`.
   - Export default object with `async fetch(request, env, ctx)`.
   - Parse `new URL(request.url)`, switch on `pathname`.
   - For `POST /webhook`: `return webhookCallback(getBot(env), "cloudflare-mod", { secretToken: env.TELEGRAM_WEBHOOK_SECRET })(request)`.
   - For `GET /`: return 200 `"miti99bot ok"`.
   - Default: 404.
2. Create `src/bot.js`:
   - Module-scope `let botInstance = null`.
   - `export function getBot(env)`:
     - If `botInstance` exists, return it.
     - Construct `new Bot(env.TELEGRAM_BOT_TOKEN)`.
     - `installDispatcher(bot, env)` — imported from `src/modules/dispatcher.js` (phase-04 — stub import now, real impl later).
     - Assign + return.
   - Temporary stub: if `installDispatcher` not yet implemented, create a placeholder function in `src/modules/dispatcher.js` that does nothing so this phase compiles.
3. Env validation: on first `getBot` call, throw if `TELEGRAM_BOT_TOKEN` / `TELEGRAM_WEBHOOK_SECRET` / `MODULES` missing. Fail fast is a feature.
4. `npm run lint` — fix any issues.
5. `wrangler dev` — hit `GET /` locally, confirm 200. Hit `POST /webhook` without secret header, confirm 401.

## Todo List
- [ ] `src/index.js` fetch handler + URL router
- [ ] `src/bot.js` memoized factory
- [ ] Placeholder `src/modules/dispatcher.js` exporting `installDispatcher(bot, env)` no-op
- [ ] Env var validation with clear error messages
- [ ] Manual smoke test via `wrangler dev`

## Success Criteria
- `GET /` returns 200 `"miti99bot ok"`.
- `POST /webhook` without header → 401 (via grammY).
- `POST /webhook` with correct `X-Telegram-Bot-Api-Secret-Token` header and a valid Telegram update JSON body → 200.
- Unknown path → 404.

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Wrong adapter string breaks webhook | Low | High | Pin `"cloudflare-mod"`; test with `wrangler dev` + curl |
| Memoized Bot leaks state between deploys | Low | Low | Warm-restart resets module scope; documented behavior |
| Cold-start latency from first Bot() construction | Med | Low | Acceptable for bot use case |

## Security Considerations
- `TELEGRAM_WEBHOOK_SECRET` MUST be configured before enabling webhook in Telegram; grammY's `secretToken` option gives 401 on mismatch.
- Worker has NO admin HTTP surface — no attack surface beyond `/webhook` (secret-gated by grammY) and the public health check.
- Never log secrets, even on error paths.

## Next Steps
- Phase 03 creates the DB abstraction that modules will use in phase 04+.
- Phase 04 replaces the dispatcher stub with real middleware.
