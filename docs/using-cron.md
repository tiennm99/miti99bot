# Using Cron (Scheduled Jobs)

Cron allows modules to run scheduled tasks at fixed intervals. Use crons for cleanup (purge old data), maintenance (recompute stats), or periodic notifications.

## Declaring Crons

In your module's default export, add a `crons` array:

```js
export default {
  name: "mymod",
  init: async ({ db, sql, env }) => { /* ... */ },
  commands: [ /* ... */ ],
  crons: [
    {
      schedule: "0 17 * * *",      // 5 PM UTC daily
      name: "cleanup",             // human-readable identifier
      handler: async (event, ctx) => {
        // event.cron = "0 17 * * *"
        // event.scheduledTime = timestamp (ms)
        // ctx = { db, sql, env } (same as module init)
      },
    },
  ],
};
```

**Handler signature:**

```js
async (event, { db, sql, env }) => {
  // event.cron — the schedule string that fired
  // event.scheduledTime — Unix timestamp (ms)
  // db — namespaced KV store (same as init)
  // sql — namespaced D1 store (same as init), null if not bound
  // env — raw worker environment
}
```

## Cron Expression Syntax

Standard 5-field cron format (minute, hour, day-of-month, month, day-of-week):

```
minute  hour  day-of-month  month  day-of-week
  0-59   0-23    1-31       1-12     0-6 (0=Sunday)

"0 17 * * *"   — 5 PM UTC daily
"*/5 * * * *"  — every 5 minutes
"0 0 1 * *"    — midnight on the 1st of each month
"0 9 * * 1"    — 9 AM UTC every Monday
"30 2 * * *"   — 2:30 AM UTC daily
```

See Cloudflare's [cron expression docs](https://developers.cloudflare.com/workers/configuration/cron-triggers/) for full syntax.

## Registering in wrangler.toml

**Important:** Crons declared in the module MUST also be listed in `wrangler.toml`. This is because Cloudflare needs to know what schedules to fire at deploy time.

Edit `wrangler.toml`:

```toml
[triggers]
crons = ["0 17 * * *", "0 0 * * *", "*/5 * * * *"]
```

Both the module contract and the `[triggers] crons` array must list the same schedules. If a schedule is in the module but not in `wrangler.toml`, Cloudflare won't fire it. If it's in `wrangler.toml` but not in any module, the worker won't know what to do with it.

**Multiple modules can share a schedule** — all matching handlers will fire (fan-out). Each module must declare its own `crons` entry; the registry validates them at load time.

## Handler Details

### Error Isolation

If one handler fails, other handlers still run. Each handler is wrapped in try/catch:

```js
// In cron-dispatcher.js
for (const entry of matching) {
  ctx.waitUntil(
    (async () => {
      try {
        await entry.handler(event, handlerCtx);
      } catch (err) {
        console.error(`[cron] handler failed:`, err);
      }
    })(),
  );
}
```

Errors are logged to the Workers console but don't crash the dispatch loop.

### Execution Time Limits

Cloudflare cron tasks have a **15-minute wall-clock limit**. Operations exceeding this timeout are killed. For large data operations:

- Batch in chunks (e.g., delete 1000 rows at a time, looping)
- Use pagination to avoid loading entire datasets into memory
- Monitor execution time and add logging

### Context Availability

Cron handlers run in the same Worker runtime as HTTP handlers, so they have access to:

- `db` — the module's namespaced KV store (read/write)
- `sql` — the module's namespaced D1 store (read/write), or null if not configured
- `env` — all Worker environment bindings (secrets, etc.)

### Return Value

Handlers should return `Promise<void>`. The runtime ignores return values.

## Local Testing

Use the local `wrangler dev` server to simulate cron triggers:

```bash
npm run dev
```

In another terminal, send a simulated cron request:

```bash
# Trigger the 5 PM daily cron
curl "http://localhost:8787/__scheduled?cron=0+17+*+*+*"

# URL-encode the cron string (spaces → +)
```

The Worker responds with `200` and logs handler output to the dev server console.

### Simulating Multiple Crons

If you have several crons with different schedules, test each by passing the exact schedule string:

```bash
curl "http://localhost:8787/__scheduled?cron=*/5+*+*+*+*"  # every 5 min
curl "http://localhost:8787/__scheduled?cron=0+0+1+*+*"   # monthly
```

## Worked Example: Trade Retention

The trading module uses a daily cron at `0 17 * * *` (5 PM UTC) to trim old trades:

**Module declaration (src/modules/trading/index.js):**

```js
crons: [
  {
    schedule: "0 17 * * *",
    name: "trim-trades",
    handler: (event, ctx) => trimTradesHandler(event, ctx),
  },
],
```

**wrangler.toml:**

```toml
[triggers]
crons = ["0 17 * * *"]
```

**Handler (src/modules/trading/retention.js):**

```js
/**
 * Delete trades older than 90 days.
 */
export async function trimTradesHandler(event, { sql }) {
  if (!sql) return;  // database not configured

  const ninetyDaysAgoMs = Date.now() - 90 * 24 * 60 * 60 * 1000;

  const result = await sql.run(
    "DELETE FROM trading_trades WHERE ts < ?",
    ninetyDaysAgoMs
  );

  console.log(`[cron] trim-trades: deleted ${result.changes} old trades`);
}
```

**wrangler.toml:**

```toml
[triggers]
crons = ["0 17 * * *"]
```

At 5 PM UTC every day, Cloudflare fires the `0 17 * * *` cron. The Worker loads the registry, finds the trading module's handler, executes `trimTradesHandler`, and logs the number of deleted rows.

## Worked Example: Stats Recalculation

Imagine a leaderboard module that caches top-10 stats:

```js
export default {
  name: "leaderboard",
  init: async ({ db, sql }) => {
    // ...
  },
  crons: [
    {
      schedule: "0 12 * * *",  // noon UTC daily
      name: "refresh-stats",
      handler: async (event, { sql, db }) => {
        if (!sql) return;

        // Recompute aggregate stats from raw data
        const topTen = await sql.all(
          `SELECT user_id, SUM(score) as total_score
           FROM leaderboard_plays
           GROUP BY user_id
           ORDER BY total_score DESC
           LIMIT 10`
        );

        // Cache in KV for fast /leaderboard command response
        await db.putJSON("cached_top_10", topTen);
        console.log(`[cron] refresh-stats: updated top 10`);
      },
    },
  ],
};
```

Every day at noon, the leaderboard updates its cached stats without waiting for a user request.

## Crons and Cold Starts

Crons execute on a fresh Worker instance (potential cold start). Module `init` hooks run before the first handler, so cron handlers can safely assume initialization is complete.

If `init` throws, the cron fires anyway but has `sql` and `db` in a half-initialized state. Handle this gracefully:

```js
handler: async (event, { sql, db }) => {
  if (!sql) {
    console.warn("sql store not available, skipping");
    return;
  }
  // proceed with confidence
}
```

## Adding a New Cron

1. **Declare in module:**
   ```js
   crons: [
     { schedule: "0 3 * * *", name: "my-cron", handler: myHandler }
   ],
   ```

2. **Add to wrangler.toml:**
   ```toml
   [triggers]
   crons = ["0 3 * * *", "0 17 * * *"]  # keep existing schedules
   ```

3. **Deploy:**
   ```bash
   npm run deploy
   ```

4. **Test locally:**
   ```bash
   npm run dev
   # in another terminal:
   curl "http://localhost:8787/__scheduled?cron=0+3+*+*+*"
   ```

## Monitoring Crons

Cron execution is logged to the Cloudflare Workers console. Check the tail:

```bash
npx wrangler tail
```

Look for `[cron]` prefixed log lines to see which crons ran and what they did.
