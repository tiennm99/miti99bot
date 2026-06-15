---
title: Integrate VNAppMob SJC gold price with auto-refresh API key
description: >-
  Add a VNAppMob SJC price provider to the gold module. The provider
  self-manages a free JWT API key by refreshing it when missing or expired,
  stores it in KV, and uses it to call the SJC endpoint.
status: completed
priority: P2
branch: main
tags:
  - gold
  - vnappmob
  - sjc
  - api-key
  - kv
blockedBy: []
blocks: []
created: '2026-06-15T02:15:10.444Z'
createdBy: 'ck:plan'
source: skill
---

# Integrate VNAppMob SJC gold price with auto-refresh API key

## Overview

Replace/add the gold spot-price source with VNAppMob's Vietnam SJC price feed (`api.vnappmob.com/api/v2/gold/sjc`). The feed returns VND/lượng directly, removing the XAU/USD + FX conversion step. It requires a free `api_key` JWT that expires in ~14 days, so the implementation must refresh and persist the key automatically.

The existing `GoldPriceClient` provider chain is extended: VNAppMob SJC becomes the new primary provider; the old XAU/USD chain becomes the fallback. A new `VNAppMobClient` handles key refresh via `POST /api/request_api_key?scope=gold`, stores the key under KV (`vnappmob:api_key`), and uses it as `Authorization: Bearer <jwt>` for `GET /api/v2/gold/sjc`.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [VNAppMob API research and key refresh design](./phase-01-vnappmob-api-research-and-key-refresh-design.md) | Completed |
| 2 | [Implement SJC price client with API key management](./phase-02-implement-sjc-price-client-with-api-key-management.md) | Completed |
| 3 | [Wire client into gold module and handlers](./phase-03-wire-client-into-gold-module-and-handlers.md) | Completed |
| 4 | [Update IaC and env handling](./phase-04-update-iac-and-env-handling.md) | Completed |
| 5 | [Tests and verification](./phase-05-tests-and-verification.md) | Completed |

## Dependencies

- No blocking plans. This touches only `internal/modules/gold`, `cmd/server/main.go`, and `template.yaml`.

## Risks

- VNAppMob key refresh endpoint may change or rate-limit. Mitigation: env override + fallback to existing XAU/USD chain.
- JWT parsing for expiry must not require a JWT library if possible (base64 + JSON). Mitigation: implement minimal JWT claim extraction; treat parse failure as "refresh needed".
- Concurrent Lambda containers may race to refresh the key. Mitigation: CAS-based single-flight or last-write-wins with short TTL.

## Success Criteria

- `/gold_price` returns SJC buy/sell prices in VND/lượng when VNAppMob is healthy.
- Missing/expired key triggers a refresh transparently without user error.
- If VNAppMob fails, the bot falls back to the existing XAU/USD-derived price.
- Env var `GOLD_VNAPP_API_KEY` can bypass auto-fetch for local dev or SSM injection.
