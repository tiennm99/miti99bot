# Phase 09 — Deploy + docs

## Context Links
- Plan: [plan.md](plan.md)
- Reports: [wrangler + secrets](../reports/researcher-260411-0853-wrangler-config-secrets.md)
- All prior phases

## Overview
- **Priority:** P1 (ship gate)
- **Status:** pending
- **Description:** first real deploy + documentation. Update README with setup/deploy steps, add an "adding a new module" guide, finalize `wrangler.toml` with real KV IDs.

## Key Insights
- Deploy is a single command — `npm run deploy` — which chains `wrangler deploy` + the register script from phase-07. No separate webhook registration or admin curl calls.
- First-time setup creates two env files: `.dev.vars` (for `wrangler dev`) and `.env.deploy` (for `scripts/register.js`). Both gitignored. Both have `.example` siblings committed.
- Production secrets for the Worker still live in CF (`wrangler secret put`). The register script reads its own copies from `.env.deploy` — same values, two homes (one for the Worker runtime, one for the deploy script). Document this clearly to avoid confusion.
- Adding a new module is a 3-step process: create folder, add to `src/modules/index.js`, add to `MODULES`. Document it.

## Requirements
### Functional
- `README.md` updated with: overview, architecture diagram, setup, local dev, deploy, adding a module, command visibility explanation.
- `wrangler.toml` has real KV namespace IDs (production + preview).
- `.dev.vars.example` + `.env.deploy.example` committed; `.dev.vars` + `.env.deploy` gitignored.
- A single `docs/` or inline README section covers how to add a new module with a minimal example.

### Non-functional
- README stays < 250 lines — link to plan phases for deep details if needed.

## Architecture
N/A — docs phase.

## Related Code Files
### Create
- `docs/adding-a-module.md` (standalone guide, referenced from README)

### Modify
- `README.md`
- `wrangler.toml` (real KV IDs)

### Delete
- none

## Implementation Steps
1. **Create KV namespaces:**
   ```bash
   wrangler kv namespace create miti99bot-kv
   wrangler kv namespace create miti99bot-kv --preview
   ```
   Paste both IDs into `wrangler.toml`.
2. **Set Worker runtime secrets (used by the deployed Worker):**
   ```bash
   wrangler secret put TELEGRAM_BOT_TOKEN
   wrangler secret put TELEGRAM_WEBHOOK_SECRET
   ```
3. **Create `.env.deploy` (used by `scripts/register.js` locally):**
   ```bash
   cp .env.deploy.example .env.deploy
   # then fill in the same TELEGRAM_BOT_TOKEN + TELEGRAM_WEBHOOK_SECRET + WORKER_URL + MODULES
   ```
   NOTE: `WORKER_URL` is unknown until after the first `wrangler deploy`. For the very first deploy, either (a) deploy once with `wrangler deploy` to learn the URL, fill `.env.deploy`, then run `npm run deploy` again, or (b) use a custom route from the start and put that URL in `.env.deploy` upfront.
4. **Preflight:**
   ```bash
   npm run lint
   npm test
   npm run register:dry   # prints the payloads without calling Telegram
   ```
   All green before deploy.
5. **Deploy (one command from here on):**
   ```bash
   npm run deploy
   ```
   This runs `wrangler deploy && npm run register`. The register step calls Telegram's `setWebhook` + `setMyCommands` with values from `.env.deploy`.
6. **Smoke test in Telegram:**
   - Type `/` in a chat with the bot — confirm the popup lists exactly the public commands (`/info`, `/help`, `/wordle`, `/loldle`, `/ping`). No protected, no private.
   - `/info` shows chat id / thread id / sender id.
   - `/help` shows util + wordle + loldle + misc sections with public + protected commands; private commands absent.
   - `/wordle`, `/loldle`, `/ping` respond.
   - `/wstats`, `/lstats`, `/mstats` respond (visible in `/help` but NOT in the `/` popup).
   - `/konami`, `/ggwp`, `/fortytwo` respond when typed, even though they're invisible everywhere.
7. **Write `README.md`:**
   - Badge-free, plain.
   - Sections: What it is, Architecture snapshot, Prereqs, Setup (KV + secrets + `.env.deploy`), Local dev, Deploy (`npm run deploy`), Command visibility levels, Adding a module, Troubleshooting.
   - Link to `docs/adding-a-module.md` and to `plans/260411-0853-telegram-bot-plugin-framework/plan.md`.
8. **Write `docs/adding-a-module.md`:**
   - Step 1: Create `src/modules/<name>/index.js` exporting default module object.
   - Step 2: Add entry to `src/modules/index.js` static map.
   - Step 3: Add `<name>` to `MODULES` in both `wrangler.toml` (runtime) and `.env.deploy` (so the register script picks up new public commands).
   - Minimal skeleton code block.
   - Note on DB namespacing (auto-prefixed).
   - Note on command naming rules (`[a-z0-9_]{1,32}`, no leading slash, uniform across all visibilities).
   - Note on visibility levels (public → in `/` menu + `/help`; protected → in `/help` only; private → hidden slash command).
9. **Troubleshooting section** — common issues:
    - 401 on webhook → `TELEGRAM_WEBHOOK_SECRET` mismatch between `wrangler secret` and `.env.deploy`. They MUST match.
    - `/help` empty for a module → check module exports `default` (not named export).
    - Module loads but no commands → check `MODULES` includes it (in both `wrangler.toml` AND `.env.deploy`).
    - Command conflict error at deploy → two modules registered the same command name; rename one.
    - `npm run register` exits with `missing env: X` → fill `X` in `.env.deploy`.
    - Node < 20.6 → `--env-file` flag unsupported; upgrade Node.

## Todo List
- [ ] Create real KV namespaces + paste IDs into `wrangler.toml`
- [ ] Set both runtime secrets via `wrangler secret put` (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`)
- [ ] Create `.env.deploy` from example (token, webhook secret, worker URL, modules)
- [ ] `npm run lint` + `npm test` + `npm run register:dry` all green
- [ ] `npm run deploy` (chains `wrangler deploy` + register)
- [ ] End-to-end smoke test in Telegram (9 public/protected commands + 3 private slash commands)
- [ ] `README.md` rewrite
- [ ] `docs/adding-a-module.md` guide
- [ ] Commit + push

## Success Criteria
- Bot responds in Telegram to all 11 slash commands (util: `/info` + `/help` public; wordle: `/wordle` public, `/wstats` protected, `/konami` private; loldle: `/loldle` public, `/lstats` protected, `/ggwp` private; misc: `/ping` public, `/mstats` protected, `/fortytwo` private).
- Telegram `/` menu shows exactly 5 public commands (`/info`, `/help`, `/wordle`, `/loldle`, `/ping`) — no protected, no private.
- `/help` output lists all four modules and their public + protected commands only.
- Private slash commands (`/konami`, `/ggwp`, `/fortytwo`) respond when invoked but appear nowhere visible.
- `npm run deploy` is a single command that goes from clean checkout to live bot (after first-time KV + secret setup).
- README is enough for a new contributor to deploy their own instance without reading the plan files.
- `docs/adding-a-module.md` lets a new module be added in < 5 minutes.

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Runtime and deploy-script secrets drift out of sync | Med | High | Document that both `.env.deploy` and `wrangler secret` must hold the SAME `TELEGRAM_WEBHOOK_SECRET`; mismatch → 401 loop |
| KV preview ID accidentally used in prod | Low | Med | Keep `preview_id` and `id` clearly labeled in wrangler.toml |
| `.dev.vars` or `.env.deploy` committed by mistake | Low | High | Gitignore both; pre-commit grep for common token prefixes |
| Bot token leaks via error responses | Low | High | grammY's default error handler does not echo tokens; double-check logs |

## Security Considerations
- `TELEGRAM_BOT_TOKEN` never appears in code, logs, or commits.
- `.dev.vars` + `.env.deploy` in `.gitignore` — verified before first commit.
- README explicitly warns against committing either env file.
- Webhook secret rotation: update `.env.deploy`, run `wrangler secret put TELEGRAM_WEBHOOK_SECRET`, then `npm run deploy`. The register step re-calls Telegram's `setWebhook` with the new secret on the same run. Single command does the whole rotation.
- Bot token rotation (BotFather reissue): update both `.env.deploy` and `wrangler secret put TELEGRAM_BOT_TOKEN`, then `npm run deploy`.

## Next Steps
- Ship. Feature work begins in a new plan after the user confirms v1 is live.
- Potential follow-ups (NOT in this plan):
  - Replace KV with D1 for relational game state.
  - Add per-user rate limiting.
  - Real wordle/loldle/misc game logic.
  - Logging to Cloudflare Logs or external sink.
