---
title: MongoDB Document Design Standards Research
created: 2026-06-28T06:05:31Z
type: research
status: complete
tags:
  - mongodb
  - schema-design
  - storage
---

# Research Report: MongoDB Document Design Standards

## Executive Summary

MongoDB does not have one universal "standard schema" shape. Its core rule is
access-pattern-first design: store data that is read and updated together in the
same document, then add references only when embedded data is high-cardinality,
unbounded, independently owned, or written on different schedules.

For this repo, `{ _id, value, version, updatedAt }` is a valid generic KV-store
envelope. It is not the most idiomatic Mongo domain model. If MongoDB becomes
the primary domain database, prefer module-specific documents with domain fields
at the document root or in named subdocuments, not hidden under a generic
`value` wrapper.

Best practical recommendation: keep current KV wrapper only while the
backend-swappable `KVStore` abstraction is important. If you want "Mongo-native"
long term, migrate one high-value module at a time to typed repositories and
domain-shaped collections. First easy improvement: store `updatedAt` as BSON
Date (`time.Time` in Go), not int64 nanos, if it will be queried, sorted, shown
as a date, or used for TTL.

## Research Methodology

- Timestamp: 2026-06-28T06:05:31Z
- Sources consulted: 6 official MongoDB documentation pages
- Search scope: official MongoDB docs only
- Recency: current MongoDB manual, version 8.3 docs visible during research
- Key terms: MongoDB data modeling, best practices, embedded documents,
  references, indexes, atomic updates, TTL indexes, schema versioning

## Key Findings

### 1. MongoDB Documents Are Field-Value Documents

MongoDB stores records as BSON documents. A document is a set of field-value
pairs, where fields may contain embedded documents, arrays, dates, numbers, and
other BSON types. The `_id` field is reserved as the primary key, must be unique
in a collection, and is immutable.

Implication: `_id` as the storage key is good. `value` as a generic envelope is
legal, but it hides the domain shape one level down.

Source: https://www.mongodb.com/docs/manual/core/document/

### 2. Access Patterns Drive Schema

MongoDB explicitly recommends structuring data around how the app reads and
writes it. Data accessed together should be stored together.

Implication for portfolios/game state: if the app always loads the whole
portfolio or game state, one document per user or subject is correct. But the
document should ideally be shaped like the domain:

```javascript
{
  _id: "user:7",
  usd: 1000.25,
  holdings: { BTC: 1, ETH: 3 },
  meta: { createdAt: ISODate("2026-06-28T00:00:00Z") },
  version: 4,
  updatedAt: ISODate("2026-06-28T06:05:31Z")
}
```

Current KV shape:

```javascript
{
  _id: "user:7",
  value: {
    usd: 1000.25,
    holdings: { BTC: 1, ETH: 3 },
    meta: { createdAt: 1782604800000 }
  },
  version: 4,
  updatedAt: 1782626731000000000
}
```

The current shape is acceptable for a KV abstraction, but a real Mongo schema
would remove the `value` envelope.

Source: https://www.mongodb.com/docs/manual/data-modeling/

### 3. Embed vs Reference

Embed when related data is queried together, updated together, "has-a" data, or
archived together. Reference when child data has high cardinality, grows without
bounds, is written at different times, or can exist independently.

Implication:

- `coin`/`gold`/`stock` current portfolio snapshot: embed balances and holdings
  in one user document.
- Trade history or command logs: do not keep appending forever inside one user
  document. Use a separate `trades`/`events` collection or bucket pattern.
- `lolschedule` API cache: separate cache documents are fine.

Source: https://www.mongodb.com/docs/manual/data-modeling/best-practices/#link-related-data

### 4. Index Queried Fields, But Not Everything

MongoDB recommends indexing fields that are frequently queried, filtered,
sorted, or joined. Indexes speed reads and sorts, but cost disk, memory, and
write overhead.

Implication:

- Current KV access by `_id` needs no extra index.
- If you query by domain fields, flattening from `value.foo` to `foo` makes
  indexes and validation cleaner.
- Indexes on nested fields are possible, e.g. `{ "value.holdings.BTC": 1 }`,
  but this is weaker schema hygiene than indexing stable top-level fields.

Source: https://www.mongodb.com/docs/manual/data-modeling/best-practices/#index-commonly-queried-fields

### 5. Version Field Is Good, But Name It By Purpose

MongoDB single-document writes are atomic. To avoid concurrent lost updates,
include the expected current value in the update filter. This maps cleanly to an
optimistic-lock `version` field.

Implication: this repo's `version` field is sound for concurrency control. If
you also need schema migration versioning, use a separate `schemaVersion` field.
Do not overload `version` for both CAS and schema version.

Source: https://www.mongodb.com/docs/manual/core/write-operations-atomicity/
Source: https://www.mongodb.com/docs/manual/data-modeling/design-patterns/data-versioning/

### 6. Dates Should Be BSON Dates When They Behave Like Dates

MongoDB document examples use Date values for date fields. TTL indexes expire
documents based on date values; if the indexed field lacks date values, the doc
will not expire.

Implication: if `updatedAt` is only parity/debug metadata, int64 nanos is
workable. If it will be used for TTL, sorting, Compass inspection, or date
filters, use BSON Date:

```javascript
{
  updatedAt: ISODate("2026-06-28T06:05:31Z")
}
```

For TTL, prefer a dedicated field such as `expiresAt`, not `updatedAt`, unless
you truly want to delete old inactive state.

Source: https://www.mongodb.com/docs/manual/core/index-ttl/

## Comparative Analysis

| Shape | Good for | Bad for | Verdict |
|---|---|---|---|
| `{ _id, value, version, updatedAt }` | Generic KV, backend portability, low code churn | Domain queries, schema validation, clean indexes, Compass readability | Keep if KV abstraction stays |
| `{ _id, ...domainFields, version, updatedAt }` | Real Mongo domain model, queryability, validation, indexes | Requires per-module repositories/migrations | Best Mongo-native long-term |
| One mega `kv` collection with `{ module, key, value }` | Single collection ops, simple export | Hot/cold data mixed, indexes more complex, less module isolation | Not better than current |
| Separate typed collections per module | Clean ownership, indexes per module, better validation | More code, less backend portability | Best if committing to Mongo |

## Implementation Recommendations

### Short Term

Keep current shape. It already stores JSON objects as native BSON under `value`,
keeps `_id` lookups efficient, and uses `version` correctly for optimistic
locking.

Fix wording/design around `updatedAt`: either keep it as write-only int64 and
stop describing it as TTL-ready, or convert to BSON Date when touched next.

### Medium Term

For new Mongo-first modules, avoid the generic KV envelope. Create a typed
repository and module-specific collection:

```javascript
// collection: coin_portfolios
{
  _id: "user:7",
  usd: 1000.25,
  holdings: { BTC: 1, ETH: 3 },
  createdAt: ISODate("2026-06-28T00:00:00Z"),
  updatedAt: ISODate("2026-06-28T06:05:31Z"),
  version: 4,
  schemaVersion: 1
}
```

Use `version` only for CAS. Use `schemaVersion` only for shape migration.

### Long Term

Migrate high-value modules one by one:

1. Pick module with real query needs.
2. Define common reads/writes first.
3. Design collection around those access patterns.
4. Add JSON schema validation only after the shape stabilizes.
5. Add indexes only for proven query/sort paths.
6. Keep KV fallback for simple modules if it still saves code.

## Common Pitfalls

- Flattening too far. Mongo likes embedded documents. Removing `value` is good;
  exploding every nested object into root fields is not automatically good.
- Unbounded arrays. Do not keep infinite histories inside one user document.
- Reusing `version` for schema version and concurrency. Use separate fields.
- TTL on `updatedAt` for persistent state. This silently turns inactivity into
  deletion.
- Indexing everything. Indexes cost writes, disk, and memory.
- Premature rewrite. KV abstraction is still valuable for tests and simple
  modules.

## Recommendation For miti99bot

The current schema is a pragmatic KV envelope, not bad practice. It is the right
compromise if the goal is small bot state with in-memory tests and backend
swappability.

If the goal changes to "MongoDB is the product database", then flatten out of
`value` into typed module collections. Start with portfolios, because they have
money-path correctness and likely future reporting needs. Do not rewrite all
modules at once.

Priority:

1. Keep `_id`.
2. Keep `version` for CAS.
3. Consider BSON Date for `updatedAt`.
4. Remove `value` only during typed module repository migration.
5. Add `schemaVersion` only if documents must support multiple shapes during
   migrations.

## Resources & References

- MongoDB Documents: https://www.mongodb.com/docs/manual/core/document/
- MongoDB Data Modeling: https://www.mongodb.com/docs/manual/data-modeling/
- Data Modeling Best Practices: https://www.mongodb.com/docs/manual/data-modeling/best-practices/
- Embed vs Reference guidance: https://www.mongodb.com/docs/manual/data-modeling/best-practices/#link-related-data
- Atomicity and concurrent update filters: https://www.mongodb.com/docs/manual/core/write-operations-atomicity/
- TTL Indexes: https://www.mongodb.com/docs/manual/core/index-ttl/
- Document and Schema Versioning: https://www.mongodb.com/docs/manual/data-modeling/design-patterns/data-versioning/

## Unresolved Questions

None.
