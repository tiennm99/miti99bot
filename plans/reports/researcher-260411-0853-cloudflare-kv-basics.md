# Researcher Report: Cloudflare Workers KV basics

**Date:** 2026-04-11
**Scope:** KV API surface, wrangler binding, limits relevant to plugin framework.

## API surface (KVNamespace binding)

```js
await env.KV.get(key, { type: "text" | "json" | "arrayBuffer" | "stream" });
await env.KV.put(key, value, { expirationTtl, expiration, metadata });
await env.KV.delete(key);
await env.KV.list({ prefix, limit, cursor });
```

### `list()` shape
```js
{
  keys: [{ name, expiration?, metadata? }, ...],
  list_complete: boolean,
  cursor: string, // present when list_complete === false
}
```
- Max `limit` per call: **1000** (also the default).
- Pagination via `cursor`. Loop until `list_complete === true`.
- Prefix filter is server-side — efficient for per-module namespacing (`wordle:` prefix).

## Limits that shape the module API
| Limit | Value | Impact on design |
|---|---|---|
| Write/sec **per key** | 1 | Counters / leaderboards must avoid hot keys. Plugin authors must know this. Document in phase-03. |
| Value size | 25 MiB | Non-issue for bot state. |
| Key size | 512 bytes | Prefixing adds ~10 bytes — no issue. |
| Consistency | Eventual (up to ~60s globally) | Read-after-write may not see update immediately from a different edge. OK for game state, NOT OK for auth sessions. |
| `list()` | Eventually consistent, max 1000/call | Paginate. |

## wrangler.toml binding

```toml
[[kv_namespaces]]
binding = "KV"
id = "<namespace-id-from-dashboard-or-wrangler-kv-create>"
preview_id = "<separate-id-for-wrangler-dev>"
```

- Access in code: `env.KV`.
- `preview_id` lets `wrangler dev` use a separate namespace — recommended.
- Create namespace: `wrangler kv namespace create miti99bot-kv` (prints IDs to paste).

## Design implications for the DB abstraction
- Interface must support `get / put / delete / list({ prefix })` — all four map 1:1 to KV.
- Namespaced factory auto-prefixes with `<module>:` — `list()` from a module only sees its own keys because prefix is applied on top of the requested prefix (e.g. module `wordle` calls `list({ prefix: "games:" })` → final KV prefix becomes `wordle:games:`).
- Return shape normalization: wrap KV's `list()` output in a simpler `{ keys: string[], cursor?: string, done: boolean }` to hide KV-specific metadata fields. Modules that need metadata can take the hit later.
- `get` default type: return string. Modules do their own JSON parse, or expose a `getJSON/putJSON` helper.

## Unresolved questions
- Do we need `metadata` and `expirationTtl` passthrough in v1? **Recommendation: yes for `expirationTtl`** (useful for easter-egg cooldowns), **no for metadata** (YAGNI).
