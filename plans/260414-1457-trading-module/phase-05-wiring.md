---
phase: 5
title: "Integration Wiring"
status: Pending
priority: P2
effort: 15m
depends_on: [4]
---

# Phase 5: Integration Wiring

## Overview

Two one-line edits to register the trading module in the bot framework.

## File changes

### `src/modules/index.js`

Add one line to `moduleRegistry`:

```js
export const moduleRegistry = {
  util: () => import("./util/index.js"),
  wordle: () => import("./wordle/index.js"),
  loldle: () => import("./loldle/index.js"),
  misc: () => import("./misc/index.js"),
  trading: () => import("./trading/index.js"),  // <-- add
};
```

### `wrangler.toml`

Append `trading` to MODULES var:

```toml
[vars]
MODULES = "util,wordle,loldle,misc,trading"
```

### `.env.deploy` (manual — not committed)

User must also add `trading` to MODULES in `.env.deploy` for the register script to pick up public commands.

## Implementation steps

1. Edit `src/modules/index.js` — add `trading` entry
2. Edit `wrangler.toml` — append `,trading` to MODULES
3. Run `npm run lint` to verify
4. Run `npm test` to verify existing tests still pass (trading tests in Phase 6)

## Verification

- `npm test` — all 56 existing tests pass
- `npm run lint` — no errors
- `npm run register:dry` — trading commands appear in output

## Rollback

Remove `trading` from both files. Existing KV data becomes inert (never read).

## Success criteria

- [ ] `src/modules/index.js` has trading entry
- [ ] `wrangler.toml` MODULES includes trading
- [ ] Existing tests pass unchanged
- [ ] Lint passes
