# Researcher Report: grammY on Cloudflare Workers

**Date:** 2026-04-11
**Scope:** grammY entry point, webhook adapter, secret-token verification, setMyCommands usage.

## Key findings

### Adapter
- Use **`"cloudflare-mod"`** adapter for ES module (fetch handler) Workers. Source: grammY `src/convenience/frameworks.ts`.
- The legacy `"cloudflare"` adapter targets service-worker style Workers. Do NOT use — CF has moved on to module workers.
- Import path (npm, not Deno): `import { Bot, webhookCallback } from "grammy";`

### Minimal fetch handler
```js
import { Bot, webhookCallback } from "grammy";

export default {
  async fetch(request, env, ctx) {
    const bot = new Bot(env.TELEGRAM_BOT_TOKEN);
    // ... register handlers
    const handle = webhookCallback(bot, "cloudflare-mod", {
      secretToken: env.TELEGRAM_WEBHOOK_SECRET,
    });
    return handle(request);
  },
};
```

### Secret-token verification
- `webhookCallback` accepts `secretToken` in its `WebhookOptions`. When set, grammY validates the incoming `X-Telegram-Bot-Api-Secret-Token` header and rejects mismatches with 401.
- **No need** to manually read the header — delegate to grammY.
- The same secret must be passed to Telegram when calling `setWebhook` (`secret_token` field).

### Bot instantiation cost
- `new Bot()` per request is acceptable for Workers (no persistent state between requests anyway). Global-scope instantiation also works and caches across warm invocations. Prefer **global-scope** for reuse but be aware env bindings are not available at module load — must instantiate lazily inside `fetch`. Recommended pattern: memoize `Bot` in a module-scope variable initialized on first request.

### setMyCommands
- Call via `bot.api.setMyCommands([{ command, description }, ...])`.
- Should be called **on demand**, not on every webhook request (rate-limit risk, latency). Two options:
  1. Dedicated admin HTTP route (e.g. `POST /admin/setup`) guarded by a second secret. Runs on demand.
  2. One-shot `wrangler` script. Adds tooling complexity.
- **Recommendation:** admin route. Keeps deploy flow in one place (`wrangler deploy` + `curl`). No extra script.

### Init flow
- `bot.init()` is NOT required if you only use `webhookCallback`; grammY handles lazy init.
- For `/admin/setup` that directly calls `bot.api.*`, call `await bot.init()` once to populate `bot.botInfo`.

## Resolved technical answers
| Question | Answer |
|---|---|
| Adapter string | `"cloudflare-mod"` |
| Import | `import { Bot, webhookCallback } from "grammy"` |
| Secret verify | pass `secretToken` in `webhookCallback` options |
| setMyCommands trigger | admin HTTP route guarded by separate secret |

## Unresolved questions
- None blocking. grammY version pin: recommend `^1.30.0` or latest stable at implementation time; phase-01 should `npm view grammy version` to confirm.
