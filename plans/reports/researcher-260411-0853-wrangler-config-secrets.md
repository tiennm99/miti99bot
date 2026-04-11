# Researcher Report: wrangler.toml, secrets, and MODULES env var

**Date:** 2026-04-11
**Scope:** how to declare secrets vs vars, local dev via `.dev.vars`, and the list-env-var question.

## Secrets vs vars

| Kind | Where declared | Deployed via | Local dev | Use for |
|---|---|---|---|---|
| **Secret** | NOT in wrangler.toml | `wrangler secret put NAME` | `.dev.vars` file (gitignored) | `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, `ADMIN_SECRET` |
| **Var** | `[vars]` in wrangler.toml | `wrangler deploy` | `.dev.vars` overrides | `MODULES`, non-sensitive config |

- Both appear on `env.NAME` at runtime — indistinguishable in code.
- `.dev.vars` is a dotenv file (`KEY=value` lines, no quotes required). Gitignore it.
- `wrangler secret put` encrypts into CF's secret store — never visible again after set.

## `[vars]` value types
- Per wrangler docs, `[vars]` accepts **strings and JSON objects**, not top-level arrays.
- Therefore `MODULES` must be a **comma-separated string**:
  ```toml
  [vars]
  MODULES = "util,wordle,loldle,misc"
  ```
- Code parses with `env.MODULES.split(",").map(s => s.trim()).filter(Boolean)`.
- **Rejected alternative:** JSON-string `MODULES = '["util","wordle"]'` + `JSON.parse`. More ceremony, no benefit, looks ugly in TOML. Stick with CSV.

## Full wrangler.toml template (proposed)
```toml
name = "miti99bot"
main = "src/index.js"
compatibility_date = "2026-04-01"
# No nodejs_compat — grammY + our code is pure Web APIs. Smaller bundle.

[vars]
MODULES = "util,wordle,loldle,misc"

[[kv_namespaces]]
binding = "KV"
id = "REPLACE_ME"
preview_id = "REPLACE_ME"

# Secrets (set via `wrangler secret put`):
#   TELEGRAM_BOT_TOKEN
#   TELEGRAM_WEBHOOK_SECRET
#   ADMIN_SECRET
```

## Local dev flow
1. `.dev.vars` contains:
   ```
   TELEGRAM_BOT_TOKEN=xxx
   TELEGRAM_WEBHOOK_SECRET=yyy
   ADMIN_SECRET=zzz
   ```
2. `wrangler dev` picks up `.dev.vars` + `[vars]` + `preview_id` KV.
3. For local Telegram testing, expose via `cloudflared tunnel` or ngrok, then `setWebhook` to the public URL.

## Secrets setup commands (for README/phase-09)
```bash
wrangler secret put TELEGRAM_BOT_TOKEN
wrangler secret put TELEGRAM_WEBHOOK_SECRET
wrangler secret put ADMIN_SECRET
wrangler kv namespace create miti99bot-kv
wrangler kv namespace create miti99bot-kv --preview
```

## Unresolved questions
- Should `MODULES` default to a hard-coded list in code if the env var is empty? **Recommendation:** no — fail loudly on empty/missing MODULES so misconfiguration is obvious. `/info` still works since `util` is always in the list.
