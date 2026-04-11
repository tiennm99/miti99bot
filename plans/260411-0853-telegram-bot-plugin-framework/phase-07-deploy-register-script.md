# Phase 07 — Post-deploy register script (`setWebhook` + `setMyCommands`)

## Context Links
- Plan: [plan.md](plan.md)
- Phase 01: [scaffold project](phase-01-scaffold-project.md) — defines `npm run deploy` + `.env.deploy.example`
- Phase 04: [module framework](phase-04-module-framework.md) — defines `publicCommands` source of truth
- Reports: [grammY on CF Workers](../reports/researcher-260411-0853-grammy-on-cloudflare-workers.md)

## Overview
- **Priority:** P2
- **Status:** pending
- **Description:** a plain Node script at `scripts/register.js` that runs automatically after `wrangler deploy`. It calls Telegram's HTTP API directly to (1) register the Worker URL as the bot's webhook with a secret token and (2) push the `public` command list to Telegram via `setMyCommands`. Idempotent. No admin HTTP surface on the Worker itself.

## Key Insights
- The Worker has no post-deploy hook — wire this via `npm run deploy` = `wrangler deploy && npm run register`.
- The script runs in plain Node (not workerd), so it can `import` the module framework directly to derive the public-command list. This keeps a SINGLE source of truth: module files.
- Config loaded via `node --env-file=.env.deploy` (Node ≥ 20.6) — zero extra deps (no dotenv package). `.env.deploy` is gitignored; `.env.deploy.example` is committed.
- `setWebhook` is called with `secret_token` field equal to `TELEGRAM_WEBHOOK_SECRET`. Telegram echoes this in `X-Telegram-Bot-Api-Secret-Token` on every update; grammY's `webhookCallback` validates it.
- `setMyCommands` accepts an array of `{ command, description }`. Max 100 commands, 256 chars per description — already enforced at module load in phase-04.
- Script must be idempotent: running twice on identical state is a no-op in Telegram's eyes. Telegram accepts repeated `setWebhook` with the same URL.
- `private` commands are excluded from `setMyCommands`. `protected` commands also excluded (by definition — they're only in `/help`).

## Requirements
### Functional
- `scripts/register.js`:
  1. Read required env: `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, `WORKER_URL`. Missing any → print which one and `process.exit(1)`.
  2. Read `MODULES` from `.env.deploy` (or shell env) — same value that the Worker uses. If absent, default to reading `wrangler.toml` `[vars].MODULES`. Prefer `.env.deploy` for single-source-of-truth at deploy time.
  3. Build the public command list by calling the same loader/registry code used by the Worker:
     ```js
     import { buildRegistry } from "../src/modules/registry.js";
     const fakeEnv = { MODULES: process.env.MODULES, KV: /* stub */ };
     const reg = await buildRegistry(fakeEnv);
     const publicCommands = [...reg.publicCommands.values()]
       .map(({ cmd }) => ({ command: cmd.name, description: cmd.description }));
     ```
     Pass a stub `KV` binding (a no-op object matching the `KVNamespace` shape) so `createStore` doesn't crash. Modules that do real KV work in `init` must tolerate this (or defer writes until first handler call — phase-06 stubs already do the latter).
  4. `POST https://api.telegram.org/bot<TOKEN>/setWebhook` with body:
     ```json
     {
       "url": "<WORKER_URL>/webhook",
       "secret_token": "<TELEGRAM_WEBHOOK_SECRET>",
       "allowed_updates": ["message", "edited_message", "callback_query"],
       "drop_pending_updates": false
     }
     ```
  5. `POST https://api.telegram.org/bot<TOKEN>/setMyCommands` with body `{ "commands": [...] }`.
  6. Print a short summary: webhook URL, count of registered public commands, list of commands. Exit 0.
  7. On any non-2xx from Telegram: print the response body + exit 1. `npm run deploy` fails loudly.
- `package.json`:
  - `"deploy": "wrangler deploy && npm run register"`
  - `"register": "node --env-file=.env.deploy scripts/register.js"`
  - `"register:dry": "node --env-file=.env.deploy scripts/register.js --dry-run"` (prints the payloads without calling Telegram — useful before first real deploy)

### Non-functional
- `scripts/register.js` < 150 LOC.
- Zero dependencies beyond Node built-ins (`fetch`, `process`).
- No top-level `await` at module scope (wrap in `main()` + `.catch(exit)`).

## Architecture
```
npm run deploy
    │
    ├── wrangler deploy                            (ships Worker code + wrangler.toml vars)
    │
    └── npm run register                           (= node --env-file=.env.deploy scripts/register.js)
            │
            ├── load .env.deploy into process.env
            ├── import buildRegistry from src/modules/registry.js
            │       └── uses stub KV, derives publicCommands
            ├── POST https://api.telegram.org/bot<T>/setWebhook {url, secret_token, allowed_updates}
            ├── POST https://api.telegram.org/bot<T>/setMyCommands {commands: [...]}
            └── print summary → exit 0
```

## Related Code Files
### Create
- `scripts/register.js`
- `scripts/stub-kv.js` — tiny no-op `KVNamespace` stub for the registry build (exports a single object with async methods returning null/empty)
- `.env.deploy.example` (already created in phase-01; documented again here)

### Modify
- `package.json` — add `deploy` + `register` + `register:dry` scripts (phase-01 already wires `deploy`)

### Delete
- none

## Implementation Steps
1. `scripts/stub-kv.js`:
   ```js
   // No-op KVNamespace stub for deploy-time registry build. Matches the minimum surface
   // our CFKVStore wrapper calls — never actually reads/writes.
   export const stubKv = {
     async get() { return null; },
     async put() {},
     async delete() {},
     async list() { return { keys: [], list_complete: true, cursor: undefined }; },
   };
   ```
2. `scripts/register.js`:
   ```js
   import { buildRegistry, resetRegistry } from "../src/modules/registry.js";
   import { stubKv } from "./stub-kv.js";

   const TELEGRAM_API = "https://api.telegram.org";

   function requireEnv(name) {
     const v = process.env[name];
     if (!v) { console.error(`missing env: ${name}`); process.exit(1); }
     return v;
   }

   async function tg(token, method, body) {
     const res = await fetch(`${TELEGRAM_API}/bot${token}/${method}`, {
       method: "POST",
       headers: { "content-type": "application/json" },
       body: JSON.stringify(body),
     });
     const json = await res.json().catch(() => ({}));
     if (!res.ok || json.ok === false) {
       console.error(`${method} failed:`, res.status, json);
       process.exit(1);
     }
     return json;
   }

   async function main() {
     const token   = requireEnv("TELEGRAM_BOT_TOKEN");
     const secret  = requireEnv("TELEGRAM_WEBHOOK_SECRET");
     const workerUrl = requireEnv("WORKER_URL").replace(/\/$/, "");
     const modules = requireEnv("MODULES");
     const dry     = process.argv.includes("--dry-run");

     resetRegistry();
     const reg = await buildRegistry({ MODULES: modules, KV: stubKv });
     const commands = [...reg.publicCommands.values()]
       .map(({ cmd }) => ({ command: cmd.name, description: cmd.description }));

     const webhookBody = {
       url: `${workerUrl}/webhook`,
       secret_token: secret,
       allowed_updates: ["message", "edited_message", "callback_query"],
       drop_pending_updates: false,
     };
     const commandsBody = { commands };

     if (dry) {
       console.log("DRY RUN — not calling Telegram");
       console.log("setWebhook:", webhookBody);
       console.log("setMyCommands:", commandsBody);
       return;
     }

     await tg(token, "setWebhook", webhookBody);
     await tg(token, "setMyCommands", commandsBody);

     console.log(`ok — webhook: ${webhookBody.url}`);
     console.log(`ok — ${commands.length} public commands registered:`);
     for (const c of commands) console.log(`  /${c.command} — ${c.description}`);
   }

   main().catch((e) => { console.error(e); process.exit(1); });
   ```
3. Update `package.json` scripts:
   ```json
   "deploy":       "wrangler deploy && npm run register",
   "register":     "node --env-file=.env.deploy scripts/register.js",
   "register:dry": "node --env-file=.env.deploy scripts/register.js --dry-run"
   ```
4. Smoke test BEFORE first real deploy:
   - Populate `.env.deploy` with real token + secret + worker URL + modules.
   - `npm run register:dry` — verify the printed payloads match expectations.
   - `npm run register` — verify Telegram returns ok, then in the Telegram client type `/` and confirm only public commands appear in the popup.
5. Lint clean (biome covers `scripts/` — phase-01 `lint` script already includes it).

## Todo List
- [ ] `scripts/stub-kv.js`
- [ ] `scripts/register.js` with `--dry-run` flag
- [ ] `package.json` scripts updated
- [ ] `.env.deploy.example` committed (created in phase-01, verify present)
- [ ] Dry-run prints correct payloads
- [ ] Real run succeeds against a test bot
- [ ] Verify Telegram `/` menu reflects public commands only
- [ ] Lint clean

## Success Criteria
- `npm run deploy` = `wrangler deploy` then automatic `setWebhook` + `setMyCommands`, no manual steps.
- `npm run register:dry` prints both payloads and exits 0 without calling Telegram.
- Missing env var produces a clear `missing env: NAME` message + exit 1.
- Telegram client `/` menu shows exactly the `public` commands (5 total with full stubs: `/info`, `/help`, `/wordle`, `/loldle`, `/ping`).
- Protected and private commands are NOT in Telegram's `/` menu.
- Running `npm run register` a second time with no code changes succeeds (idempotent).
- Wrong `TELEGRAM_WEBHOOK_SECRET` → subsequent Telegram update → 401 from Worker (validated via grammY, not our code) — this is the end-to-end proof of correct wiring.

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `.env.deploy` committed by accident | Low | High | `.gitignore` from phase-01 + pre-commit grep recommended in phase-09 |
| Stub KV missing a method the registry calls | Med | Med | `buildRegistry` only uses KV via `createStore`; stub covers all four methods used |
| Module `init` tries to write to KV and crashes on stub | Med | Med | Contract: `init` may read, must tolerate empty results; all writes happen in handlers. Documented in phase-04. |
| `setMyCommands` rate-limited if spammed | Low | Low | `npm run deploy` is developer-driven, not per-request |
| Node < 20.6 doesn't support `--env-file` | Low | Med | `package.json` `"engines": { "node": ">=20.6" }` (add in phase-01) |
| Webhook secret leaks via CI logs | Low | High | Phase-09 documents: never pass `.env.deploy` through CI logs; prefer CF Pages/Workers CI with masked secrets |

## Security Considerations
- `.env.deploy` contains the bot token — MUST be gitignored and never logged by the script beyond `console.log(\`ok\`)`.
- The register script runs locally on the developer's machine — not in CI by default. If CI is added later, use masked secrets + `--env-file` pointing at a CI-provided file.
- No admin HTTP surface on the Worker means no timing-attack attack surface, no extra secret rotation, no `ADMIN_SECRET` to leak.
- `setWebhook` re-registration rotates nothing automatically — if the webhook secret is rotated, you MUST re-run `npm run register` so Telegram uses the new value on future updates.

## Next Steps
- Phase 08 tests the registry code that the script reuses (the script itself is thin).
- Phase 09 documents the full deploy workflow (`.env.deploy` setup, first-run checklist, troubleshooting).
