# Adding a Module

A module is a plugin that registers commands with the bot. The framework handles loading, routing, visibility, `/help` rendering, and per-module database namespacing. A new module is usually under 100 lines.

## Three-step checklist

### 1. Create the module folder

```
src/modules/<name>/index.js
```

Module name must match `^[a-z0-9_-]+$`. The folder name, the `name` field in the default export, and the key in the static import map MUST all be identical.

### 2. Add it to the static import map

Edit `src/modules/index.js`:

```js
export const moduleRegistry = {
  util: () => import("./util/index.js"),
  wordle: () => import("./wordle/index.js"),
  // ... existing entries ...
  mynew: () => import("./mynew/index.js"), // add this line
};
```

Static imports are required — wrangler's bundler strips unused code based on static analysis. Dynamic `import(variablePath)` would bundle everything.

### 3. Enable the module via env

Add the name to `MODULES` in two places:

`wrangler.toml`:
```toml
[vars]
MODULES = "util,wordle,loldle,misc,mynew"
```

`.env.deploy` (so the register script sees it too):
```
MODULES=util,wordle,loldle,misc,mynew
```

Modules NOT listed in `MODULES` are simply not loaded — no errors, no overhead.

## Minimal module skeleton

```js
/** @type {import("../registry.js").BotModule} */
const mynewModule = {
  name: "mynew",
  commands: [
    {
      name: "hello",
      visibility: "public",
      description: "Say hello",
      handler: async (ctx) => {
        await ctx.reply("hi!");
      },
    },
  ],
};

export default mynewModule;
```

## Using the database

If your module needs storage, opt into it with an `init` hook:

```js
/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

const mynewModule = {
  name: "mynew",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "count",
      visibility: "public",
      description: "Increment a counter",
      handler: async (ctx) => {
        const state = (await db.getJSON("counter")) ?? { n: 0 };
        state.n += 1;
        await db.putJSON("counter", state);
        await ctx.reply(`counter: ${state.n}`);
      },
    },
  ],
};

export default mynewModule;
```

**Key namespacing is automatic.** The `db` you receive is a `KVStore` whose keys are all prefixed with `mynew:` before hitting Cloudflare KV. The raw key for `counter` is `mynew:counter`. Two modules cannot read each other's data unless they reconstruct the prefix by hand.

### Available `KVStore` methods

```js
await db.get("key")                        // → string | null
await db.put("key", "value", { expirationTtl: 60 })  // seconds
await db.delete("key")
await db.list({ prefix: "games:", limit: 50, cursor })  // paginated

// JSON helpers (recommended — DRY for structured state)
await db.getJSON("key")                    // → parsed | null (swallows corrupt JSON)
await db.putJSON("key", { a: 1 }, { expirationTtl: 60 })
```

See `src/db/kv-store-interface.js` for the full JSDoc contract.

## Command contract

Each command is:

```js
{
  name: "hello",            // ^[a-z0-9_]{1,32}$, no leading slash
  visibility: "public",     // "public" | "protected" | "private"
  description: "Say hello", // required, ≤ 256 chars
  handler: async (ctx) => { ... },  // grammY context
}
```

### Name rules

- Lowercase letters, digits, underscore
- 1 to 32 characters
- No leading `/`
- Must be unique across ALL loaded modules, regardless of visibility

### Visibility levels

| Level | In `/` menu | In `/help` | When to use |
|---|---|---|---|
| `public` | yes | yes | Normal commands users should discover |
| `protected` | no | yes | Admin / debug tools you don't want in the menu but want discoverable via `/help` |
| `private` | no | no | Easter eggs, testing hooks — fully hidden |

Private commands are still slash commands — users type `/mycmd`. They're simply absent from Telegram's `/` popup and from `/help` output.

## Optional: D1 Storage

If your module needs a SQL database for relational queries, scans, or append-only history, add an `init` hook that receives `sql`:

```js
/** @type {import("../../db/sql-store-interface.js").SqlStore | null} */
let sql = null;

const myModule = {
  name: "mymod",
  init: async ({ db, sql: sqlStore, env }) => {
    db = store;
    sql = sqlStore;  // null when env.DB is not configured
  },
  commands: [ /* ... */ ],
};
```

Create migration files in `src/modules/<name>/migrations/`:

```
src/modules/mymod/migrations/
├── 0001_initial.sql
├── 0002_add_index.sql
└── ...
```

Run migrations at deploy time:

```bash
npm run db:migrate           # production
npm run db:migrate -- --local # local dev
npm run db:migrate -- --dry-run # preview
```

For full details on D1 usage, table naming, and the SQL API, see [`docs/using-d1.md`](./using-d1.md).

## Optional: Scheduled Jobs

If your module needs to run maintenance tasks (cleanup, stats refresh) on a schedule, add a `crons` array:

```js
const myModule = {
  name: "mymod",
  init: async ({ db, sql, env }) => { /* ... */ },
  commands: [ /* ... */ ],
  crons: [
    {
      schedule: "0 2 * * *",  // 2 AM UTC daily
      name: "daily-cleanup",
      handler: async (event, { db, sql, env }) => {
        // handler receives same context as init
        await sql.run("DELETE FROM mymod_old WHERE created < ?", oldTimestamp);
      },
    },
  ],
};
```

**Important:** Every cron schedule declared in a module MUST also be registered in `wrangler.toml`:

```toml
[triggers]
crons = ["0 2 * * *"]  # matches module declaration
```

For full details on cron syntax, local testing, and worked examples, see [`docs/using-cron.md`](./using-cron.md).

## Testing your module

Add a test in `tests/modules/<name>.test.js` or extend an existing suite. The `tests/fakes/` directory provides `fake-kv-namespace.js`, `fake-bot.js`, `fake-d1.js`, and `fake-modules.js` for hermetic unit tests that don't touch Cloudflare or Telegram.

Run:

```bash
npm test
```

Also worth running before deploying:

```bash
npm run register:dry
```

This prints the `setMyCommands` payload your module will push to Telegram — a fast way to verify the public command descriptions look right.

## Full example

See `src/modules/misc/index.js` — it's a minimal module that uses the DB (`putJSON` / `getJSON` via `/ping` + `/mstats`) and registers one command at each visibility level. Copy it as a starting point for your own module. See `src/modules/trading/` for a full example with D1 storage and scheduled crons.
