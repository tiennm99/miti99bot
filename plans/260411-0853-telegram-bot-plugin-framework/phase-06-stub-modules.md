# Phase 06 — Stub modules (wordle / loldle / misc)

## Context Links
- Plan: [plan.md](plan.md)
- Phase 04: [module framework](phase-04-module-framework.md)

## Overview
- **Priority:** P2
- **Status:** pending
- **Description:** three stub modules proving the plugin system. Each registers one `public`, one `protected`, and one `private` command. All commands are slash commands; private ones are simply absent from `setMyCommands` and `/help`. Handlers are one-liners that echo or reply.

## Key Insights
- Stubs are NOT feature-complete games. Their job: exercise the plugin loader, visibility levels, registry, dispatcher, and `/help` rendering.
- Each stub module demonstrates DB usage via `getJSON` / `putJSON` in one handler — proves `createStore` namespacing + JSON helpers work end-to-end.
- Private commands follow the same slash-name rules as public/protected (`[a-z0-9_]{1,32}`). They're hidden, not text-matched.

## Requirements
### Functional
- `wordle` module:
  - public `/wordle` → `"Wordle stub — real game TBD."`
  - protected `/wstats` → `await db.getJSON("stats")` → returns `"games played: ${stats?.gamesPlayed ?? 0}"`.
  - private `/konami` → `"⬆⬆⬇⬇⬅➡⬅➡BA — secret wordle mode unlocked (stub)"`.
- `loldle` module:
  - public `/loldle` → `"Loldle stub."`
  - protected `/lstats` → `"loldle stats stub"`.
  - private `/ggwp` → `"gg well played (stub)"`.
- `misc` module:
  - public `/ping` → `"pong"` + `await db.putJSON("last_ping", { at: Date.now() })`.
  - protected `/mstats` → `const last = await db.getJSON("last_ping");` → echoes `last.at` or `"never"`.
  - private `/fortytwo` → `"The answer."` (slash-command regex excludes bare numbers, so we spell it).

### Non-functional
- Each stub's `index.js` < 80 LOC.
- No additional utilities — stubs use only what phase-03/04/05 provide.

## Architecture
```
src/modules/
├── wordle/
│   └── index.js
├── loldle/
│   └── index.js
└── misc/
    └── index.js
```

Each `index.js` exports `{ name, commands, init }` per the contract defined in phase 04. `init` stashes the injected `db` on a module-scope `let` so handlers can reach it.

Example shape (pseudo):
```js
let db;
export default {
  name: "wordle",
  init: async ({ db: store }) => { db = store; },
  commands: [
    { name: "wordle", visibility: "public",    description: "Play wordle",
      handler: async (ctx) => ctx.reply("Wordle stub — real game TBD.") },
    { name: "wstats", visibility: "protected", description: "Wordle stats",
      handler: async (ctx) => {
        const stats = await db.getJSON("stats");
        await ctx.reply(`games played: ${stats?.gamesPlayed ?? 0}`);
      } },
    { name: "konami", visibility: "private",   description: "Easter egg — retro code",
      handler: async (ctx) => ctx.reply("⬆⬆⬇⬇⬅➡⬅➡BA — secret wordle mode unlocked (stub)") },
  ],
};
```

## Related Code Files
### Create
- `src/modules/wordle/index.js`
- `src/modules/loldle/index.js`
- `src/modules/misc/index.js`

### Modify
- none (static map in `src/modules/index.js` already lists all four — from phase 04)

### Delete
- none

## Implementation Steps
1. Create `src/modules/wordle/index.js` per shape above (`/wordle`, `/wstats`, `/konami`).
2. Create `src/modules/loldle/index.js` (`/loldle`, `/lstats`, `/ggwp`).
3. Create `src/modules/misc/index.js` (`/ping`, `/mstats`, `/fortytwo`) including the `last_ping` `putJSON` / `getJSON` demonstrating DB usage.
4. Verify `src/modules/index.js` static map includes all three (added in phase-04).
5. `wrangler dev` smoke test: send each command via a mocked Telegram update; verify routing and KV writes land in the preview namespace with prefix `wordle:` / `loldle:` / `misc:`.
6. Lint clean.

## Todo List
- [ ] `wordle/index.js`
- [ ] `loldle/index.js`
- [ ] `misc/index.js`
- [ ] Verify KV writes are correctly prefixed (check via `wrangler kv key list --preview`)
- [ ] Manual webhook smoke test for all 9 commands
- [ ] Lint clean

## Success Criteria
- With `MODULES="util,wordle,loldle,misc"` the bot loads all four modules on cold start.
- `/help` output shows three stub sections (wordle, loldle, misc) with 2 commands each (public + protected), plus `util` section.
- `/help` does NOT list `/konami`, `/ggwp`, `/fortytwo`.
- Telegram's `/` menu (after `scripts/register.js` runs) shows `/wordle`, `/loldle`, `/ping`, `/info`, `/help` — nothing else.
- Typing `/konami` in Telegram invokes the handler. Typing `/Konami` does NOT (Telegram + grammY match case-sensitively).
- `/ping` writes `last_ping` via `putJSON`; subsequent `/mstats` reads it back via `getJSON`.
- Removing `wordle` from `MODULES` (`MODULES="util,loldle,misc"`) successfully boots without loading wordle; `/wordle` becomes unknown command (grammY default: silently ignored).

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Stub handlers accidentally leak into production behavior | Low | Low | Stubs clearly labeled in reply text |
| Private command name too guessable | Low | Low | These are stubs; real easter eggs can pick obscure names |
| DB write fails silently | Low | Med | Handlers `await` writes; errors propagate to grammY error handler |

## Security Considerations
- Stubs do NOT read user input for DB keys — they use fixed keys. Avoids injection.
- `/ping` timestamp is server time — no sensitive data.

## Next Steps
- Phase 07 adds `scripts/register.js` to run `setWebhook` + `setMyCommands` at deploy time.
- Phase 08 tests the full routing flow with these stubs as fixtures.
