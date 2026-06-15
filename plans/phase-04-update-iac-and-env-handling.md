---
phase: 4
title: "Update IaC and env handling"
status: completed
priority: P2
effort: "1h"
dependencies: [2]
---

# Phase 4: Update IaC and env handling

## Overview

Add optional env vars for VNAppMob configuration, export them in `cmd/server/main.go`, and expose CloudFormation parameters in `template.yaml`.

## Requirements

- Allow manual API key override (`GOLD_VNAPP_API_KEY`).
- Allow base URL override (`GOLD_VNAPP_API_URL`) for testing.
- Support SSM Parameter Store injection via `GOLD_VNAPP_API_KEY_PARAMETER_NAME`.

## Architecture

In `cmd/server/main.go`:
- Add fields to `config`: `GoldVNAppAPIURL`, `GoldVNAppAPIKey`, `GoldVNAppAPIKeyParam`.
- Read env vars `GOLD_VNAPP_API_URL`, `GOLD_VNAPP_API_KEY`, `GOLD_VNAPP_API_KEY_PARAMETER_NAME`.
- Add binding to `resolveSSMSecrets`.
- Export via `exportOptionalEnv("GOLD_VNAPP_API_URL", ...)` and `GOLD_VNAPP_API_KEY`.

In `template.yaml`:
- Add parameters `GoldVNAppAPIURL`, `GoldVNAppAPIKeyParameterName`.
- Add env vars under `BotFunction.Environment.Variables`.
- IAM policy already allows SSM fetch for `/miti99bot/${StackEnv}/*`, so no new policy needed.

## Related Code Files

- Modify: `cmd/server/main.go`
- Modify: `template.yaml`

## Implementation Steps

1. Extend `config` struct and `loadConfig`.
2. Add SSM binding and optional env export.
3. Add CFN parameters and pass to Lambda env.
4. Verify SSM path pattern matches existing IAM wildcard.

## Success Criteria

- [x] `GOLD_VNAPP_API_KEY` env var reaches the gold module.
- [x] `GOLD_VNAPP_API_KEY_PARAMETER_NAME` is fetched from SSM at startup.
- [x] `template.yaml` deploys without syntax errors (`sam validate`).

## Risk Assessment

- **Risk**: Forgetting to export env var means `os.Getenv` in module sees nothing. **Mitigation**: mirror existing `exportOptionalEnv` calls exactly.
