# Phase 01 — Scaffold project

## Context Links
- Plan: [plan.md](plan.md)
- Reports: [wrangler + secrets](../reports/researcher-260411-0853-wrangler-config-secrets.md)

## Overview
- **Priority:** P1 (blocker for everything)
- **Status:** pending
- **Description:** bootstrap repo structure, package.json, wrangler.toml template, biome, vitest, .gitignore, .dev.vars.example. No runtime code yet.

## Key Insights
- `nodejs_compat` flag NOT needed — grammY + our code uses Web APIs only. Keeps bundle small.
- `MODULES` must be declared as a comma-separated string in `[vars]` (wrangler does not accept top-level TOML arrays in `[vars]`).
- `.dev.vars` is a dotenv-format file for local secrets — gitignore it. Commit `.dev.vars.example`.
- biome handles lint + format in one binary. Single config file (`biome.json`). No eslint/prettier.

## Requirements
### Functional
- `npm install` produces a working dev environment.
- `npm run dev` starts `wrangler dev` on localhost.
- `npm run lint` / `npm run format` work via biome.
- `npm test` runs vitest (zero tests initially — exit 0).
- `wrangler deploy` pushes to CF (will fail without real KV ID — expected).

### Non-functional
- No TypeScript. `.js` + JSDoc.
- Zero extra dev deps beyond: `wrangler`, `grammy`, `@biomejs/biome`, `vitest`.

## Architecture
```
miti99bot/
├── src/
│   └── (empty — filled by later phases)
├── scripts/
│   └── (empty — register.js added in phase-07)
├── tests/
│   └── (empty)
├── package.json
├── wrangler.toml
├── biome.json
├── vitest.config.js
├── .dev.vars.example
├── .env.deploy.example
├── .gitignore       # add: node_modules, .dev.vars, .env.deploy, .wrangler, dist
├── README.md        # (already exists — updated in phase-09)
└── LICENSE          # (already exists)
```

## Related Code Files
### Create
- `package.json`
- `wrangler.toml`
- `biome.json`
- `vitest.config.js`
- `.dev.vars.example`
- `.env.deploy.example`

### Modify
- `.gitignore` (add `node_modules/`, `.dev.vars`, `.env.deploy`, `.wrangler/`, `dist/`, `coverage/`)

### Delete
- none

## Implementation Steps
1. `npm init -y`, then edit `package.json`:
   - `"type": "module"`
   - scripts:
     - `dev` → `wrangler dev`
     - `deploy` → `wrangler deploy && npm run register`
     - `register` → `node --env-file=.env.deploy scripts/register.js` (auto-runs `setWebhook` + `setMyCommands` — see phase-07)
     - `lint` → `biome check src tests scripts`
     - `format` → `biome format --write src tests scripts`
     - `test` → `vitest run`
2. `npm install grammy`
3. `npm install -D wrangler @biomejs/biome vitest`
4. Pin versions by checking `npm view <pkg> version` and recording exact versions.
5. Create `wrangler.toml` from template in research report — leave KV IDs as `REPLACE_ME`.
6. Create `biome.json` with defaults + 2-space indent, double quotes, semicolons.
7. Create `vitest.config.js` with `environment: "node"` (pure logic tests only, no workerd pool).
8. Create `.dev.vars.example` (local dev secrets used by `wrangler dev`):
   ```
   TELEGRAM_BOT_TOKEN=
   TELEGRAM_WEBHOOK_SECRET=
   ```
9. Create `.env.deploy.example` (consumed by `scripts/register.js` in phase-07; loaded via `node --env-file`):
   ```
   TELEGRAM_BOT_TOKEN=
   TELEGRAM_WEBHOOK_SECRET=
   WORKER_URL=https://<worker-subdomain>.workers.dev
   ```
10. Append `.gitignore` entries.
10. `npm run lint` + `npm test` + `wrangler --version` — all succeed.

## Todo List
- [ ] `npm init`, set `type: module`, scripts
- [ ] Install runtime + dev deps
- [ ] `wrangler.toml` template
- [ ] `biome.json`
- [ ] `vitest.config.js`
- [ ] `.dev.vars.example`
- [ ] Update `.gitignore`
- [ ] Smoke-run `npm run lint` / `npm test`

## Success Criteria
- `npm install` exits 0.
- `npm run lint` exits 0 (nothing to lint yet — biome treats this as pass).
- `npm test` exits 0.
- `npx wrangler --version` prints a version.
- `git status` shows no tracked `.dev.vars`, no `node_modules`.

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| wrangler version pins conflict with Node version | Low | Med | Require Node ≥ 20 in `engines` field |
| biome default rules too strict | Med | Low | Start with `recommended`; relax only if blocking |
| `type: module` trips vitest | Low | Low | vitest supports ESM natively |

## Security Considerations
- `.dev.vars` MUST be gitignored. Double-check before first commit.
- `wrangler.toml` MUST NOT contain any secret values — only `[vars]` for non-sensitive `MODULES`.

## Next Steps
- Phase 02 needs the fetch handler skeleton in `src/index.js`.
- Phase 03 needs the KV binding wired in `wrangler.toml` (already scaffolded here).
