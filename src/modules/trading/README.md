# Trading Module

Paper-trading system where each Telegram user manages a virtual portfolio.

## Commands

| Command | Action |
|---------|--------|
| `/trade_topup <amount> [currency]` | Add fiat (VND default). Tracks cumulative invested via `totalvnd`. |
| `/trade_buy <amount> <symbol>` | Buy at market price, deducting VND. Stocks must be integer quantities. |
| `/trade_sell <amount> <symbol>` | Sell holdings back to VND at market price. |
| `/trade_convert <amount> <from> <to>` | Convert between fiat currencies (VND, USD). |
| `/trade_stats` | Portfolio breakdown with all assets valued in VND, plus P&L vs invested. |

## Supported Symbols

| Symbol | Category | Source | Label |
|--------|----------|--------|-------|
| BTC | crypto | CoinGecko | Bitcoin |
| ETH | crypto | CoinGecko | Ethereum |
| SOL | crypto | CoinGecko | Solana |
| TCB | stock | TCBS | Techcombank |
| VPB | stock | TCBS | VPBank |
| FPT | stock | TCBS | FPT Corp |
| VNM | stock | TCBS | Vinamilk |
| HPG | stock | TCBS | Hoa Phat |
| GOLD | others | CoinGecko (PAX Gold) | Gold (troy oz) |

Currencies: VND, USD.

## Database

KV namespace prefix: `trading:`

| Key | Type | Description |
|-----|------|-------------|
| `user:<telegramId>` | JSON | Per-user portfolio (balances + holdings) |
| `prices:latest` | JSON | Cached merged prices from all APIs |

### Schema: `user:<telegramId>`

```json
{
  "currency": { "VND": 5000000, "USD": 100 },
  "stock": { "TCB": 10, "FPT": 5 },
  "crypto": { "BTC": 0.005, "ETH": 1.2 },
  "others": { "GOLD": 0.1 },
  "totalvnd": 10000000
}
```

- `currency` — fiat balances (VND, USD)
- `stock` / `crypto` / `others` — asset quantities keyed by symbol
- `totalvnd` — cumulative VND value of all top-ups (cost basis for P&L)
- VND is the sole settlement currency — buy/sell deducts/adds VND
- Empty categories are `{}`, not absent — migration-safe loading fills missing keys

### Schema: `prices:latest`

```json
{
  "ts": 1713100000000,
  "crypto": { "BTC": 1500000000, "ETH": 50000000, "SOL": 3000000 },
  "stock": { "TCB": 25000, "VPB": 18000, "FPT": 120000, "VNM": 70000, "HPG": 28000 },
  "forex": { "USD": 25400 },
  "others": { "GOLD": 72000000 }
}
```

- `ts` — Unix epoch milliseconds of last fetch
- All prices in VND per unit
- Cache TTL: 60 seconds (stale fallback up to 5 minutes)

## Price Sources

Three free APIs fetched in parallel, cached in KV for 60 seconds:

| API | Purpose | Auth | Rate Limit |
|-----|---------|------|-----------|
| CoinGecko `/api/v3/simple/price` | Crypto + gold prices in VND | None | 30 calls/min (free) |
| TCBS `/stock-insight/v1/stock/bars-long-term` | Vietnam stock close prices (× 1000) | None | Unofficial |
| open.er-api.com `/v6/latest/USD` | USD/VND forex rate | None | 1,500/month (free) |

On partial API failure, available data is returned. On total failure, stale cache up to 5 minutes old is used before surfacing an error.

## File Layout

```
src/modules/trading/
├── index.js          — module entry, wires handlers to commands
├── symbols.js        — hardcoded symbol registry (9 assets, 2 currencies)
├── format.js         — VND/USD/crypto/stock/P&L formatters
├── portfolio.js      — per-user KV read/write, balance checks
├── prices.js         — API fetching + 60s cache
├── handlers.js       — topup/buy/sell/convert handlers
└── stats-handler.js  — stats/P&L breakdown handler
```

## Adding a Symbol

Add one line to `symbols.js`:

```js
NEWSYM: { category: "crypto", apiId: "coingecko-id", label: "New Coin" },
```

For stocks, `apiId` is the TCBS ticker. For crypto/gold, `apiId` is the CoinGecko ID.
