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

## Data Model

Per-user portfolio stored as a single KV object at key `user:<telegramId>`:

```js
{ currency: { VND, USD }, stock: {}, crypto: {}, others: {}, totalvnd: 0 }
```

- `totalvnd` tracks cumulative VND value of all top-ups (cost basis for P&L)
- VND is the sole settlement currency — buy/sell deducts/adds VND

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
