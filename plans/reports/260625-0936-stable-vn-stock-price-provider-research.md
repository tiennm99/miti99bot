---
type: research-report
topic: stable-vn-stock-price-provider
created: 2026-06-25 09:36 UTC
status: done
---

# Research Report: Stable Free VN Stock Price Provider

## Executive Summary

Best fit: **EODHD free plan as primary EOD provider**, with DynamoDB daily cache. It is the only provider found with explicit Vietnam exchange support (`VN` / MIC `XSTC`) and a real free API-key plan. It is not real-time on free tier, but this bot is paper trading, so previous/official-ish EOD close is acceptable if rendered as EOD/last close.

Do not depend on SSI/KBS broker-app endpoints as primary. Live evidence from Lambda: SSI returns Cloudflare security pages, KBS times out from AWS egress. Yahoo works today, but it is not a stable contracted API for VN stocks. Keep Yahoo/SSI/KBS only as emergency fallback.

If user wants intraday/current price with a stable contract, no truly free option found. The free stable path is EOD only. Paid floor appears around $19.99/mo for EODHD all-world EOD; intraday/live is higher.

## Methodology

- Date: 2026-06-25.
- Scope: Vietnam listed equities for `stock` module: `/stock_stats`, `/stock_buy`, `/stock_sell`.
- Criteria: explicit VN coverage, free API key, documented REST API, low-volume Lambda-safe, legal/terms risk, implementation effort.
- Sources consulted: EODHD, Marketstack, Twelve Data, FMP, Alpha Vantage, existing production probes.
- Direct probes:
  - Twelve Data `/stocks?country=Vietnam`, `/stocks?country=Viet%20Nam`, `/stocks?exchange=HOSE` returned empty arrays.
  - Twelve Data `/stocks?symbol=FPT` resolved Thailand `FPT`, not Vietnam FPT Corp.
  - EODHD demo token returned `403 Forbidden` for `exchange-symbol-list/VN`; expected, demo token not real free key.
  - Marketstack without key returned `missing_access_key`; expected.

## Key Findings

### 1. EODHD

EODHD explicitly lists **Vietnam Stocks - VN**, country Vietnam, MIC `XSTC`, timezone `Asia/Ho_Chi_Minh`, and 593 active tickers. Example listed format is `AAA.VN`, `ACB.VN`, etc. Source: https://eodhd.com/exchange/VN

Free plan:
- $0/mo.
- 20 calls/day.
- EOD data only, past-year depth.
- Personal use.
- Requires registration/API key.

Docs/pricing sources:
- https://eodhd.com/financial-apis/api-for-historical-data-and-volumes
- https://eodhd.com/pricing
- https://eodhd.com/list-of-stock-markets

Pros:
- Explicit VN coverage.
- Documented REST API.
- Free key allowed.
- Enough for bot if we cache daily and request only held/watchlist symbols.
- Symbol format simple: `FPT.VN`, `TCB.VN`, etc.

Cons:
- Free quota low: 20/day.
- EOD, not live current price.
- Terms disclaim data may not be real-time/accurate and is indicative.
- Commercial/display use may require paid/commercial agreement.

Verdict: **Primary recommendation**.

### 2. Marketstack

Marketstack has a documented free plan and stable API infra, but no confirmed Vietnam symbol coverage from public unauthenticated probe. It advertises global EOD data, stock tickers info, and exchange info.

Free plan:
- 100 requests/month.
- EOD data and up to 12 months history.
- API key required.

Sources:
- https://marketstack.com/
- https://marketstack.com/pricing
- https://docs.apilayer.com/marketstack/docs/api-endpoints-v1

Pros:
- Documented.
- Stable SaaS provider.
- Supports multi-symbol EOD requests via `symbols` parameter according to docs/search result.

Cons:
- Free quota worse than EODHD for daily bot use unless multi-symbol request covers all holdings.
- VN coverage not verified without real key.
- Intraday/real-time mostly paid/US-oriented.

Verdict: **Candidate backup only after registering a free key and proving `FPT`, `TCB`, `HPG`, `MSN`, `MWG`, `SSI`, `VND` coverage**.

### 3. Twelve Data

Twelve Data has good free quota reputation and strong docs, but public reference data did not show Vietnam equities in probes.

Source:
- https://support.twelvedata.com/en/articles/5620513-how-to-find-all-available-symbols-at-twelve-data
- https://twelvedata.com/

Probe results:
- `https://api.twelvedata.com/stocks?country=Vietnam` -> `[]`
- `https://api.twelvedata.com/stocks?country=Viet%20Nam` -> `[]`
- `https://api.twelvedata.com/stocks?exchange=HOSE` -> `[]`
- `https://api.twelvedata.com/stocks?symbol=FPT` -> Thailand `FPT`, not Vietnam.

Verdict: **Reject for now** unless support confirms VN coverage on a plan/add-on.

### 4. Financial Modeling Prep

FMP free plan is attractive on paper: 250 calls/day and EOD/reference data. But the public docs/pricing suggest global coverage only at higher tiers, and demo API key did not allow exchange/symbol coverage validation.

Sources:
- https://site.financialmodelingprep.com/developer/docs
- https://site.financialmodelingprep.com/pricing-plans

Pros:
- 250 calls/day free.
- Strong docs/API shape.

Cons:
- Vietnam coverage not verified.
- Pricing page implies Basic/free has limited symbols and EOD only; global coverage appears in Ultimate paid tier.
- Display/redistribution needs licensing agreement.

Verdict: **Reject until a real free key confirms VN symbols**.

### 5. Alpha Vantage

Alpha Vantage is reputable and free-key based, but no evidence found for Vietnam exchange coverage. It is more useful for US/global large markets than VN equities.

Source:
- https://www.alphavantage.co/

Verdict: **Reject for VN stocks**.

### 6. Broker/App Internal Endpoints

Current/existing providers:
- SSI direct quote: `iboard-query.ssi.com.vn`.
- SSI chart history: `iboard-api.ssi.com.vn`.
- KBS data-day: `kbbuddywts.kbsec.com.vn`.
- Yahoo chart: `query1.finance.yahoo.com`.

Findings:
- SSI now returns 403 Cloudflare security page from this workspace and Lambda.
- KBS returns locally but times out from Lambda.
- Yahoo works from Lambda today, but is not a contracted API.

Verdict: **fallback only, never primary**.

## Comparative Analysis

| Provider | VN coverage proven | Free key | Free quota | Current price | Stable API | Fit |
|---|---:|---:|---:|---:|---:|---|
| EODHD | yes | yes | 20/day | EOD only | yes | best |
| Marketstack | unknown | yes | 100/month | EOD free | yes | backup candidate |
| Twelve Data | no in probes | yes | likely generous | unknown | yes | reject |
| FMP | unknown | yes | 250/day | EOD free | yes | reject until proven |
| Alpha Vantage | no evidence | yes | free | unknown | yes | reject |
| SSI/KBS/Yahoo | yes-ish | no | unlimited-ish | yes | no | fallback only |

## Implementation Recommendation

### Provider Order

```text
EODHD EOD cache -> Yahoo emergency -> SSI emergency -> KBS emergency -> no price
```

Do not put Yahoo before EODHD once EODHD key exists. Yahoo is useful operationally but not stable.

### Data Model

```go
type StockQuote struct {
    Symbol      string
    PriceVND    float64
    Source      string // eodhd, yahoo, ssi, kbs
    AsOfDate    string // YYYY-MM-DD
    RetrievedAt int64
    Stale       bool
}
```

Cache keys:

```text
stock-price:FPT:2026-06-25
stock-price:TCB:2026-06-25
```

### API Shape

EODHD EOD endpoint:

```text
GET https://eodhd.com/api/eod/FPT.VN?api_token=$EODHD_API_KEY&fmt=json&period=d&from=YYYY-MM-DD&to=YYYY-MM-DD
```

Use latest returned row:

```json
{
  "date": "2026-06-25",
  "open": 71000,
  "high": 71700,
  "low": 70800,
  "close": 71000,
  "adjusted_close": 71000,
  "volume": 6592100
}
```

For portfolio valuation, use `close`. For paper buy/sell, either:
- Use same `close` and label trade as "last close", or
- Keep Yahoo as live-ish fallback only for trade commands.

### Quota Strategy

Free EODHD quota is 20/day. Therefore:

1. Cache per symbol per trading date.
2. Fetch only missing cache entries.
3. For `/stock_stats`, batch unique held symbols but call EODHD sequentially under a short timeout.
4. Do not refresh more than once/day per symbol.
5. If quota exceeded, use stale cache and label `(stale YYYY-MM-DD)`.

With current portfolio of 7 symbols, daily refresh costs 7 calls/day. Fits.

### Env Config

Add:

```text
STOCK_EODHD_API_KEY_PARAMETER_NAME=/miti99bot/prod/eodhd-api-key
STOCK_EODHD_API_URL=https://eodhd.com/api
```

Store key in SSM SecureString, same pattern as Telegram/Gemini secrets.

### User-Facing Copy

When using EODHD:

```text
FPT x2300 @ 71.000 VND (EOD 2026-06-25) = ...
```

When fallback/stale:

```text
FPT x2300 @ 71.000 VND (stale EOD 2026-06-24) = ...
```

## Common Pitfalls

- Calling EODHD on every `/stock_stats`: burns quota fast.
- Treating EOD price as live market price without label.
- Assuming Marketstack/Twelve/FMP support Vietnam because marketing says "global".
- Keeping broker-app endpoints as primary.
- No stale cache. The bot should degrade to cached prices, not zero portfolio value.

## Recommended Next Steps

1. Register EODHD free key.
2. Manually verify these URLs with real key:
   - `FPT.VN`
   - `HPG.VN`
   - `MSN.VN`
   - `MWG.VN`
   - `SSI.VN`
   - `TCB.VN`
   - `VND.VN`
3. If all seven work, implement `EODHDPriceProvider` + daily cache.
4. Change `/stock_stats` labels to show source/date.
5. Keep Yahoo/SSI/KBS emergency fallbacks after EODHD/cache.

## Resources

- EODHD Vietnam exchange: https://eodhd.com/exchange/VN
- EODHD supported exchanges: https://eodhd.com/list-of-stock-markets
- EODHD EOD API: https://eodhd.com/financial-apis/api-for-historical-data-and-volumes
- EODHD pricing: https://eodhd.com/pricing
- Marketstack pricing: https://marketstack.com/pricing
- Marketstack docs: https://docs.apilayer.com/marketstack/docs/api-endpoints-v1
- Twelve Data symbol reference: https://support.twelvedata.com/en/articles/5620513-how-to-find-all-available-symbols-at-twelve-data
- FMP docs: https://site.financialmodelingprep.com/developer/docs
- FMP pricing: https://site.financialmodelingprep.com/pricing-plans
- Alpha Vantage: https://www.alphavantage.co/

## Unresolved Questions

- Does EODHD free key return all current holdings (`FPT.VN`, `HPG.VN`, `MSN.VN`, `MWG.VN`, `SSI.VN`, `TCB.VN`, `VND.VN`) via API, not just website pages?
- Is "last close" acceptable for `/stock_buy` and `/stock_sell`, or should those commands keep live-ish Yahoo fallback?
- Is bot use personal/non-commercial under EODHD terms, or does Telegram display to group chats require paid/commercial terms?
