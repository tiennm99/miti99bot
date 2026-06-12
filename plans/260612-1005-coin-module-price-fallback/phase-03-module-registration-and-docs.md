---
phase: 3
title: Module registration and docs
status: completed
priority: P2
effort: 1.5-2h
dependencies:
  - 2
---

# Phase 3: Module registration and docs

## Context Links

- Composition root: `cmd/server/main.go`
- Module docs: `README.md`
- Deploy config: `template.yaml`
- Gold registration reference: `plans/260611-0735-gold-module-trading-parity/phase-04-module-registration-and-docs.md`

## Overview

Register `coin` as a first-class module, enable it in the deployed AWS module default, and document optional provider URL overrides.

## Key Insights

- `cmd/server/main.go` owns the factory map to avoid import cycles.
- `template.yaml` controls deployed `MODULES` default via `ModulesCSV`.
- User explicitly requested AWS registration; `coin` is included in deployed `ModulesCSV` default.

## Requirements

- Functional: deployed `MODULES` default includes `coin`; `MODULES=coin` also starts and registers all coin commands.
- Functional: optional env overrides are passed through startup config if needed by `NewCoinPriceClientFromEnv`.
- Functional: README module table includes `coin` and command summary.
- Non-functional: keep deployment free-tier; no SSM secret required.

## Architecture

```text
cmd/server/main.go factories()
  "coin": coin.New

loadConfig()
  CoinBinanceAPIURL
  CoinCoinbaseAPIURL
  CoinCoinGeckoAPIURL

main()
  exportOptionalEnv("COIN_BINANCE_API_URL", cfg.CoinBinanceAPIURL)
  exportOptionalEnv("COIN_COINBASE_API_URL", cfg.CoinCoinbaseAPIURL)
  exportOptionalEnv("COIN_COINGECKO_API_URL", cfg.CoinCoinGeckoAPIURL)
```

If the coin price client reads env directly, config additions are optional. Prefer explicit config additions only if matching the gold pattern is worth the extra lines.

## Related Code Files

- Modify: `cmd/server/main.go`
- Modify: `README.md`
- Modify: `template.yaml`
- Modify/create: `cmd/server/*_test.go` if factory/config tests exist or are needed
- Read: `internal/modules/registry_test.go`

## Implementation Steps

1. Import `internal/modules/coin` in `cmd/server/main.go`.
2. Add `"coin": coin.New` to `factories()`.
3. Decide default deployment behavior:
   - add `coin` to `ModulesCSV` default so AWS deploy enables it
4. Add non-secret template parameters only if URL overrides must be deploy-configurable:
   - `CoinBinanceAPIURL`
   - `CoinCoinbaseAPIURL`
   - `CoinCoinGeckoAPIURL`
5. Wire env variables into Lambda environment if parameters are added.
6. Update README module table with `coin` and provider fallback summary.
7. Add command usage examples in README only if existing docs style supports it; otherwise keep docs minimal.

## Todo List

- [x] Factory import and map entry added.
- [x] README module table updated.
- [x] `template.yaml` updated with `coin` in deployed `ModulesCSV` default.
- [x] Optional env overrides documented.

## Success Criteria

- [x] `modules.Build([]string{"coin"}, factories(), ...)` succeeds in test or local verification.
- [x] `/help` can list coin commands when module enabled.
- [x] No new secrets or paid services required.
- [x] Docs do not overpromise real trading.

## Risk Assessment

Adding `coin` to default deployed modules exposes new public commands immediately. User explicitly chose AWS registration; commands are public paper-trading only and require no secrets.

## Security Considerations

Do not add API keys or Parameter Store secrets. Provider URL overrides are non-secret config only.

## Next Steps

Phase 4 verifies unit behavior and full module integration.
