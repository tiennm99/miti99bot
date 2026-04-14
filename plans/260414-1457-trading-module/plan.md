---
title: "Fake Trading Module"
description: "Paper trading module for Telegram bot — virtual portfolio with crypto, stocks, forex, gold"
status: pending
priority: P2
effort: 6h
branch: main
tags: [feature, module, trading]
created: 2026-04-14
---

# Fake Trading Module

## Phases

| # | Phase | Status | Effort | Files |
|---|-------|--------|--------|-------|
| 1 | [Symbol registry + formatters](phase-01-symbols-and-format.md) | Pending | 45m | `src/modules/trading/symbols.js`, `src/modules/trading/format.js` |
| 2 | [Price fetching + caching](phase-02-prices.md) | Pending | 1h | `src/modules/trading/prices.js` |
| 3 | [Portfolio data layer](phase-03-portfolio.md) | Pending | 45m | `src/modules/trading/portfolio.js` |
| 4 | [Command handlers + module entry](phase-04-commands.md) | Pending | 1.5h | `src/modules/trading/index.js` |
| 5 | [Integration wiring](phase-05-wiring.md) | Pending | 15m | `src/modules/index.js`, `wrangler.toml` |
| 6 | [Tests](phase-06-tests.md) | Pending | 1.5h | `tests/modules/trading/*.test.js` |

## Dependencies

```
Phase 1 ──┐
Phase 2 ──┼──► Phase 4 ──► Phase 5
Phase 3 ──┘                    │
Phase 1,2,3,4 ────────────► Phase 6
```

## Data flow

```
User /trade_buy 0.5 BTC
  -> index.js parses args, validates
  -> prices.js fetches BTC/VND (cache-first, 60s TTL)
  -> portfolio.js reads user KV, checks VND balance
  -> portfolio.js deducts VND, adds BTC qty, writes KV
  -> format.js renders reply
  -> ctx.reply()
```

## Rollback

Remove `trading` from `MODULES` in `wrangler.toml` + `src/modules/index.js`. KV data inert.

## Key decisions

- VND sole settlement currency for buy/sell
- Single KV object per user (acceptable race for paper trading)
- 60s price cache TTL via KV putJSON with expirationTtl
- Gold via PAX Gold on CoinGecko (troy ounces)
- Stocks integer-only quantities
