# Phase 05 — JSDoc Pass

**Priority:** P2 (can run in parallel with 01–04)
**Status:** Complete

## Overview

Add ESLint + `eslint-plugin-jsdoc` for JSDoc syntax/completeness linting. Add `@typedef` definitions and annotate public functions. No `tsc`, no `jsconfig.json` type checking — JSDoc is documentation only.

## Requirements

**Functional**
- `npm run lint` runs Biome (existing) + ESLint (new, JSDoc-only rules).
- ESLint config scoped narrowly: only `jsdoc/*` rules enabled, no stylistic rules (Biome owns those).
- CI-friendly: lint failures exit non-zero.

**Non-functional**
- No build step, no emit.

## Architecture

### ESLint config

`eslint.config.js` (flat config, ESLint 9+):

```js
import jsdoc from "eslint-plugin-jsdoc";

export default [
  {
    files: ["src/**/*.js"],
    plugins: { jsdoc },
    rules: {
      "jsdoc/check-alignment": "warn",
      "jsdoc/check-param-names": "error",
      "jsdoc/check-tag-names": "error",
      "jsdoc/check-types": "error",
      "jsdoc/no-undefined-types": "error",
      "jsdoc/require-param-type": "error",
      "jsdoc/require-returns-type": "error",
      "jsdoc/valid-types": "error",
    },
  },
];
```

Do NOT require JSDoc on every function — only lint the ones that have it.

### Typedefs to add

Central file: `src/types.js` (JSDoc-only module, re-exported nothing).

- `Env` — Cloudflare bindings + vars (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, `KV`, `DB`, `MODULES`).
- `Module` — `{ name, init?, commands[], crons? }`.
- `Command` — `{ name, visibility, description, handler }`.
- `Cron` — `{ schedule, name, handler }`.
- `ModuleContext` — `{ db: KVStore, sql: SqlStore, env: Env }`.
- `KVStore` — existing file; ensure typedef complete.
- `SqlStore` — created in Phase 01.
- `Trade` — `{ id, userId, symbol, side, qty, priceVnd, ts }`.
- `Portfolio` — existing shape.

### Files to annotate

- `src/index.js`
- `src/bot.js`
- `src/db/*.js`
- `src/modules/registry.js`
- `src/modules/dispatcher.js`
- `src/modules/cron-dispatcher.js` (from Phase 02)
- `src/modules/validate-command.js`
- `src/modules/trading/*.js`

## Related Code Files

**Create**
- `eslint.config.js`
- `src/types.js`

**Modify**
- `package.json` — add `eslint` + `eslint-plugin-jsdoc` devDeps; update `lint` script to `biome check ... && eslint src`
- All files listed above — add `@param`/`@returns`/`@typedef` where missing

## Todo List

- [x] Install `eslint`, `eslint-plugin-jsdoc`
- [x] `eslint.config.js` with JSDoc-only rules
- [x] Update `lint` script
- [x] `src/types.js` central typedef file
- [x] Annotate `src/index.js`, `src/bot.js`
- [x] Annotate `src/db/*`
- [x] Annotate `src/modules/registry.js`, `dispatcher.js`, `validate-command.js` (skipped `cron-dispatcher.js` — Phase 02 owns it)
- [x] Annotate `src/modules/trading/*`
- [x] Run `npm run lint` — clean

## Success Criteria

- `npm run lint` exits 0.
- Module contract typedef visible to editor tooling (hover shows shape).
- No new runtime behavior.

## Risks

- ESLint 9 flat config quirks with `eslint-plugin-jsdoc` — pin versions known to work together.
