---
type: research-report
topic: ssi-direct-current-price-api
created: 2026-06-25 15:23 Asia/Saigon
status: done
---

# Research Report: SSI Direct Current Price API

## Executive Summary

SSI has a direct current quote API. Do not use the chart-history endpoint for current portfolio valuation if this endpoint stays available.

Use:

- Single symbol: `GET https://iboard-query.ssi.com.vn/stock/{TICKER}`
- Batch: `POST https://iboard-query.ssi.com.vn/stock/multiple`

For `/stock_stats`, batch endpoint is the better fit: one HTTP call for all holdings, direct VND prices, no date math, no "close array last element" parsing.

## Research Methodology

- Conducted: 2026-06-25.
- Gemini: enabled in local config but auth/trust failed; used WebSearch + direct endpoint inspection.
- Inspected SSI app shell: https://iboard.ssi.com.vn/
- Inspected main app bundle: `/build/static/js/h7pe3rzS.js`
- Found `API_SERVICES_NAME.STOCK_QUERY`, base `https://iboard-query.ssi.com.vn`, and stock query calls:
  - `/stock/{symbol}`
  - `/stock/multiple`
  - `/stock/stock-info`
  - `/system/time`
- Live-probed single and batch endpoints.

## Key Findings

### 1. Single-Symbol Current Quote

Example:

```sh
curl -sS 'https://iboard-query.ssi.com.vn/stock/TCB' \
  -H 'Accept: application/json' \
  -H 'Origin: https://iboard.ssi.com.vn' \
  -H 'Referer: https://iboard.ssi.com.vn/' \
  -H 'User-Agent: Mozilla/5.0 (miti99bot)'
```

Observed result:

- Status: `200`
- Content-Type: `application/json; charset=utf-8`
- Time: ~309ms local
- `data.matchedPrice`: `33400`
- `data.tradingDate`: `20260625`
- `data.tradingCurrencyISOCode`: `VND`

Important fields:

| Field | Meaning | Use |
|---|---|---|
| `matchedPrice` | current/last matched price, VND | primary current price |
| `expectedMatchedPrice` | expected/derived matched price, VND | fallback only if matched empty |
| `refPrice` | reference price | do not use for valuation |
| `best1Bid` / `best1Offer` | order book top | do not use for valuation |
| `tradingDate` | YYYYMMDD market date | display/cache freshness |
| `tradingStatus` | market/security status | log/display optional |

### 2. Batch Current Quote

Example:

```sh
curl -sS 'https://iboard-query.ssi.com.vn/stock/multiple' \
  -X POST \
  -H 'Accept: application/json' \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -H 'Origin: https://iboard.ssi.com.vn' \
  -H 'Referer: https://iboard.ssi.com.vn/' \
  -H 'User-Agent: Mozilla/5.0 (miti99bot)' \
  --data 'stocks=MWG&stocks=SSI&stocks=TCB&stocks=VND&stocks=FPT&stocks=HPG&stocks=MSN'
```

Observed result:

- Status: `200`
- Time: ~312ms local
- `data` count: `7`
- Prices direct VND, no scaling.

Observed prices:

| Symbol | matchedPrice | expectedMatchedPrice | refPrice | tradingDate | exchange |
|---|---:|---:|---:|---|---|
| FPT | 71000 | 71000 | 70800 | 20260625 | hose |
| HPG | 23400 | 23400 | 23500 | 20260625 | hose |
| MSN | 71500 | 71500 | 71500 | 20260625 | hose |
| MWG | 77200 | 77200 | 77800 | 20260625 | hose |
| SSI | 26500 | 26500 | 26700 | 20260625 | hose |
| TCB | 33400 | 33400 | 32500 | 20260625 | hose |
| VND | 17350 | 17350 | 17600 | 20260625 | hose |

### 3. Chart History Still Useful, But Not Best For Current Price

Previous chart endpoint:

```text
https://iboard-api.ssi.com.vn/statistics/charts/history?symbol=TCB&resolution=D&from=1781136000&to=1782345600
```

It works, but:

- requires `from` / `to` Unix seconds
- returns arrays
- close values are thousand VND, so multiply by `1000`
- one request per symbol

Direct batch quote avoids all of that.

## Recommendation

Use SSI direct batch as primary for `/stock_stats`:

```text
POST https://iboard-query.ssi.com.vn/stock/multiple
Content-Type: application/x-www-form-urlencoded
body: stocks=MWG&stocks=SSI&...
parse: data[].matchedPrice
```

Provider order:

1. SSI direct batch for stats.
2. SSI single direct for buy/sell or one-off fetches.
3. KBS as fallback.
4. Last-good DynamoDB cache as final fallback for stats only.

## Implementation Notes

- `matchedPrice > 0`: use it.
- If `matchedPrice <= 0` and `expectedMatchedPrice > 0`: optional fallback, label source maybe `SSI expected`.
- Never use `refPrice` as current valuation.
- Keep headers: `Origin`, `Referer`, `User-Agent`, `Accept`.
- Batch limit in SSI bundle appears to be `CONFIG_LIMIT_STOCK_MULTIPLE_QUERY=400`, far above this bot's typical portfolio size.
- Still treat this as unofficial app-internal API. Add KBS/cache fallback.

## Suggested Go Shape

```go
type SSIStockQuote struct {
    StockSymbol          string  `json:"stockSymbol"`
    MatchedPrice         float64 `json:"matchedPrice"`
    ExpectedMatchedPrice float64 `json:"expectedMatchedPrice"`
    RefPrice             float64 `json:"refPrice"`
    TradingDate          string  `json:"tradingDate"`
    TradingStatus        string  `json:"tradingStatus"`
    Exchange             string  `json:"exchange"`
}
```

## Next Steps

1. Implement `SSIDirectProvider`.
2. Add `FetchPrices(ctx, []string)` for batch stats path.
3. Keep single `FetchPrice(ctx, ticker)` wrapper for buy/sell.
4. Add tests for:
   - batch parses `matchedPrice` directly as VND
   - `refPrice` is ignored
   - batch partial result handles missing ticker
   - fallback to KBS/cache on SSI failure

## References

- SSI iBoard app: https://iboard.ssi.com.vn/
- Single quote endpoint example: https://iboard-query.ssi.com.vn/stock/TCB
- Batch quote endpoint: https://iboard-query.ssi.com.vn/stock/multiple
- Chart history endpoint example: https://iboard-api.ssi.com.vn/statistics/charts/history?symbol=TCB&resolution=D&from=1781136000&to=1782345600

## Unresolved Questions

- Should buy/sell use SSI direct quote immediately, or keep KBS until stats is stabilized?
- Should `expectedMatchedPrice` be allowed when `matchedPrice` is zero during pre-open/ATO/ATC?

