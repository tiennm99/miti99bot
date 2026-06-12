# Research Report: Coin Module For USD Topup And Crypto Paper Trading

---
type: report
topic: coin-module
created_at: 2026-06-12 09:48 UTC
status: complete
---

## Table Of Contents

- [Executive Summary](#executive-summary)
- [Research Methodology](#research-methodology)
- [Key Findings](#key-findings)
- [Comparative Analysis](#comparative-analysis)
- [Implementation Recommendations](#implementation-recommendations)
- [Resources And References](#resources-and-references)
- [Next Steps](#next-steps)
- [Unresolved Questions](#unresolved-questions)

## Executive Summary

Add a separate `coin` module, not an extension of `trading`. Existing `trading` is VN-stock/VND oriented. Existing `gold` is the better structural reference for fractional assets, standalone KV namespace, price client, topup/buy/sell/stats commands, and optional env URL overrides.

Recommended free price strategy for MVP: provider chain with Binance, Coinbase, and CoinGecko. Try Binance first for exchange-listed USD/USDT pairs, then Coinbase public exchange rates, then CoinGecko simple price. If one provider fails, rate limits, returns no price, or does not support the coin, fall back to the next provider without mutating portfolio state.

Keep scope tight: paper trading only, USD cash balance only, market-price buy/sell only, whitelist common coins first. No deposits, withdrawals, live orders, wallets, tax, charts, limit orders, leverage, or on-chain tokens.

## Research Methodology

- Sources consulted: 4 official docs plus local repo source.
- Date range: live docs checked on 2026-06-12.
- Key search terms used: `CoinGecko simple price free API`, `Binance ticker price endpoint`, `Coinbase exchange rates unauthenticated`, `crypto price API rate limit`.
- Local references checked:
  - `README.md`
  - `internal/modules/trading/*`
  - `internal/modules/gold/*`
  - `internal/modules/registry.go`
  - `template.yaml`

## Key Findings

### 1. Repo Architecture

The bot loads modules by name from `MODULES`. Each module gets module-scoped KV through `kv.For(name)`, so a new `coin` module can own independent per-user state with key `user:{telegram_id}`.

Existing trading flow:

```text
Telegram command
  -> parse sender and args
  -> fetch/resolve price before lock
  -> acquire per-user lock or CAS update
  -> load portfolio
  -> mutate cash/assets
  -> save portfolio
  -> reply
```

Use same flow. Network calls should stay outside mutation critical path.

### 2. Current Trade References

`trading` strengths:

- command registration style is clear.
- `senderInfo` and `argsAfterCommand` are reusable patterns.
- price client uses `http.Client` timeout and test injection.
- portfolio methods keep mutation readable.

`gold` strengths:

- standalone module for one asset class.
- fractional quantity model.
- normalize invalid floats.
- CAS `UpdatePortfolio` avoids lost updates when storage supports it.
- env URL overrides for price APIs.

For `coin`, use `gold` as the closer reference, with `trading` command names and asset map style.

### 3. API Recommendation

Recommended primary architecture: three-provider failover.

```text
Fetch coin USD price
  -> Binance ticker price: SYMBOLUSDT, then SYMBOLUSD when supported
  -> Coinbase exchange rates: currency=SYMBOL, read rates.USD
  -> CoinGecko simple price: local symbol -> CoinGecko coin ID, read usd
  -> return ErrNoCoinPrice if all fail
```

Provider order:

1. Binance: best first source for highly traded pairs, simple ticker endpoint, live exchange quote.
2. Coinbase: broad public exchange-rate endpoint, no auth, direct USD quote.
3. CoinGecko: broadest metadata-backed fallback, useful when exchanges miss a symbol.

Binance API:

```http
GET https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT
```

Expected shape:

```json
{
  "symbol": "BTCUSDT",
  "price": "67321.42"
}
```

Coinbase API:

```http
GET https://api.coinbase.com/v2/exchange-rates?currency=BTC
```

Expected shape:

```json
{
  "data": {
    "currency": "BTC",
    "rates": {
      "USD": "67321.42"
    }
  }
}
```

CoinGecko API:

```http
GET https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd&include_last_updated_at=true
```

Expected shape:

```json
{
  "bitcoin": {
    "usd": 67321.42,
    "last_updated_at": 1711356300
  }
}
```

Pros:

- resilient to provider-specific outages.
- Binance gives exchange-like current pair quote for major coins.
- Coinbase gives direct USD rates without API key.
- CoinGecko gives broad ID-based coverage and optional freshness metadata.
- no secret key required for MVP if using public/demo-free paths.

Cons:

- more code than one provider.
- Binance is pair-based; some coins will not have `USD`/`USDT` pairs.
- Coinbase quote availability depends on Coinbase supported currencies.
- CoinGecko public/demo rate limits vary by traffic; avoid chatty stats calls.
- provider prices can differ. Reply should include source used.

### 4. Provider Notes

Binance:

- `/api/v3/ticker/price?symbol=BTCUSDT` returns latest exchange-pair price.
- Very simple and high-quality for listed USDT pairs.
- Good first provider for `BTCUSDT`, `ETHUSDT`, etc.
- Requires strict 429 backoff because repeat abuse can lead to temporary IP ban.

Coinbase:

- `/v2/exchange-rates?currency=BTC` returns rates for one base currency.
- No authentication required.
- Good second provider because it provides direct USD quote and simple JSON.
- Rate limits are less explicit in the opened exchange-rate page, so still cache.

CoinGecko:

- `/simple/price` supports coin IDs/symbols/names, `vs_currencies`, market cap, volume, 24h change, and `last_updated_at`.
- Docs warn public/demo usage is around 30 calls/minute and varies by traffic.
- Use as third provider because it has broad coverage and stable coin IDs.
- Maintain local symbol-to-ID mapping to avoid ambiguous symbol lookup.

CoinCap:

- Current docs redirect to pro API docs.
- Not recommended for no-key MVP unless verified in implementation.

### 5. Security Considerations

- This is paper trading. User balances are self-declared topups, not real money.
- Do not integrate wallets, private keys, exchange accounts, deposits, withdrawals, or API trade credentials.
- Whitelist supported coins to avoid phishing-style fake tickers and ambiguous symbols.
- Validate finite positive amounts and quantities. Reject NaN, Inf, zero, negative.
- Use Telegram user ID for state, same as trading/gold.
- Rate-limit failure must not mutate portfolio.
- Never log full Telegram message text if it could include user-entered values beyond command diagnostics.

### 6. Performance And Reliability

- Use 10s HTTP timeout, same as existing price clients.
- Cache prices in memory for short TTL, recommended 15-30 seconds, to reduce API calls during `/coin_stats`.
- Fetch price before portfolio update.
- For stats, use cache first. If cache misses, CoinGecko can batch via `ids=bitcoin,ethereum&vs_currencies=usd`; Binance can fetch multiple symbols through the `symbols` parameter, but only for listed pairs. Keep MVP simple: per-symbol provider chain with 15-30s cache and cap displayed holdings if needed.
- Surface `429` as "price API rate limited, try later"; do not retry in tight loop.

## Comparative Analysis

| Provider | Auth | Best For | Weakness | MVP Role |
|---|---:|---|---|---|
| Binance ticker price | No for market data | listed exchange pairs | USDT/USD-pair only, IP ban on abuse | First |
| Coinbase exchange rates | No | direct crypto-to-USD quote | unclear explicit rate quota on page | Second |
| CoinGecko simple price | Demo/pro key preferred; public limits vary | broad coin IDs, batch, freshness | rate limits, key/root URL confusion | Third |
| CoinCap | unclear/current pro docs | asset market data | free no-key path unclear | Avoid now |

## Implementation Recommendations

### Module Shape

Create:

```text
internal/modules/coin/
  coin.go
  handlers.go
  portfolio.go
  prices.go
  format.go
  symbols.go
  *_test.go
```

Register in composition root where current module factories live. Add `coin` to `template.yaml` `ModulesCSV` only if this module should be enabled by default.

### Commands

Use public commands:

```text
/coin_price <COIN>
/coin_topup <usd_amount>
/coin_buy <usd_amount> <COIN>
/coin_sell <qty> <COIN>
/coin_stats
```

Reasoning:

- Buy by USD amount is easier for users than fractional coin quantity.
- Sell by quantity is explicit and avoids accidental "sell all" behavior.
- Add `/coin_sell_usd <usd_amount> <COIN>` later only if needed.

### Supported Coins

Start with local whitelist:

```go
var supportedCoins = map[string]string{
    "BTC": "BTC",
    "ETH": "ETH",
    "SOL": "SOL",
    "BNB": "BNB",
    "XRP": "XRP",
    "ADA": "ADA",
    "DOGE": "DOGE",
    "TON": "TON",
}
```

Keep symbols uppercase. Do not accept arbitrary names at first.

### Portfolio Model

Use USD cash plus fractional holdings:

```go
type Portfolio struct {
    USD    float64            `json:"usd"`
    Assets map[string]float64 `json:"assets"`
    Meta   PortfolioMeta      `json:"meta"`
}

type PortfolioMeta struct {
    Invested  float64 `json:"invested"`
    CreatedAt int64   `json:"createdAt"`
}
```

This mirrors `gold` more than `trading`, because coins are fractional.

### Price Client

MVP client should be a small provider chain, not one hardcoded API:

```go
type PriceClient struct {
    HTTP      *http.Client
    Providers []PriceProvider
    CacheTTL  time.Duration
}

type PriceProvider interface {
    FetchUSD(ctx context.Context, symbol string) (CoinPrice, error)
}

type CoinPrice struct {
    Symbol string
    USD    float64
    Source string
}
```

Default provider URLs:

```text
Binance:  https://api.binance.com/api/v3/ticker/price
Coinbase: https://api.coinbase.com/v2/exchange-rates
CoinGecko: https://api.coingecko.com/api/v3/simple/price
```

Env overrides:

```text
COIN_BINANCE_API_URL
COIN_COINBASE_API_URL
COIN_COINGECKO_API_URL
```

Failover rules:

```text
for provider in providers:
  price, err := provider.FetchUSD(symbol)
  if err == nil && price.USD > 0:
    return price
  if err is rate-limit/network/no-price:
    continue
return ErrNoCoinPrice
```

Do not fallback after invalid user input or unsupported local symbol; fail before provider calls.

### Mutation Rules

- Topup: add USD, increment `Meta.Invested`.
- Buy: deduct USD amount, add `usd_amount / price` units.
- Sell: deduct units, add `qty * price` USD.
- Stats: show USD balance, each coin holding, market value, total value, simple P/L vs `Meta.Invested`.

### Validation

Reject:

- unsupported coin.
- amount <= 0.
- qty <= 0.
- NaN/Inf.
- price <= 0.
- insufficient USD or coin balance.

Normalize dust below `1e-9` to zero.

### Tests

Minimum test set:

- portfolio load new user.
- topup increments USD and invested.
- buy deducts USD and credits fractional coin.
- sell deducts coin and credits USD.
- insufficient USD.
- insufficient coin.
- unsupported coin.
- Binance price decode success.
- Coinbase price decode success.
- CoinGecko price decode success.
- provider chain falls back from Binance to Coinbase.
- provider chain falls back from Coinbase to CoinGecko.
- provider chain returns no price after all providers fail.
- price no USD rate.
- API 429 / non-2xx error.
- stats with cached/fake price client.

## Quick Start Guide

1. Copy structure from `internal/modules/gold`.
2. Replace VND/Luong with USD/assets map.
3. Implement provider-chain price client with injected HTTP client and URL overrides.
4. Register commands in `coin.go`.
5. Register factory in server composition root.
6. Add module to `MODULES` for local testing.
7. Run `go test ./internal/modules/coin ./internal/modules/...`.

## Common Pitfalls

- Do not store real exchange credentials. Out of scope.
- Do not use arbitrary ticker lookup. Whitelist first.
- Do not mutate portfolio before price fetch succeeds.
- Do not assume all symbols have Binance pairs, Coinbase USD rates, or CoinGecko IDs.
- Do not hide price source; include source in `/coin_price`, buy, sell, and stats replies.
- Do not fan out unlimited price calls in stats. Add cache or cap holdings.
- Do not use `int64` for coin holdings; crypto needs fractional units.

## Resources And References

- Coinbase Exchange Rates API: https://docs.cdp.coinbase.com/coinbase-app/track-apis/exchange-rates
- CoinGecko Simple Price API: https://docs.coingecko.com/reference/simple-price
- CoinGecko common errors and rate limits: https://docs.coingecko.com/docs/common-errors-rate-limit
- Binance symbol price ticker: https://developers.binance.com/docs/binance-spot-api-docs/rest-api/market-data-endpoints
- Binance limits: https://developers.binance.com/docs/binance-spot-api-docs/rest-api/limits

## Next Steps

1. Implement `internal/modules/coin` using `gold` as base pattern.
2. Use Binance -> Coinbase -> CoinGecko price provider chain with per-provider URL overrides.
3. Add short TTL cache in price client.
4. Add unit tests for portfolio, handlers, and price decode.
5. Update `README.md` module table after implementation.
6. Decide whether `coin` is enabled by default in `template.yaml`.

## Unresolved Questions

- Enable `coin` by default in deployed `ModulesCSV`, or keep opt-in like `gold`?
- Should `/coin_buy` accept USD amount only, or also support quantity mode?
- Which initial whitelist: top 8 above enough, or include more from day one?
- Should provider order be configurable by env, or fixed as Binance -> Coinbase -> CoinGecko?
