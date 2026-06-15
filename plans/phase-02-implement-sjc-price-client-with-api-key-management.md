---
phase: 2
title: Implement SJC price client with API key management
status: completed
priority: P1
effort: 3h
dependencies:
  - 1
---

# Phase 2: Implement SJC price client with API key management

## Overview

Create `internal/modules/gold/vnappmob_client.go` with a client that refreshes, caches, and uses the VNAppMob API key, and exposes a method that returns a VND/lượng price.

## Requirements

- Self-contained client: refresh key on demand, cache in KV, parse expiry from JWT payload.
- Env overrides for base URL and token.
- 403 from SJC endpoint triggers one refresh retry.
- No new external dependencies beyond standard library.

## Architecture

```go
type VNAppMobClient struct {
    HTTP       *http.Client
    BaseURL    string          // default https://api.vnappmob.com
    Token      string          // optional env override GOLD_VNAPP_API_KEY
    KV         storage.KVStore // module KV
    nowFn      func() time.Time
}

func NewVNAppMobClientFromEnv(kv storage.KVStore) *VNAppMobClient

func (c *VNAppMobClient) FetchSJCPrice(ctx context.Context) (buy, sell float64, err error)
```

- Key is stored under `"vnappmob:api_key"`.
- Helper `getKey(ctx)` returns the current valid key, refreshing if needed.
- Helper `refreshKey(ctx)` calls `POST {BaseURL}/api/request_api_key?scope=gold`, validates JWT, stores JSON.
- Helper `jwtExp(token)` extracts middle segment, base64-url decodes, JSON-parses `exp`.
- Helper `isExpired(token)` returns true if `exp - now < 24h` or parse fails.

## Related Code Files

- Create: `internal/modules/gold/vnappmob_client.go`
- Reference: `internal/modules/gold/price_urls.go`
- Reference: `internal/modules/stock/income_events.go`

## Implementation Steps

1. Add constants:
   ```go
   vnappmobDefaultURL = "https://api.vnappmob.com"
   vnappmobKeyCacheKey = "vnappmob:api_key"
   vnappmobRefreshBuffer = 24 * time.Hour
   vnappmobHTTPTimeout = 10 * time.Second
   ```
2. Implement `NewVNAppMobClientFromEnv(kv)` reading `GOLD_VNAPP_API_URL` and `GOLD_VNAPP_API_KEY`.
3. Implement `getKey`/`refreshKey`/`jwtExp`/`isExpired`.
4. Implement `FetchSJCPrice`: build request, attach Bearer token, decode `{"results":[...]}`, validate first result, return buy/sell.
5. On 403, call `refreshKey` once and retry.

## Success Criteria

- [x] `VNAppMobClient` compiles and passes go vet.
- [x] Unit tests for JWT expiry parsing cover valid, malformed, and missing `exp`.
- [x] Mock server test verifies 403 triggers refresh and retry.
- [x] `FetchSJCPrice` returns `ErrNoGoldPrice` when response has no results or invalid values.

## Risk Assessment

- **Risk**: KV not available in local `memory` provider. **Mitigation**: KVStore is always passed; memory provider implements the interface.
- **Risk**: Race between concurrent refreshes. **Mitigation**: simple last-write-wins is acceptable; key validity is the same for all callers.
