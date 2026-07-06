---
phase: 3
title: Deployment Docs And Env Surfaces
status: completed
priority: P2
dependencies:
  - 1
  - 2
---

# Phase 3: Deployment Docs And Env Surfaces

## Overview

Document the new optional environment variables across local and Coolify
deployment surfaces so the bot can find the wheelofnames service without code
changes.

## Requirements

- Functional: Document `WHEELOFNAMES_API_URL` as the full `/api/gif` endpoint.
- Functional: Document `WHEELOFNAMES_API_TOKEN` as the Bearer token matching the
  wheelofnames service `API_TOKEN`.
- Non-functional: `.env.example` must use placeholders, not real secrets.
- Non-functional: Deploy docs must explain private network HTTP URL vs public
  HTTPS URL options.
- Non-functional: Compose comments should not force the feature on by default
  if the service is not running.

## Architecture

The bot remains a single long-polling service. The wheelofnames service is a
separate container or external service. The bot uses outbound HTTP only.
The current wheelofnames service `compose.yml` forwards its own env from the
shell with `${VAR:-default}` fallbacks, including `PORT`, `API_TOKEN`,
`MAX_CONCURRENT_RENDERS`, and render limits.

Coolify/private-network example:

```env
WHEELOFNAMES_API_URL=http://wheelofnames:3000/api/gif
WHEELOFNAMES_API_TOKEN=<same value as wheelofnames API_TOKEN>
```

Public URL example:

```env
WHEELOFNAMES_API_URL=https://wheelofnames.example.com/api/gif
WHEELOFNAMES_API_TOKEN=<same value as wheelofnames API_TOKEN>
```

## Related Code Files

- Modify: `/config/workspace/tiennm99/miti99bot/.env.example`
- Modify: `/config/workspace/tiennm99/miti99bot/compose.yml`
- Modify: `/config/workspace/tiennm99/miti99bot/docs/deploy-coolify-selfhosted.md`
- Optional modify: `/config/workspace/tiennm99/miti99bot/README.md` only if the
  command description needs to mention remote rendering.

## Implementation Steps

1. Add optional env entries to `.env.example` under operational settings.
2. Add commented or placeholder entries in `compose.yml` near other operational
   env vars.
3. Update deploy docs required/optional env table.
4. Add a short deployment note:
   - URL unset = local renderer fallback.
   - URL set + token = remote Remotion GIF rendering.
   - Token should match wheelofnames service `API_TOKEN`.
   - Bot default remote render request is 512px, 20fps, 7 seconds total.
5. Avoid suggesting multiple bot replicas; existing single-replica rule still
   applies.

## Success Criteria

- [ ] `.env.example` includes both variables with safe placeholder values.
- [ ] `compose.yml` includes the env names without embedding secrets.
- [ ] Deploy docs tell operators how to wire both services.
- [ ] Docs state fallback behavior when env is absent or remote fails.

## Risk Assessment

- Risk: Docs imply a public bot ingress is required.
  Mitigation: explicitly state this is outbound HTTP from the bot to
  wheelofnames; Telegram transport stays long polling.
- Risk: Operators put a service base URL without `/api/gif`.
  Mitigation: examples show full endpoint and implementation returns clear
  safe error/fallback.
