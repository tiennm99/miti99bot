---
phase: 1
title: VNAppMob API research and key refresh design
status: completed
priority: P1
effort: 1h
dependencies: []
---

# Phase 1: VNAppMob API research and key refresh design

## Overview

Confirm the exact request/response contracts for API key refresh and SJC price fetch, decide where the key is persisted, and define the expiry detection strategy.

## Requirements

- **Functional**: Document how to refresh the free API key, how to call `GET /api/v2/gold/sjc`, and the JSON response shape.
- **Non-functional**: Use only built-in Go packages for JWT claim extraction if possible; avoid new dependencies.

## Architecture

1. **Key refresh endpoint**: `POST https://api.vnappmob.com/api/request_api_key?scope=gold`
   - Returns a raw JWT string.
   - JWT payload contains `exp` (Unix seconds), `scope`, `permission`.
2. **SJC price endpoint**: `GET https://api.vnappmob.com/api/v2/gold/sjc`
   - Header: `Authorization: Bearer <jwt>`
   - Response: `{"results":[{"buy_1l":<float>,"sell_1l":<float>, ...}]}`
   - Use the first element's `buy_1l`/`sell_1l` as the spot price.
3. **Key storage**:
   - KV key: `vnappmob:api_key`
   - Value JSON: `{"token":"<jwt>","exp":<unix>}`
   - Stored under the gold module's prefixed KV (already `gold:`).
4. **Expiry detection**:
   - Parse JWT payload (middle segment), base64-url decode, JSON-decode `exp`.
   - Refresh when `exp - now < refreshBuffer` (buffer e.g. 1h or 1 day).
   - If parsing fails, force refresh.

## Related Code Files

- Read: `internal/modules/gold/prices.go`
- Read: `internal/modules/gold/price_providers.go`
- Read: `internal/modules/gold/price_urls.go`
- Read: `internal/storage/kv_store.go`

## Implementation Steps

1. Re-fetch `https://api.vnappmob.com/api/request_api_key?scope=gold` to confirm response format and inspect a fresh JWT via `jwt.io` or a small Go snippet.
2. Decode JWT payload and verify fields (`exp`, `iat`, `scope`, `permission`).
3. Document the response sample in this phase file.
4. Choose buffer: 24h (refresh one day before expiry, avoiding midnight edge cases).
5. Decide concurrency strategy: CAS lock key `vnappmob:refresh_lock` or accept last-write-wins with 1-min TTL.

## Success Criteria

- [x] API refresh endpoint verified to return a JWT string.
- [x] JWT payload fields and expiry semantics documented.
- [x] SJC endpoint response shape documented with sample values.
- [x] KV key names and refresh buffer decided.

## Risk Assessment

- **Risk**: Refresh endpoint returns non-JWT or changes shape. **Mitigation**: treat non-JWT as an error and fall back to existing providers.
- **Risk**: Clock skew causes premature expiry. **Mitigation**: 24h buffer absorbs skew; refresh on any 403 even if expiry appears valid.
