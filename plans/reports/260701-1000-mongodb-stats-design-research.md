---
type: research
topic: mongodb-stats-design
created_at: 2026-07-01T03:00:06Z
---

# Research Report: MongoDB Stats Design

## Executive Summary

For this bot's `/stats` feature, best primary model is one aggregate document per
`(command, user)` pair, plus one anonymous command bucket when sender has no
username. This keeps documents bounded, uses MongoDB atomic `$inc`, and makes all
current views queryable without dynamic object keys.

Do not store stats as "one user document with commands map" or "one command
document with users map" as primary shape. Those designs make one query easy and
the opposite query expensive, create hot growing documents, and push us toward
dynamic field names. Pair docs are boring and better.

Recommended document:

```js
{ _id: "ping:42", cmd: "ping", uid: 42, user: "alice", n: 3 }
{ _id: "ping", cmd: "ping", n: 2 } // no public username
```

## Methodology

- Sources consulted: MongoDB official docs, Go driver docs, local codebase.
- Date: 2026-07-01.
- Key terms: MongoDB schema design, unbounded arrays, `$inc`, atomicity,
  aggregation `$group`, `$sort`, compound indexes, ESR guideline.
- Scope: stats module only. Bot persistence remains one collection per module.

## Codebase Context

- Repo: Go Telegram bot.
- Storage: `internal/storage` has one MongoDB collection per module.
- Current stats: `count:*`, `user:*`, `pair:*` key families in one collection.
- Current views:
  - `/stats`: top commands.
  - `/stats users`: top users.
  - `/stats user <name>`: commands by user.
  - `/stats cmd <name>`: users by command.
- Mongo docs already store flattened BSON fields, so stats fields can be queried.

## Key Findings

### 1. Avoid Growing Nested User/Command Maps

MongoDB docs warn against unbounded arrays because growing documents hurt
performance and risk document size limits. The same practical concern applies to
ever-growing embedded maps like `commands` under a user or `users` under a
command: one document grows forever and becomes a write hotspot.

Source: MongoDB "Avoid Unbounded Arrays":
https://www.mongodb.com/docs/manual/data-modeling/design-antipatterns/unbounded-arrays/

### 2. Use Atomic `$inc` For Counts

MongoDB `$inc` creates missing numeric fields, increments by the given amount,
and is atomic within one document. This fits per-pair counter documents exactly.

Source: MongoDB `$inc` operator:
https://www.mongodb.com/docs/manual/reference/operator/update/inc/

MongoDB write operations are atomic at single-document level. Multi-document
updates are not atomic as a whole, so each counter increment should update one
aggregate document.

Source: MongoDB atomicity:
https://www.mongodb.com/docs/manual/core/write-operations-atomicity/

### 3. Use Aggregation For Cross-Dimension Totals

MongoDB `$group` combines documents by a group key. That fits:

```js
// top commands
[{ $group: { _id: "$cmd", n: { $sum: "$n" } } }]

// top users
[{ $match: { uid: { $gt: 0 } } },
 { $group: { _id: "$uid", user: { $first: "$user" }, n: { $sum: "$n" } } }]
```

Source: MongoDB `$group`:
https://www.mongodb.com/docs/manual/reference/operator/aggregation/group/

Use `$sort` then `$limit` for top K. MongoDB can coalesce adjacent sort/limit so
only top N must be kept in memory.

Source: MongoDB `$sort` optimization:
https://www.mongodb.com/docs/manual/reference/operator/aggregation/sort/

### 4. Index From Query Shape, Not Guesswork

MongoDB aggregation can use indexes, especially early `$match` and `$sort`
stages. The ESR guideline says equality fields first, then sort, then range for
compound indexes.

Sources:
- Aggregation pipeline optimization:
  https://www.mongodb.com/docs/manual/core/aggregation-pipeline-optimization/
- ESR guideline:
  https://www.mongodb.com/docs/manual/tutorial/equality-sort-range-guideline/
- Indexing strategies:
  https://www.mongodb.com/docs/manual/applications/indexes/

For this module, start with no extra indexes or a tiny set. Traffic likely low.
Add indexes once data grows or `/stats` gets slow.

Useful indexes if needed:

```js
db.stats.createIndex({ cmd: 1, n: -1 })       // /stats cmd <name>
db.stats.createIndex({ uid: 1, n: -1 })       // /stats user <name>
db.stats.createIndex({ user: 1, updatedAt: -1 }) // resolve username
```

Do not add many indexes early. Each index costs memory/disk and write work.

## Comparative Analysis

| Shape | Example | Good | Bad | Verdict |
|---|---|---|---|---|
| User-rooted | `{ uid, user, commands: { ping: 3 } }` | `/stats user` easy | `/stats cmd` scans users, dynamic keys, growing doc | Reject |
| Command-rooted | `{ cmd, users: { "42": { n: 3 } } }` | `/stats cmd` easy | `/stats user` scans commands, hot command docs, dynamic keys | Reject |
| Pair-rooted | `{ cmd, uid, user, n }` | All 4 views queryable, bounded docs, atomic `$inc` | Aggregation needed for totals | Recommend |
| Event log | `{ cmd, uid, user, at }` per invocation | Full history | Too much data, needs rollups | Overkill |

## Implementation Recommendations

### Primary Model

Use one document per aggregate:

```go
type usageEntry struct {
    Cmd      string `bson:"cmd" json:"cmd"`
    UserID   int64  `bson:"uid,omitempty" json:"uid,omitempty"`
    Username string `bson:"user,omitempty" json:"user,omitempty"`
    N        int64  `bson:"n" json:"n"`
}
```

Keys:

```text
<cmd>        anonymous/no-public-username bucket
<cmd>:<uid>  named user pair bucket
```

### Write Path

Use MongoDB `UpdateOne` with `upsert: true`:

```js
db.stats.updateOne(
  { _id: "ping:42" },
  {
    $set: { cmd: "ping", uid: 42, user: "alice" },
    $inc: { n: 1 },
    $currentDate: { updatedAt: true }
  },
  { upsert: true }
)
```

This avoids read-modify-write races in the current generic `DocStore` loop.

### Read Path

Use Mongo queries/aggregation when backend is MongoDB. Keep in-memory fallback
for tests and no-database local run.

Queries:

```js
// /stats
db.stats.aggregate([
  { $group: { _id: "$cmd", n: { $sum: "$n" } } },
  { $sort: { n: -1, _id: 1 } },
  { $limit: 20 }
])

// /stats users
db.stats.aggregate([
  { $match: { uid: { $gt: 0 }, user: { $type: "string", $ne: "" } } },
  { $group: { _id: "$uid", user: { $first: "$user" }, n: { $sum: "$n" } } },
  { $sort: { n: -1, user: 1 } },
  { $limit: 20 }
])

// /stats user alice
db.stats.find({ uid: 42 }).sort({ n: -1, cmd: 1 }).limit(20)

// /stats cmd ping
db.stats.find({ cmd: "ping", uid: { $gt: 0 } }).sort({ n: -1, user: 1 }).limit(20)
```

### Username Rename

When a user calls a command with a new username, refresh all docs for that `uid`
with `UpdateMany({ uid }, { $set: { user } })`. It is not atomic as a whole, but
stats are best-effort and eventual consistency is acceptable.

### Migration

Existing prod data likely in old key prefixes. Options:

1. No migration: simplest, stats reset after deploy.
2. Read old + new for one release: more code, little value.
3. One-off migration script: only if historical stats matter.

Recommendation: no migration unless user explicitly wants historical stats.

## Common Pitfalls

- Do not use dynamic fields like `commands.ping` or `users.42` as primary model.
- Do not grow one document forever.
- Do not use generic get-then-put for hot counters on MongoDB.
- Do not over-index small collections.
- Do not use username as identity. Telegram username can change; use `uid`.

## Decision

Recommended: pair-rooted aggregate docs, Mongo atomic upsert increments, Mongo
aggregation for display views, memory fallback for tests.

## References

- MongoDB Avoid Unbounded Arrays:
  https://www.mongodb.com/docs/manual/data-modeling/design-antipatterns/unbounded-arrays/
- MongoDB `$inc`:
  https://www.mongodb.com/docs/manual/reference/operator/update/inc/
- MongoDB Atomicity:
  https://www.mongodb.com/docs/manual/core/write-operations-atomicity/
- MongoDB `$group`:
  https://www.mongodb.com/docs/manual/reference/operator/aggregation/group/
- MongoDB `$sort`:
  https://www.mongodb.com/docs/manual/reference/operator/aggregation/sort/
- MongoDB Aggregation Optimization:
  https://www.mongodb.com/docs/manual/core/aggregation-pipeline-optimization/
- MongoDB ESR Guideline:
  https://www.mongodb.com/docs/manual/tutorial/equality-sort-range-guideline/
- MongoDB Indexing Strategies:
  https://www.mongodb.com/docs/manual/applications/indexes/
- MongoDB Go Driver Update:
  https://www.mongodb.com/docs/drivers/go/current/crud/update/
- MongoDB Go Driver Indexes:
  https://www.mongodb.com/docs/drivers/go/current/indexes/

## Next Steps

1. Confirm pair-rooted aggregate docs.
2. Finish stats implementation.
3. Run `go test ./internal/modules/stats ./internal/storage`.
4. Optional later: add Mongo integration test if local Mongo available.

## Unresolved Questions

- Keep old stats by migration, or accept reset?
- Add indexes now, or wait until data size/latency proves need?
