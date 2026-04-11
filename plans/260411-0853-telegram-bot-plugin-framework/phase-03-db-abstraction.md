# Phase 03 — DB abstraction

## Context Links
- Plan: [plan.md](plan.md)
- Reports: [Cloudflare KV basics](../reports/researcher-260411-0853-cloudflare-kv-basics.md)

## Overview
- **Priority:** P1
- **Status:** pending
- **Description:** define minimal KV-like interface (`get/put/delete/list` + `getJSON/putJSON` convenience) via JSDoc, implement `CFKVStore` against `KVNamespace`, expose `createStore(moduleName)` factory that auto-prefixes keys with `<module>:`.

## Key Insights
- Interface is deliberately small. YAGNI: no metadata, no bulk ops, no transactions.
- `expirationTtl` IS exposed on `put()` — useful for cooldowns / easter-egg throttling.
- `getJSON` / `putJSON` are thin wrappers (`JSON.parse` / `JSON.stringify`). Included because every planned module stores structured state — DRY, not speculative. `getJSON` returns `null` when the key is missing OR when the stored value fails to parse (log + swallow), so callers can treat both as "no record".
- `list()` returns a normalized shape `{ keys: string[], cursor?: string, done: boolean }` — strip KV-specific fields. Cursor enables pagination for modules that grow past 1000 keys.
- Factory-based prefixing means swapping `CFKVStore` → `D1Store` or `RedisStore` later is one-file change. NO module touches KV directly.
- When a module calls `list({ prefix: "games:" })`, the wrapper concatenates to `"wordle:games:"` before calling KV and strips `"wordle:"` from returned keys so the module sees its own namespace.
- KV hot-key limit (1 write/sec per key) — document in JSDoc so module authors are aware.

## Requirements
### Functional
- Interface exposes: `get(key)`, `put(key, value, options?)`, `delete(key)`, `list(options?)`, `getJSON(key)`, `putJSON(key, value, options?)`.
- `options` on `put` / `putJSON`: `{ expirationTtl?: number }` (seconds).
- `options` on `list`: `{ prefix?: string, limit?: number, cursor?: string }`.
- `createStore(moduleName, env)` returns a prefixed store bound to `env.KV`.
- `get` returns `string | null`.
- `put` accepts `string` only — structured data goes through `putJSON`.
- `getJSON(key)` → `any | null`. On missing key: `null`. On malformed JSON: log a warning and return `null` (do NOT throw — a single corrupt record must not crash the bot).
- `putJSON(key, value, opts?)` → serializes with `JSON.stringify` then delegates to `put`. Throws if `value` is `undefined` or contains a cycle.
- `list` returns `{ keys: string[], cursor?: string, done: boolean }`, with module prefix stripped. `cursor` is passed through from KV so callers can paginate.

### Non-functional
- JSDoc `@typedef` for `KVStore` interface so IDE completion works without TS.
- `src/db/` total < 150 LOC.

## Architecture
```
module code
  │  createStore("wordle", env)
  ▼
PrefixedStore  ── prefixes all keys with "wordle:" ──┐
  │                                                   ▼
  └──────────────────────────────────────────► CFKVStore (wraps env.KV)
                                                      │
                                                      ▼
                                                  env.KV (binding)
```

## Related Code Files
### Create
- `src/db/kv-store-interface.js` — JSDoc `@typedef` only, no runtime code
- `src/db/cf-kv-store.js` — `CFKVStore` class wrapping `KVNamespace`
- `src/db/create-store.js` — `createStore(moduleName, env)` prefixing factory

### Modify
- none

### Delete
- none

## Implementation Steps
1. `src/db/kv-store-interface.js`:
   ```js
   /**
    * @typedef {Object} KVStorePutOptions
    * @property {number} [expirationTtl] seconds
    *
    * @typedef {Object} KVStoreListOptions
    * @property {string} [prefix]
    * @property {number} [limit]
    * @property {string} [cursor]
    *
    * @typedef {Object} KVStoreListResult
    * @property {string[]} keys
    * @property {string} [cursor]
    * @property {boolean} done
    *
    * @typedef {Object} KVStore
    * @property {(key: string) => Promise<string|null>} get
    * @property {(key: string, value: string, opts?: KVStorePutOptions) => Promise<void>} put
    * @property {(key: string) => Promise<void>} delete
    * @property {(opts?: KVStoreListOptions) => Promise<KVStoreListResult>} list
    * @property {(key: string) => Promise<any|null>} getJSON
    * @property {(key: string, value: any, opts?: KVStorePutOptions) => Promise<void>} putJSON
    */
   export {};
   ```
2. `src/db/cf-kv-store.js`:
   - `export class CFKVStore` with `constructor(kvNamespace)` stashing binding.
   - `get(key)` → `this.kv.get(key, { type: "text" })`.
   - `put(key, value, opts)` → `this.kv.put(key, value, opts?.expirationTtl ? { expirationTtl: opts.expirationTtl } : undefined)`.
   - `delete(key)` → `this.kv.delete(key)`.
   - `list({ prefix, limit, cursor } = {})` → call `this.kv.list({ prefix, limit, cursor })`, map to normalized shape `{ keys: result.keys.map(k => k.name), cursor: result.cursor, done: result.list_complete }`.
   - `getJSON(key)` → `const raw = await this.get(key); if (raw == null) return null; try { return JSON.parse(raw); } catch (e) { console.warn("getJSON parse failed", { key, err: String(e) }); return null; }`.
   - `putJSON(key, value, opts)` → `if (value === undefined) throw new Error("putJSON: value is undefined"); return this.put(key, JSON.stringify(value), opts);`.
3. `src/db/create-store.js`:
   - `export function createStore(moduleName, env)`:
     - Validate `moduleName` is non-empty `[a-z0-9_-]+`.
     - `const base = new CFKVStore(env.KV)`.
     - `const prefix = \`${moduleName}:\``.
     - Return object:
       - `get(key)` → `base.get(prefix + key)`
       - `put(key, value, opts)` → `base.put(prefix + key, value, opts)`
       - `delete(key)` → `base.delete(prefix + key)`
       - `list(opts)` → call `base.list({ ...opts, prefix: prefix + (opts?.prefix ?? "") })`, then strip the `<module>:` prefix from returned keys before returning.
       - `getJSON(key)` → `base.getJSON(prefix + key)`
       - `putJSON(key, value, opts)` → `base.putJSON(prefix + key, value, opts)`
4. JSDoc every exported function. Types on params + return.
5. `npm run lint`.

## Todo List
- [ ] `src/db/kv-store-interface.js` JSDoc types (incl. `getJSON` / `putJSON`)
- [ ] `src/db/cf-kv-store.js` CFKVStore class (incl. `getJSON` / `putJSON`)
- [ ] `src/db/create-store.js` prefixing factory (all six methods)
- [ ] Validate module name in factory
- [ ] JSDoc on every exported symbol
- [ ] Lint clean

## Success Criteria
- All four interface methods round-trip via `wrangler dev` + preview KV namespace (manual sanity check OK; unit tests land in phase-08).
- Prefix stripping verified: `createStore("wordle").put("k","v")` → raw KV key is `wordle:k`; `createStore("wordle").list()` returns `["k"]`.
- Swapping `CFKVStore` for a future backend requires changes ONLY inside `src/db/`.

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| KV hot-key limit breaks a module (1 w/s per key) | Med | Med | Document in JSDoc; phase-08 adds a throttle util if needed |
| Eventual consistency confuses testers | High | Low | README note in phase-09 |
| Corrupt JSON crashes a module handler | Low | Med | `getJSON` swallows parse errors → `null`; logs warning |
| Module name collision with prefix characters | Low | Med | Validate regex `^[a-z0-9_-]+$` in factory |

## Security Considerations
- Colon in module names would let a malicious module escape its namespace — regex validation blocks this.
- Never expose raw `env.KV` outside `src/db/` — enforce by code review (module registry only passes the prefixed store, never the bare binding).

## Next Steps
- Phase 04 consumes `createStore` from the module registry's `init` hook.
