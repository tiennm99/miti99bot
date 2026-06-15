---
title: Fix VNAppMob gold price key parsing
description: >-
  The /api/request_api_key endpoint returns the JWT inside a JSON object
  {"results":"<jwt>"}, but the client was sending the whole JSON object as the
  Bearer token, causing SJC requests to fail with 403 "Invalid header padding".
status: completed
priority: P1
branch: main
tags:
  - gold
  - vnappmob
  - bugfix
  - api-key
blockedBy: []
blocks: []
created: '2026-06-15T04:45:00Z'
createdBy: opencode
---

# Fix VNAppMob gold price key parsing

## Root cause

`VNAppMobClient.refreshKeyLocked` reads the response body from
`GET /api/request_api_key?scope=gold` and tries to handle either a raw JWT or a
JSON-quoted string. The live endpoint actually returns:

```json
{"results":"eyJhbGciOiJIUzI1NiIs..."}
```

Because `json.Unmarshal` into a plain `string` fails for an object, the code
falls back to using the entire JSON object (including braces and quotes) as the
token. The subsequent `Authorization: Bearer {"results":...}` header is rejected
by `/api/v2/gold/sjc` with `403 - Forbidden: Auth Error: Invalid header padding`,
so all `/gold_price` requests fail with "Could not fetch gold price".

## Fix

1. Parse the refresh response as `struct{ Results string }` first.
2. Fall back to raw body / JSON-quoted string for backward compatibility.
3. Update tests to exercise the JSON-object response format.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | Parse JSON object wrapper in refresh response | Completed |
| 2 | Add unit tests for object-format token | Completed |
| 3 | Run unit tests and live smoke test | Completed |
| 4 | Code review and docs update | Completed |

## Files

- `internal/modules/gold/vnappmob_client.go`
- `internal/modules/gold/vnappmob_client_test.go`

## Success criteria

- `/gold_price` returns live SJC buy/sell prices.
- Existing unit tests still pass.
- New test covers the `{"results":"<jwt>"}` response shape.
