---
phase: 4
title: Module registration and docs
status: completed
priority: P2
effort: 1h
dependencies:
  - 3
---

# Phase 4: Module registration and docs

## Overview

Wire the gold module into the bot catalog and docs as an opt-in module for first deploy.

## Requirements

- Functional: `gold` is a first-class module selectable by `MODULES`.
- Functional: first implementation keeps `gold` opt-in; do not add it to the default `template.yaml` `ModulesCSV` until the operator explicitly promotes the spot-priced module after opt-in smoke.
- Functional: optional gold price endpoint env vars are documented if implemented.
- Non-functional: docs must state price source limitation clearly.

## Architecture

Registration follows the current composition-root pattern:

- import `internal/modules/gold` in `cmd/server/main.go`
- add `"gold": gold.New` to `factories()`
- leave `template.yaml` default `ModulesCSV` unchanged for first deploy
- optionally pass `GOLD_PRICE_API_URL` and `GOLD_FX_API_URL` through Lambda env if runtime overrides are implemented through config

## Related Code Files

- Modify: `cmd/server/main.go`
- Modify: `README.md`
- Modify: `docs/deploy-aws.md`
- Maybe modify: `template.yaml` only for override env pass-through, not default module enablement
- Maybe modify: `cmd/server/main_test.go` or equivalent to test real catalog wiring
- Maybe modify: `internal/modules/registry_test.go` only if existing tests assert full known list

## Implementation Steps

1. Add `gold` import and factory entry.
2. Keep `gold` opt-in for first deploy:
   - do not add `gold` to default `ModulesCSV`
   - document `MODULES=...,gold` enablement
3. Update README module table with `gold` and mark it opt-in if defaults remain unchanged.
4. Add docs section:
   - default commands
   - price source: world spot XAU converted to VND, not SJC local retail
   - no secrets required for default source
   - endpoint override env vars and HTTPS validation rules if implemented
   - ExchangeRate-API attribution note if displayed/required
5. Add or update tests for real composition-root wiring:
   - `factories()["gold"]` exists
   - `modules.Build([]string{"gold"}, factories(), ...)` succeeds
6. Add namespace-isolation coverage for `trading` and `gold` both using `user:<id>` under different module prefixes.

## Success Criteria

- [x] `MODULES=gold` starts without unknown-module error through the real `cmd/server` factory catalog.
- [x] `/help` lists gold commands when module is enabled.
- [x] README and deploy docs accurately describe gold's price source, opt-in status, and default units.
- [x] `template.yaml` default modules remain unchanged unless user explicitly accepts default production enablement.

## Risk Assessment

Adding `gold` to default modules would expose a spot-price product decision immediately after deploy. Mitigation: keep it opt-in for first deploy and promote to default only after a separate operator decision.
