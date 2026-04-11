# Phase 04 — Module framework (contract, registry, loader, dispatcher)

## Context Links
- Plan: [plan.md](plan.md)
- Reports: [grammY on CF Workers](../reports/researcher-260411-0853-grammy-on-cloudflare-workers.md), [KV basics](../reports/researcher-260411-0853-cloudflare-kv-basics.md)

## Overview
- **Priority:** P1
- **Status:** pending
- **Description:** define the plugin contract, build the registry, implement the static-map loader filtered by `env.MODULES`, and write the grammY dispatcher middleware that routes commands and enforces visibility. **All three visibility levels (public / protected / private) are slash commands.** Visibility only controls which commands appear in Telegram's `/` menu (public only) and in `/help` output (public + protected). Private commands are hidden slash commands — easter eggs that still route through `bot.command()`.

## Key Insights
- wrangler bundles statically — dynamic `import(variablePath)` defeats tree-shaking and can fail at bundle time. **Solution:** static map `{ name: () => import("./name/index.js") }`, filtered at runtime by `env.MODULES.split(",")`.
- **Single routing path:** every command — regardless of visibility — is registered via `bot.command(name, handler)`. grammY handles slash-prefix parsing, case-sensitivity, and `/cmd@botname` suffix in groups automatically. No custom text-match code.
- Visibility is pure metadata. It affects two downstream consumers:
  1. phase-07's `scripts/register.js` → `setMyCommands` payload (public only).
  2. phase-05's `/help` renderer (public + protected, skip private).
- Command-name conflicts: two modules registering the same command name = registry throws at load. **One unified namespace across all visibility levels** — a public `/foo` in module A collides with a private `/foo` in module B. Fail fast > mystery last-wins.
- The registry is built ONCE per warm instance, inside `getBot(env)`. Not rebuilt per request.
- grammY's `bot.command()` matches exactly against the command name token — case-sensitive per Telegram's own semantics. This naturally satisfies the "exact, case-sensitive" requirement for private commands.

## Requirements
### Functional
- Module contract (locked):
  ```js
  export default {
    name: "wordle",              // must match folder name, [a-z0-9_-]+
    commands: [
      { name: "wordle", visibility: "public",    description: "Play wordle", handler: async (ctx) => {...} },
      { name: "wstats", visibility: "protected", description: "Stats",        handler: async (ctx) => {...} },
      { name: "konami", visibility: "private",   description: "Easter egg",   handler: async (ctx) => {...} },
    ],
    init: async ({ db, env }) => {}, // optional
  };
  ```
- `name` on a command: slash-command name without leading `/`, `[a-z0-9_]{1,32}` (Telegram's own limit). Same regex for all visibility levels — private commands are still `/foo`.
- `description`: required for all three visibility levels (private descriptions are used internally for debugging + not surfaced to Telegram/users). Max 256 chars (Telegram's limit on public command descriptions). Enforce uniformly.
- Loader reads `env.MODULES`, splits, trims, dedupes. For each name, look up static map; unknown name → throw.
- Each module's `init` is called once with `{ db: createStore(module.name, env), env }`.
- Registry builds three indexed maps (same shape, partitioned by visibility) PLUS one flat map of all commands for conflict detection + dispatch:
  - `publicCommands: Map<string, {module, cmd}>` — source of truth for `setMyCommands` + `/help`.
  - `protectedCommands: Map<string, {module, cmd}>` — source of truth for `/help`.
  - `privateCommands: Map<string, {module, cmd}>` — bookkeeping only (not surfaced anywhere visible).
  - `allCommands: Map<string, {module, cmd, visibility}>` — flat index used by the dispatcher to `bot.command()` every entry regardless of visibility.
- Name conflict across modules (any visibility combination) → throw `Error("command conflict: ...")` naming both modules and the command.

### Non-functional
- `src/modules/registry.js` < 150 LOC.
- `src/modules/dispatcher.js` < 60 LOC.
- No global mutable state outside the registry itself.

## Architecture

```
env.MODULES = "util,wordle,loldle,misc"
          │
          ▼
   loadModules(env) ──► static map lookup ──► import() each ──► array of module objects
          │
          ▼
   buildRegistry(modules, env)
          │
          ├── for each module: call init({db, env}) if present
          └── flatten commands → 3 visibility-partitioned maps + 1 flat `allCommands` map
                │                (conflict check walks `allCommands`: one namespace, all visibilities)
                ▼
   installDispatcher(bot, registry)
          │
          └── for each entry in allCommands:
                bot.command(cmd.name, cmd.handler)
```

**Why no text-match middleware:** grammY's `bot.command()` already handles exact-match slash routing, case sensitivity, and group-chat `/cmd@bot` disambiguation. Private commands ride that same path — they're just absent from `setMyCommands` and `/help`.

## Related Code Files
### Create
- `src/modules/index.js` — static import map
- `src/modules/registry.js` — `loadModules` + `buildRegistry` + `getCurrentRegistry`
- `src/modules/dispatcher.js` — `installDispatcher(bot, env)` (replaces phase-02 stub)
- `src/modules/validate-command.js` — shared validators (name regex, visibility, description)

### Modify
- `src/bot.js` — call `installDispatcher(bot, env)` (was a no-op stub)

### Delete
- none

## Implementation Steps
1. `src/modules/index.js`:
   ```js
   export const moduleRegistry = {
     util:   () => import("./util/index.js"),
     wordle: () => import("./wordle/index.js"),
     loldle: () => import("./loldle/index.js"),
     misc:   () => import("./misc/index.js"),
   };
   ```
   (Stub module folders land in phase-05/06 — tests in phase-08 can inject a fake map.)
2. `src/modules/validate-command.js`:
   - `VISIBILITIES = ["public", "protected", "private"]`.
   - `COMMAND_NAME_RE = /^[a-z0-9_]{1,32}$/` (uniform across all visibilities).
   - `validateCommand(cmd, moduleName)`:
     - `visibility` ∈ `VISIBILITIES` (throw otherwise).
     - `name` matches `COMMAND_NAME_RE` (no leading `/`).
     - `description` is non-empty string, ≤ 256 chars.
     - `handler` is a function.
     - All errors mention both the module name and the offending command name.
3. `src/modules/registry.js`:
   - Module-scope `let currentRegistry = null;` (used by `/help` in phase-05).
   - `async function loadModules(env)`:
     - Parse `env.MODULES` → array, trim, dedupe, skip empty.
     - Empty list → throw `Error("MODULES env var is empty")`.
     - For each name, `const loader = moduleRegistry[name]`; unknown → throw `Error(\`unknown module: ${name}\`)`.
     - `const mod = (await loader()).default`.
     - Validate `mod.name === name`.
     - Validate each command via `validateCommand`.
     - Return ordered array of module objects.
   - `async function buildRegistry(env)`:
     - Call `loadModules`.
     - Init four maps: `publicCommands`, `protectedCommands`, `privateCommands`, `allCommands`.
     - For each module (in `env.MODULES` order):
       - If `init`: `await mod.init({ db: createStore(mod.name, env), env })`. Wrap in try/catch; rethrow with module name prefix.
       - For each cmd:
         - If `allCommands.has(cmd.name)` → throw `Error(\`command conflict: /${cmd.name} registered by both ${existing.module.name} and ${mod.name}\`)`.
         - `allCommands.set(cmd.name, { module: mod, cmd, visibility: cmd.visibility });`
         - Push into the visibility-specific map too.
     - `currentRegistry = { publicCommands, protectedCommands, privateCommands, allCommands, modules };`
     - Return it.
   - `export function getCurrentRegistry() { if (!currentRegistry) throw new Error("registry not built yet"); return currentRegistry; }`
   - `export function resetRegistry() { currentRegistry = null; }` (test helper; phase-08 uses it in `beforeEach`).
4. `src/modules/dispatcher.js`:
   - `export async function installDispatcher(bot, env)`:
     - `const reg = await buildRegistry(env);`
     - `for (const { cmd } of reg.allCommands.values()) { bot.command(cmd.name, cmd.handler); }`
     - Return `reg` (caller may ignore; `/help` reads via `getCurrentRegistry()`).
5. Update `src/bot.js` to `await installDispatcher(bot, env)` before returning. This makes `getBot` async — update `src/index.js` to `await getBot(env)`.
6. Lint clean.

## Todo List
- [ ] `src/modules/index.js` static import map
- [ ] `src/modules/validate-command.js` (uniform regex + description length cap)
- [ ] `src/modules/registry.js` — `loadModules` + `buildRegistry` + `getCurrentRegistry` + `resetRegistry`
- [ ] `src/modules/dispatcher.js` — single loop, all visibilities via `bot.command()`
- [ ] Update `src/bot.js` + `src/index.js` for async `getBot`
- [ ] Unified-namespace conflict detection
- [ ] Lint clean

## Success Criteria
- With `MODULES="util"` and util exposing one public cmd, `wrangler dev` + mocked webhook update correctly routes.
- Conflict test (phase-08): two fake modules both register `/foo` (regardless of visibility) → `buildRegistry` throws with both module names and the command name in the message.
- Unknown module name in `MODULES` → throws with clear message.
- Registry built once per warm instance (memoized via `getBot`).
- `/konami` (a private command) responds when typed; does NOT appear in `/help` output; does NOT appear in Telegram's `/` menu after `scripts/register.js` runs.

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Dynamic import breaks bundler tree-shaking | N/A | N/A | Mitigated by static map design |
| `init` throws → entire bot broken | Med | High | Wrap in try/catch, log module name, re-throw with context |
| Module mutates passed `env` | Low | Med | Pass `env` as-is; document contract: modules must not mutate |
| Private command accidentally listed in `setMyCommands` | Low | Med | `scripts/register.js` reads `publicCommands` only (not `allCommands`) |
| Description > 256 chars breaks `setMyCommands` payload | Low | Low | Validator enforces cap at module load |

## Security Considerations
- A private command with the same name as a public one in another module would be ambiguous. Unified conflict detection blocks this at load.
- Module authors get an auto-prefixed DB store — they CANNOT read other modules' data unless they reconstruct prefixes manually (code review responsibility).
- `init` errors must log module name for audit, never the env object (may contain secrets).
- Private commands do NOT provide security — anyone who guesses the name can invoke them. They are for discoverability control, not access control.

## Next Steps
- Phase 05 implements util module (`/info`, `/help`) consuming the registry.
- Phase 06 adds stub modules proving the plugin system end-to-end.
