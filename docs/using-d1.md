# Using D1 (SQL Database)

D1 is Cloudflare's serverless SQL database. Use it when your module needs to query structured data, perform scans, or maintain append-only history. For simple key → JSON blobs or per-user state, KV is lighter and faster.

## When to Choose D1 vs KV

| Use Case | D1 | KV |
|----------|----|----|
| Simple key → JSON state | — | ✓ |
| Per-user blob (config, stats) | — | ✓ |
| Relational queries (JOIN, GROUP BY) | ✓ | — |
| Scans (all users' records, filtered) | ✓ | — |
| Leaderboards, sorted aggregates | ✓ | — |
| Append-only history/audit log | ✓ | — |
| Exact row counts with WHERE | ✓ | — |

The trading module uses D1 for `trading_trades` (append-only history). Each `/trade_buy` and `/trade_sell` writes a row; `/history` scans the last N rows per user.

## Accessing SQL in a Module

In your module's `init`, receive `sql` (alongside `db` for KV):

```js
/** @type {import("../../db/sql-store-interface.js").SqlStore | null} */
let sql = null;

const myModule = {
  name: "mymod",
  init: async ({ db, sql: sqlStore, env }) => {
    sql = sqlStore;  // cache for handlers
  },
  commands: [
    {
      name: "myquery",
      visibility: "public",
      description: "Query the database",
      handler: async (ctx) => {
        if (!sql) {
          await ctx.reply("Database not configured");
          return;
        }
        const rows = await sql.all("SELECT * FROM mymod_items LIMIT 10");
        await ctx.reply(`Found ${rows.length} rows`);
      },
    },
  ],
};
```

**Important:** `sql` is `null` when `env.DB` is not bound (e.g., in tests without a fake D1 setup). Always guard:

```js
if (!sql) {
  // handle gracefully — module still works, just without persistence
}
```

## Table Naming Convention

All tables must follow the pattern `{moduleName}_{table}`:

- `trading_trades` — trading module's trades table
- `mymod_items` — mymod's items table
- `mymod_leaderboard` — mymod's leaderboard table

Enforce this by convention in code review. The `sql.tablePrefix` property is available for dynamic table names:

```js
const tableName = `${sql.tablePrefix}items`;  // = "mymod_items"
await sql.all(`SELECT * FROM ${tableName}`);
```

## Writing Migrations

Migrations live in `src/modules/<name>/migrations/NNNN_descriptive.sql`. Files are sorted lexically and applied in order (one-way only; no down migrations).

**Naming:** Use a 4-digit numeric prefix, then a descriptive name:

```
src/modules/trading/migrations/
├── 0001_trades.sql      # first migration
├── 0002_add_fees.sql    # second migration (optional)
└── 0003_...
```

**Example migration:**

```sql
-- src/modules/mymod/migrations/0001_items.sql
CREATE TABLE mymod_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_mymod_items_user ON mymod_items(user_id);
```

Key points:
- One-way only — migrations never roll back.
- Create indexes for columns you'll filter/sort by.
- Use `user_id` (snake_case in SQL), not `userId`.
- Reference other tables with full names: `other_module_items`.

The migration runner (`scripts/migrate.js`) tracks applied migrations in a `_migrations` table and skips any that have already run.

## SQL API Reference

The `SqlStore` provides these methods. All accept parameterized bindings (? placeholders):

### `run(query, ...binds)`
Execute INSERT / UPDATE / DELETE / CREATE. Returns `{ changes, last_row_id }`.

```js
const result = await sql.run(
  "INSERT INTO mymod_items (user_id, name) VALUES (?, ?)",
  userId,
  "Widget"
);
console.log(result.last_row_id);  // newly inserted row ID
```

### `all(query, ...binds)`
Execute SELECT, return all rows as plain objects.

```js
const items = await sql.all(
  "SELECT * FROM mymod_items WHERE user_id = ?",
  userId
);
// items = [{ id: 1, user_id: 123, name: "Widget", created_at: 1234567 }, ...]
```

### `first(query, ...binds)`
Execute SELECT, return first row or `null` if no match.

```js
const item = await sql.first(
  "SELECT * FROM mymod_items WHERE id = ?",
  itemId
);
if (!item) {
  // not found
}
```

### `prepare(query, ...binds)`
Advanced: return a D1 prepared statement for use with `.batch()`.

```js
const stmt = sql.prepare("INSERT INTO mymod_items (user_id, name) VALUES (?, ?)");
const batch = [
  stmt.bind(userId1, "Item1"),
  stmt.bind(userId2, "Item2"),
];
await sql.batch(batch);
```

### `batch(statements)`
Execute multiple prepared statements in a single round-trip.

```js
const stmt = sql.prepare("INSERT INTO mymod_items (user_id, name) VALUES (?, ?)");
const results = await sql.batch([
  stmt.bind(userId1, "Item1"),
  stmt.bind(userId2, "Item2"),
  stmt.bind(userId3, "Item3"),
]);
```

## Running Migrations

### Production

```bash
npm run db:migrate
```

This walks `src/modules/*/migrations/*.sql` (sorted), checks which have already run (tracked in `_migrations` table), and applies only new ones via `wrangler d1 execute --remote`.

### Local Dev

```bash
npm run db:migrate -- --local
```

Applies migrations to your local D1 binding in `.dev.vars`.

### Preview (Dry Run)

```bash
npm run db:migrate -- --dry-run
```

Prints the migration plan without executing anything. Useful before a production deploy.

## Testing with Fake D1

For hermetic unit tests without a real D1 binding, use `tests/fakes/fake-d1.js`. It's a minimal in-memory SQL implementation that covers common patterns:

```js
import { describe, it, expect, beforeEach, vi } from "vitest";
import { FakeD1 } from "../../fakes/fake-d1.js";

describe("trading trades", () => {
  let sql;

  beforeEach(async () => {
    const fakeDb = new FakeD1();
    // setup
    await fakeDb.run(
      "CREATE TABLE trading_trades (id INTEGER PRIMARY KEY, user_id INTEGER, qty INTEGER)"
    );
    sql = fakeDb;
  });

  it("inserts and retrieves trades", async () => {
    await sql.run(
      "INSERT INTO trading_trades (user_id, qty) VALUES (?, ?)",
      123,
      10
    );
    const rows = await sql.all(
      "SELECT qty FROM trading_trades WHERE user_id = ?",
      123
    );
    expect(rows).toEqual([{ qty: 10 }]);
  });
});
```

Note: `FakeD1` supports a subset of SQL features needed for current modules. Extend it in `tests/fakes/fake-d1.js` if you need additional syntax (CTEs, window functions, etc.).

## First-Time D1 Setup

If your deployment environment doesn't have a D1 database yet:

```bash
npx wrangler d1 create miti99bot-db
```

Copy the database ID from the output, then add it to `wrangler.toml`:

```toml
[[d1_databases]]
binding = "DB"
database_name = "miti99bot-db"
database_id = "<paste-id-here>"
```

Then run migrations:

```bash
npm run db:migrate
```

The `_migrations` table is created automatically. After that, new migrations apply on every deploy.

## Worked Example: Simple Counter

**Migration:**

```sql
-- src/modules/counter/migrations/0001_counters.sql
CREATE TABLE counter_state (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  count INTEGER NOT NULL DEFAULT 0
);
INSERT INTO counter_state (count) VALUES (0);
```

**Module:**

```js
import { createCounterHandler } from "./handler.js";

/** @type {import("../../db/sql-store-interface.js").SqlStore | null} */
let sql = null;

export default {
  name: "counter",
  init: async ({ sql: sqlStore }) => {
    sql = sqlStore;
  },
  commands: [
    {
      name: "count",
      visibility: "public",
      description: "Increment global counter",
      handler: (ctx) => createCounterHandler(sql)(ctx),
    },
  ],
};
```

**Handler:**

```js
export function createCounterHandler(sql) {
  return async (ctx) => {
    if (!sql) {
      await ctx.reply("Database not configured");
      return;
    }

    // increment
    await sql.run("UPDATE counter_state SET count = count + 1 WHERE id = 1");

    // read current
    const row = await sql.first("SELECT count FROM counter_state WHERE id = 1");
    await ctx.reply(`Counter: ${row.count}`);
  };
}
```

Run `/count` multiple times and watch the counter increment. The count persists across restarts because it's stored in D1.
