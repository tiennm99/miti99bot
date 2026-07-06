---
phase: 1
title: API Client And Env Config
status: completed
priority: P2
dependencies: []
---

# Phase 1: API Client And Env Config

## Overview

Create a small misc-local HTTP client for the wheelofnames API and load its
endpoint/token from system env. This phase adds the remote render capability
without changing command behavior yet.

## Requirements

- Functional: Read `WHEELOFNAMES_API_URL` and `WHEELOFNAMES_API_TOKEN` from env.
- Functional: POST JSON to the configured full endpoint URL.
- Functional: Include Bearer auth only when token is non-empty.
- Functional: Return GIF bytes and metadata needed by the command.
- Non-functional: Use a bounded `http.Client` timeout, around 30 seconds.
- Non-functional: Never log or expose `WHEELOFNAMES_API_TOKEN`.
- Non-functional: Allow `http://` for private Coolify/Docker networks and
  `https://` for public deployments; reject other schemes.

## Architecture

Add `wheelofnames_beta_api_client.go` in package `misc`.

Suggested types:

```go
type wheelBetaAPIClient struct {
    HTTP  *http.Client
    URL   string
    Token string
}

type wheelBetaAPIRequest struct {
    Options     []string `json:"options"`
    WinnerIndex int      `json:"winnerIndex"`
    DurationMs  int      `json:"durationMs"`
    HoldMs      int      `json:"holdMs"`
    FPS         int      `json:"fps"`
    Size        int      `json:"size"`
    Theme       string   `json:"theme"`
}
```

Use a method like:

```go
func (c *wheelBetaAPIClient) Render(ctx context.Context, options []string, winner int) ([]byte, error)
```

Return errors for:
- client not configured
- invalid URL scheme or malformed URL
- request build/transport failure
- non-2xx response
- non-`image/gif` content type
- empty or oversized body

Keep body reads bounded. Use a conservative max such as 12 MiB because the bot
default remote render is now 512px, 20fps, and 7 seconds total.

## Related Code Files

- Create: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/wheelofnames_beta_api_client.go`
- Create: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/wheelofnames_beta_api_client_test.go`
- Modify: none in command path yet

## Implementation Steps

1. Add env constants:
   - `WHEELOFNAMES_API_URL`
   - `WHEELOFNAMES_API_TOKEN`
2. Add `newWheelBetaAPIClientFromEnv()` that trims env strings.
3. Implement URL validation and request construction.
4. Set request headers:
   - `Content-Type: application/json`
   - `Accept: image/gif`
   - `Authorization: Bearer <token>` when token is non-empty
5. Encode request with fixed render options:
   - `durationMs: 6000`
   - `holdMs: 1000`
   - `fps: 20`
   - `size: 512`
   - `theme: "classic"`
6. Read response body through `io.LimitReader`.
7. Add focused client tests using `httptest.Server`.

## Success Criteria

- [ ] URL unset returns a typed/configuration error that command can treat as fallback.
- [ ] Valid request test confirms JSON body and winner index.
- [ ] Valid request test confirms default render fields:
  `durationMs=6000`, `holdMs=1000`, `fps=20`, `size=512`, `theme=classic`.
- [ ] Token test confirms Bearer auth header.
- [ ] HTTP 500 and 401 return errors without leaking response body as user text.
- [ ] Non-GIF content type returns error.
- [ ] Oversized response returns error.

## Risk Assessment

- Risk: Env URL points at service base URL instead of `/api/gif`.
  Mitigation: Plan/docs explicitly define `WHEELOFNAMES_API_URL` as full endpoint.
- Risk: Remote service token appears in logs.
  Mitigation: Never log request headers or token value; log only high-level status.
- Risk: 512px/20fps render takes longer than older 384px smoke path.
  Mitigation: 30-second HTTP timeout and local fallback; tune only after
  production timing data.
