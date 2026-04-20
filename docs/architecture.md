# Architecture

A deeper look at how miti99bot is wired: what loads when, where data lives, how commands get from Telegram into a handler, and why the boring parts are boring on purpose.

For setup and day-to-day commands, see the top-level [README](../README.md).
For authoring a new plugin module, see [`adding-a-module.md`](./adding-a-module.md).

## 1. Design goals

- **Plug-n-play modules.** A module = one folder + one line in a static import map + one name in `MODULES`. Adding or removing one must never require touching framework code.
- **YAGNI / KISS / DRY.** Small surface area. No speculative abstractions beyond the KV interface (which is explicitly required so storage can be swapped).
- **Fail loud at load, not at runtime.** Invalid commands, unknown modules, name conflicts, missing env — all throw during registry build so the first request never sees a half-configured bot.
- **Single source of truth.** `/help` renders the registry. The register script reads the registry. `setMyCommands` is derived from the registry. Modules define commands in exactly one place.
- **No admin HTTP surface.** One less attack surface, one less secret. Webhook + menu registration happen out-of-band via a post-deploy node script.

## 2. Component overview

```
src/
├── index.js                 ── fetch router: POST /webhook + GET / health
├── bot.js                   ── memoized grammY Bot factory, lazy dispatcher install
├── db/
│   ├── kv-store-interface.js   ── JSDoc typedefs only — the contract
│   ├── cf-kv-store.js          ── Cloudflare KV adapter
│   └── create-store.js         ── per-module prefixing factory
├── modules/
│   ├── index.js             ── static import map (add new modules here)
│   ├── registry.js          ── loader + builder + conflict detection + memoization
│   ├── dispatcher.js        ── bot.command() for every visibility
│   ├── validate-command.js  ── shared validators
│   ├── util/                ── fully implemented: /info + /help
│   ├── trading/             ── paper trading: VN stocks (dynamic symbol resolution)
│   ├── wordle/              ── 5-letter guessing game (KV storage)
│   ├── loldle/              ── classic-mode LoL champion guesser (KV storage)
│   └── misc/                ── stub that exercises the DB (ping/mstats)
└── util/
    └── escape-html.js

scripts/
├── register.js              ── post-deploy: setWebhook + setMyCommands
└── stub-kv.js               ── no-op KV binding for deploy-time registry build
```

## 3. Cold-start and the bot factory

The Cloudflare Worker runtime hands your `fetch(request, env, ctx)` function fresh on every **cold** start. Warm requests on the same instance reuse module-scope state. We exploit that to initialize the grammY Bot exactly once per warm instance:

```
first request  ──► getBot(env)  ──► new Bot(TOKEN)
                                   └── installDispatcher(bot, env)
                                         ├── buildRegistry(env)
                                         │      ├── loadModules(env.MODULES)
                                         │      ├── init() each module
                                         │      └── flatten commands into 4 maps
                                         └── for each: bot.command(name, handler)
                                                      ▼
                                            return bot (cached at module scope)

later requests ──► getBot(env) returns cached bot
```

`getBot` uses both a resolved instance (`botInstance`) **and** an in-flight promise (`botInitPromise`) to handle the case where two concurrent requests race the first init. If init throws, the promise is cleared so the next request retries — a failed init should not permanently wedge the worker.

Required env vars (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, `MODULES`) are checked upfront: a missing var surfaces as a 500 with a clear error message on the first request, rather than a confusing runtime error deep inside grammY.

## 4. The module contract

Every module is a single default export with this shape:

```js
export default {
  name: "wordle",                              // must match folder + import map key
  init: async ({ db, sql, env }) => { ... },   // optional, called once at build time
  commands: [
    {
      name: "wordle",                          // ^[a-z0-9_]{1,32}$, no leading slash
      visibility: "public",                    // "public" | "protected" | "private"
      description: "Play wordle",              // required, ≤256 chars
      handler: async (ctx) => { ... },        // grammY context
    },
    // ...
  ],
  crons: [                                     // optional scheduled jobs
    {
      schedule: "0 2 * * *",                   // cron expression
      name: "cleanup",                         // unique within module
      handler: async (event, ctx) => { ... }, // receives { db, sql, env }
    },
  ],
};
```

- The command name regex is **uniform** across all visibility levels. A private command is still a slash command (`/konami`) — it is simply absent from Telegram's `/` menu and from `/help` output. It is NOT a hidden text-match easter egg.
- `description` is required for **all** visibilities. Private descriptions never reach Telegram; they exist so the registry remains self-documenting for debugging.
- `init({ db, sql, env })` is the one place where a module should do setup work. The `db` parameter is a `KVStore` whose keys are automatically prefixed with `<moduleName>:`. The `sql` parameter is a `SqlStore` (or `null` if `env.DB` is not bound) — for relational data. `env` is the raw worker env (read-only by convention).
- `crons` is optional. Each entry declares a scheduled job; the schedule MUST also be registered in `wrangler.toml` `[triggers] crons`.

Validation runs per-command at registry load, and cross-module conflict detection runs at the same step. Any violation throws — deployment fails loudly before any request is served.

## 5. Module loading: why the static map

Cloudflare Workers bundle statically via wrangler. A dynamic import from a variable path (`import(name)`) either fails at bundle time or forces the bundler to include every possible import target, defeating tree-shaking. So we have an explicit map:

```js
// src/modules/index.js
export const moduleRegistry = {
  util:    () => import("./util/index.js"),
  wordle:  () => import("./wordle/index.js"),
  loldle:  () => import("./loldle/index.js"),
  misc:    () => import("./misc/index.js"),
  trading: () => import("./trading/index.js"),
};
```

At runtime, `loadModules(env)` parses `env.MODULES` (comma-separated), trims, dedupes, and calls only the loaders for the listed names. Modules NOT listed are never imported — wrangler tree-shakes them out of the bundle if they reference code that is otherwise unused.

Adding a new module is a **two-line change**: create the folder, add one line to this map. Removing a module is a **zero-line change**: just drop the name from `MODULES`.

## 6. The registry and unified conflict detection

`buildRegistry(env)` produces four maps:

- `publicCommands: Map<name, entry>` — source of truth for `/help` public section + `setMyCommands` payload
- `protectedCommands: Map<name, entry>` — source of truth for `/help` protected section
- `privateCommands: Map<name, entry>` — bookkeeping only (hidden from `/help` and `setMyCommands`)
- `allCommands: Map<name, entry>` — **unified** flat index used by the dispatcher and by conflict detection

Conflict detection walks `allCommands` as commands are added. If two modules (in any visibility combination) both try to register `foo`, build throws:

```
command conflict: /foo registered by both "a" and "b"
```

This is stricter than a visibility-scoped key space. Rationale: a user typing `/foo` sees exactly one response, regardless of visibility. If the framework silently picks one or the other, the behavior becomes order-dependent and confusing. Throwing at load means the ambiguity must be resolved in code.

The memoized registry is also exposed via `getCurrentRegistry()` so `/help` can read it at handler time without rebuilding. `resetRegistry()` exists for tests.

## 7. The dispatcher

Minimalism is the point:

```js
export async function installDispatcher(bot, env) {
  const reg = await buildRegistry(env);
  for (const { cmd } of reg.allCommands.values()) {
    bot.command(cmd.name, cmd.handler);
  }
  return reg;
}
```

Every command — public, protected, **and private** — is registered via `bot.command()`. grammY handles:

- Slash prefix parsing
- Case sensitivity (Telegram commands are case-sensitive in practice)
- `/cmd@botname` suffix matching in group chats
- Argument capture via the grammY context

There is no custom text-match middleware, no `bot.on("message:text", ...)` handler, no private-command-specific path. One routing path for all three visibilities. This is what reduced the original two-path design (slash + text-match) to one during the revision pass.

## 8. Storage: KVStore and SqlStore

Modules NEVER touch `env.KV` or `env.DB` directly. They receive prefixed stores from the module context.

### KVStore (key-value, fast reads/writes)

For simple state and blobs, use `db` (a `KVStore`):

```js
// In a module's init:
init: async ({ db, env }) => {
  moduleDb = db;   // stash for handlers
},

// In a handler:
const state = await moduleDb.getJSON("game:42");
await moduleDb.putJSON("game:42", { score: 100 }, { expirationTtl: 3600 });
```

The interface (full JSDoc in `src/db/kv-store-interface.js`):

```js
get(key)                              // → string | null
put(key, value, { expirationTtl? })
delete(key)
list({ prefix?, limit?, cursor? })    // → { keys, cursor?, done }
getJSON(key)                          // → any | null (swallows corrupt JSON)
putJSON(key, value, { expirationTtl? })
```

#### Prefix mechanics

`createStore("wordle", env)` returns a wrapped store where every key is rewritten:

```
module calls:             wrapper sends to CFKVStore:      raw KV key:
─────────────────────────  ─────────────────────────────    ─────────────
put("games:42", v)     ──►  put("wordle:games:42", v)  ──►  wordle:games:42
get("games:42")        ──►  get("wordle:games:42")     ──►  wordle:games:42
list({prefix:"games:"})──►  list({prefix:"wordle:games:"})  (then strips "wordle:" from returned keys)
```

Two stores for different modules cannot read each other's data unless they reconstruct prefixes by hand — a code-review boundary, not a cryptographic one.

### SqlStore (relational, scans, append-only history)

For complex queries, aggregates, or audit logs, use `sql` (a `SqlStore`):

```js
// In a module's init:
init: async ({ sql }) => {
  sqlStore = sql;  // null if env.DB not bound
},

// In a handler or cron:
const trades = await sqlStore.all(
  "SELECT * FROM trading_trades WHERE user_id = ? ORDER BY ts DESC LIMIT 10",
  userId
);
```

The interface (full JSDoc in `src/db/sql-store-interface.js`):

```js
run(query, ...binds)      // INSERT/UPDATE/DELETE — returns { changes, last_row_id }
all(query, ...binds)      // SELECT all rows → array of objects
first(query, ...binds)    // SELECT first row → object | null
prepare(query, ...binds)  // Prepared statement for batch operations
batch(statements)         // Execute multiple statements in one round-trip
```

All tables must follow the naming convention `{moduleName}_{table}` (e.g., `trading_trades`).

Tables are created via migrations in `src/modules/<name>/migrations/*.sql`. The migration runner (`scripts/migrate.js`) applies them on deploy and tracks them in `_migrations` table.

### Swapping the backends

To replace Cloudflare KV with a different store (e.g. Upstash Redis, Postgres):

1. Create a new `src/db/<name>-store.js` that implements the `KVStore` interface.
2. Change the one `new CFKVStore(env.KV)` line in `src/db/create-store.js` to construct your new adapter.
3. Update `wrangler.toml` bindings.

That's the full change. No module code moves.

To replace D1 with a different SQL backend:

1. Create a new `src/db/<name>-sql-store.js` that implements the `SqlStore` interface.
2. Change the one `new CFSqlStore(env.DB)` line in `src/db/create-sql-store.js` to construct your new adapter.
3. Update `wrangler.toml` bindings.

## 9. HTTP and Scheduled Entry Points

### Webhook (HTTP)

```js
// src/index.js — simplified
export default {
  async fetch(request, env) {
    const { pathname } = new URL(request.url);
    if (request.method === "GET" && pathname === "/") {
      return new Response("miti99bot ok", { status: 200 });
    }
    if (request.method === "POST" && pathname === "/webhook") {
      const handler = await getWebhookHandler(env);
      return handler(request);
    }
    return new Response("not found", { status: 404 });
  },

  async scheduled(event, env, ctx) {
    // Cloudflare cron trigger
    const registry = await getRegistry(env);
    dispatchScheduled(event, env, ctx, registry);
  },
};
```

`getWebhookHandler` is memoized and constructs `webhookCallback(bot, "cloudflare-mod", { secretToken: env.TELEGRAM_WEBHOOK_SECRET })` once. grammY's `webhookCallback` validates the `X-Telegram-Bot-Api-Secret-Token` header on every request, so a missing or mismatched secret returns `401` before the update reaches any handler.

### Scheduled (Cron)

Cloudflare fires cron triggers specified in `wrangler.toml` `[triggers] crons`. The `scheduled(event, env, ctx)` handler receives:

- `event.cron` — the schedule string (e.g., "0 17 * * *")
- `event.scheduledTime` — Unix timestamp (ms) when the trigger fired
- `ctx.waitUntil(promise)` — keeps the handler alive until promise resolves

Flow:

```
Cloudflare cron trigger
        │
        ▼
scheduled(event, env, ctx)
        │
        ├── getRegistry(env) — build registry (same as HTTP)
        │      └── load + init all modules
        │
        └── dispatchScheduled(event, env, ctx, registry)
                   │
                   ├── filter registry.crons by event.cron match
                   │
                   └── for each matching cron:
                       ├── createStore(moduleName, env)  — KV store
                       ├── createSqlStore(moduleName, env) — D1 store
                       └── ctx.waitUntil(handler(event, { db, sql, env }))
                               └── wrapped in try/catch for isolation
```

Each handler fires independently. If one fails, others still run.

## 10. Deploy flow and the register script

Deploy is a single idempotent command:

```bash
npm run deploy
# = wrangler deploy && node --env-file=.env.deploy scripts/register.js
```

```
npm run deploy
    │
    ├── wrangler deploy
    │      └── uploads src/ + wrangler.toml vars to CF
    │
    └── scripts/register.js
          ├── reads .env.deploy into process.env (Node --env-file)
          ├── imports buildRegistry from src/modules/registry.js
          ├── calls buildRegistry({ MODULES, KV: stubKv }) to derive public cmds
          │       └── stubKv satisfies the binding without real IO
          ├── POST /bot<T>/setWebhook  { url, secret_token, allowed_updates }
          └── POST /bot<T>/setMyCommands  { commands: [...public only] }
```

The register script imports the **same** module loader + registry the Worker uses. That means the set of public commands pushed to Telegram's `/` menu is always consistent with the set of public commands the Worker will actually respond to. No chance of drift. No duplicate command list maintained somewhere.

`stubKv` is a no-op KV binding provided so `createStore` doesn't crash during the deploy-time build. Module `init` hooks are expected to tolerate missing state at deploy time — either by reading only (no writes), or by deferring writes until the first handler call.

`--dry-run` prints both payloads with the webhook secret redacted, without calling Telegram. Use this to sanity-check what will be pushed before a real deploy.

### Why the register step is not in the Worker

A previous design sketched a `POST /admin/setup` route inside the Worker, gated by a third `ADMIN_SECRET`. It was scrapped because:

- The Worker gains no capability from it — it can just as easily run from a node script.
- It adds a third secret to manage and rotate.
- It adds an attack surface (even a gated one) to a Worker whose only other route is the Telegram webhook.
- Running locally + idempotently means the exact same script works whether invoked by a human, CI, or a git hook.

## 11. Security posture

- `TELEGRAM_BOT_TOKEN` lives in two places: Cloudflare Workers secrets (`wrangler secret put`) for runtime, and `.env.deploy` (gitignored, local-only) for the register script. These two copies must match.
- `TELEGRAM_WEBHOOK_SECRET` is validated by grammY on every webhook request. Telegram echoes it via `X-Telegram-Bot-Api-Secret-Token` on every update; wrong or missing header → `401`. Rotate by updating both the CF secret and `.env.deploy`, then re-running `npm run deploy` (the register step re-calls `setWebhook` with the new value on the same run).
- `.dev.vars` and `.env.deploy` are in `.gitignore`; their `*.example` siblings are committed.
- Module authors get a prefixed store — they cannot accidentally read another module's keys, but the boundary is a code-review one. A motivated module could reconstruct prefixes by hand. This is fine for first-party modules; it is NOT a sandbox.
- Private commands provide **discoverability control**, not access control. Anyone who knows the name can invoke them.
- HTML injection in `/help` output is blocked by `escapeHtml` on module names and descriptions.

## 12. Testing philosophy

Pure-logic unit tests only. No `workerd` pool, no Telegram fixtures, no integration-level tooling. 200 tests run in ~2s.

Test seams:

- **`cf-kv-store.test.js`** — round-trips, `list()` pagination cursor, `expirationTtl` passthrough, `getJSON`/`putJSON` (including corrupt-JSON swallow), `undefined` value rejection.
- **`create-store.test.js`** — module-name validation, prefix mechanics, module-to-module isolation, JSON helpers through the prefix layer.
- **`validate-command.test.js`** — uniform regex, leading-slash rejection, description length cap, all visibilities.
- **`registry.test.js`** — module loading, trim/dedupe, unknown/missing/empty `MODULES`, unified-namespace conflict detection (same AND cross-visibility), `init` injection, `getCurrentRegistry`/`resetRegistry`.
- **`dispatcher.test.js`** — every visibility registered via `bot.command()`, dispatcher does NOT install any `bot.on()` middleware, handler identity preserved.
- **`help-command.test.js`** — module grouping, `(protected)` suffix, zero private-command leakage, HTML escaping of module names + descriptions, placeholder when no commands are visible.
- **`escape-html.test.js`** — the four HTML entities, non-double-escaping, non-string coercion.

Each module adds its own tests under `tests/modules/<name>/`. See module READMEs for coverage details.

Tests inject fakes (`fake-kv-namespace`, `fake-bot`, `fake-modules`) via parameter passing — no `vi.mock`, no path-resolution flakiness.

## 13. Module-specific documentation

Each module maintains its own `README.md` with commands, data model, and implementation details. See `src/modules/<name>/README.md` for module-specific docs.

## 14. Non-goals (for now)

- Real game logic in `misc` — it's a stub that exercises the DB. Real game modules (`wordle`, `loldle`, `trading`) are live; `misc` stays a framework sanity check.
- A sandbox between modules. Same-origin trust model: all modules are first-party code.
- Per-user rate limiting. Cloudflare's own rate limiting is available as a higher layer if needed.
- `nodejs_compat` flag. Not needed — grammY + this codebase use only Web APIs.
- A CI pipeline. Deploys are developer-driven in v1.
- Internationalization. The bot replies in English; add i18n per-module if a module needs it.

## 15. Further reading

- The phased implementation plan: `plans/260411-0853-telegram-bot-plugin-framework/` — 9 phase files with detailed rationale, risk assessments, and todo lists.
- Researcher reports: `plans/reports/researcher-260411-0853-*.md` — grammY on Cloudflare Workers, Cloudflare KV basics, wrangler config and secrets.
- grammY docs: <https://grammy.dev>
- Cloudflare Workers KV: <https://developers.cloudflare.com/kv/>
