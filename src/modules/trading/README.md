# Trading Module

Paper-trading system where each Telegram user manages a virtual portfolio. Currently supports **VN stocks only** — crypto, gold, and currency exchange coming later.

## Commands

| Command | Action |
|---------|--------|
| `/trade_topup <amount>` | Add VND to account. Tracks cumulative invested in `meta.invested`. |
| `/trade_buy <qty> <TICKER>` | Buy VN stock at market price, deducting VND. Integer quantities only. |
| `/trade_sell <qty> <TICKER>` | Sell stock holdings back to VND at market price. |
| `/trade_convert` | Currency exchange (coming soon). |
| `/trade_stats` | Portfolio breakdown with all assets valued in VND, plus P&L vs invested. |

## Symbol Resolution

Symbols are **resolved dynamically** — no hardcoded registry. When a user buys a ticker:

1. Check KV cache (`sym:<TICKER>`) → if cached, use it
2. Query KBS API to verify the ticker exists and has price data
3. Cache the resolution permanently in KV

Any valid VN stock ticker listed on KBS "just works" without code changes.

## Database

KV namespace prefix: `trading:`

| Key | Type | Description |
|-----|------|-------------|
| `user:<telegramId>` | JSON | Per-user portfolio |
| `sym:<TICKER>` | JSON | Cached symbol resolution |
| `forex:latest` | JSON | Cached BIDV forex rates |

### Schema: `user:<telegramId>`

```json
{
  "currency": { "VND": 5000000 },
  "assets": { "TCB": 10, "FPT": 5, "VNM": 100 },
  "meta": { "invested": 10000000 }
}
```

- `currency` — fiat balances (VND only for now)
- `assets` — flat map of stock quantities keyed by ticker
- `meta.invested` — cumulative VND value of all top-ups (cost basis for P&L)
- Migrates old formats automatically on load (`totalvnd` → `meta.invested`, `stock`/`crypto`/`others` → `assets`)

### Schema: `sym:<TICKER>`

```json
{ "symbol": "TCB", "category": "stock", "label": "TCB" }
```

Cached permanently after first successful KBS lookup.

## Price Source

| API | Purpose | Auth |
|-----|---------|------|
| KBS `/iis-server/investment/stocks/{TICKER}/data_day` | VN stock daily close (VND, unscaled) | None |

Prices are fetched on demand per symbol (not batch-cached), since any ticker can be queried dynamically. KBS returns a multi-day OHLCV window; we take the latest bar's close.

## File Layout

```
src/modules/trading/
├── index.js          — module entry, wires handlers to commands
├── symbols.js        — dynamic symbol resolution via KBS + KV cache
├── format.js         — VND/stock number formatters
├── portfolio.js      — per-user KV read/write, flat assets map
├── prices.js         — KBS stock price fetch + BIDV forex (for future use)
├── handlers.js       — topup/buy/sell/convert handlers
└── stats-handler.js  — stats/P&L breakdown handler
```

## Future

- Crypto (CoinGecko), gold (PAX Gold), currency exchange (BIDV bid/ask rates)
- Dynamic symbol resolution will extend to CoinGecko search for crypto
