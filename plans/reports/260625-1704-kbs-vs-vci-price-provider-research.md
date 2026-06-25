---
type: research-report
topic: kbs-vs-vci-price-provider
created: 2026-06-25 17:04 UTC
status: done
---

# Research Report: KBS vs VCI Price Provider

## Executive Summary

For this bot, if we only need **current price**, the comparison is **KBS `/stock/iss` vs VCI `/price/symbols/getList`**. Both return all needed symbols in one batch request and returned identical prices in live probe.

**Best practical answer: test KBS price-board first, then VCI fallback.** KBS current-price response is simpler and is vnstock's default for Vietnam market quote. Our previous Lambda failure was against KBS `data_day` history endpoint with weaker headers, not the KBS current price-board endpoint. Therefore KBS current quote is still worth a Lambda probe before replacing it.

Do not treat either as stable. Both are unofficial web/app endpoints, not contracted API-key services. Both rejected raw curl fingerprints during probe: KBS returned `400 Request Blocked`, VCI returned `403 Error Page`. With browser-like headers, both worked. That means both can break by WAF/fingerprint changes.

Best stable architecture remains: **EODHD EOD/cache primary when key exists -> KBS current quote after Lambda proof -> VCI current quote fallback -> SSI direct fallback -> no price**.

## Research Methodology

- Timestamp: 2026-06-25 17:04 UTC.
- Scope: VN equity **current price only** for `miti99bot` stock module on AWS Lambda.
- Sources consulted:
  - `vnstock` source at commit `5bf05e3e494b1d109750c143e964945ed7be3f7d`.
  - Live endpoint probes from this workspace.
  - Prior production evidence: KBS timed out from Lambda during `/stock_stats` probe.
- Key terms: KBS, VCI, Vietcap, vnstock quote, price board, OHLCChart, Lambda egress.

## Key Findings

### KBS

vnstock default Vietnam market quote route uses KBS:

```text
Market().quote("SSI")
  -> Trading.price_board()
  -> POST https://kbbuddywts.kbsec.com.vn/iis-server/investment/stock/iss
  -> {"code":"SSI"}
```

Live probe with browser headers:

| Probe | Result |
|---|---:|
| Current quote, 7 symbols | `200`, `0.297s` |
| Raw/default curl | `400 Request Blocked` |

Pros:
- Simple payload.
- Flat fields: `CP` close/current price, `B1..B3` bid, `S1..S3` ask.
- vnstock uses it as default for unified market quote.

Cons:
- KBS `data_day` history endpoint timed out from our AWS Lambda environment; `stock/iss` current quote still needs Lambda proof.
- Current Go fallback currently uses per-symbol `data_day`, not the KBS batch price-board endpoint.
- WAF/fingerprint sensitive. Needs browser-like headers.
- No official read-only API key contract.

### VCI

vnstock supports VCI/Vietcap current quote endpoint:

```text
POST https://trading.vietcap.com.vn/api/price/symbols/getList
```

Live probe with browser headers:

| Probe | Result |
|---|---:|
| Current quote, 7 symbols | `200`, `0.357s` |
| Raw/default curl | `403 Error Page` |

Current quote returned same current prices as KBS:

| Symbol | KBS | VCI |
|---|---:|---:|
| FPT | 71000 | 71000 |
| HPG | 23400 | 23400 |
| MSN | 71500 | 71500 |
| MWG | 77200 | 77200 |
| SSI | 26500 | 26500 |
| TCB | 33400 | 33400 |
| VND | 17350 | 17350 |

Pros:
- Better fit for `/stock_stats`: one batch request for all current quotes.
- Rich structured response: `listingInfo`, `bidAsk`, `matchPrice`.
- Gives matched price, bid/ask, session, reference, floor, ceiling, volume.
- Same observed prices as KBS.

Cons:
- 403 without browser-like headers.
- More nested parsing than KBS.
- No official read-only API key contract.
- Needs Lambda proof before promoting.

## Comparative Analysis

| Criteria | KBS | VCI | Winner |
|---|---|---|---|
| Current quote batch | Yes | Yes | Tie |
| Payload simplicity | Flat fields | Nested objects | KBS |
| Parser safety | Easy | Moderate | KBS |
| Lambda evidence | History endpoint timed out; price-board not tested | Not tested yet | Unknown |
| Browser/WAF sensitivity | Yes | Yes | Tie |
| vnstock default | Yes | No | KBS |
| Current price richness | Good | Better structured | VCI |
| Official/stable API | No | No | Neither |
| Best for current price only | First Lambda probe | Fallback probe | KBS |

## Recommendation

For current price only, use **KBS price-board first**, but only after a Lambda probe passes. Use **VCI second**.

Recommended order:

```text
EODHD daily cache -> KBS stock/iss current quote -> VCI current quote -> SSI direct quote -> no price
```

If no EODHD key yet:

```text
KBS stock/iss -> VCI price/symbols/getList -> SSI direct quote
```

Implementation notes:
- Replace current KBS `data_day` fallback with KBS `stock/iss` batch endpoint for current price.
- Add VCI only as current quote fallback: `price/symbols/getList`.
- Add browser-like headers exactly; `Mozilla/5.0 (miti99bot)` is probably too bot-looking.
- Cache result briefly per command or per minute to avoid repeated WAF pressure.
- Log provider/source/date in stock output.

## Common Pitfalls

- Assuming vnstock's default means stable in Lambda. It does not.
- Calling KBS `data_day` per symbol when price-board can batch current price.
- Promoting either provider without Lambda proof.
- Calling either "official API". These are web/app backend endpoints.
- Using weak headers. Both rejected raw/default curl.

## Resources & References

- vnstock KBS constants: https://github.com/thinh-vu/vnstock/blob/5bf05e3e494b1d109750c143e964945ed7be3f7d/vnstock/explorer/kbs/const.py
- vnstock KBS price board: https://github.com/thinh-vu/vnstock/blob/5bf05e3e494b1d109750c143e964945ed7be3f7d/vnstock/explorer/kbs/trading.py
- vnstock VCI price board: https://github.com/thinh-vu/vnstock/blob/5bf05e3e494b1d109750c143e964945ed7be3f7d/vnstock/explorer/vci/trading.py
- vnstock UI routing defaults: https://github.com/thinh-vu/vnstock/blob/5bf05e3e494b1d109750c143e964945ed7be3f7d/vnstock/ui/_registry.py

## Next Steps

1. Add KBS `stock/iss` batch current-price provider behind env flag or provider order config.
2. Deploy to Lambda and fake-probe `/stock_stats`.
3. If KBS price-board passes Lambda, place it before VCI/SSI.
4. If KBS fails from Lambda, add VCI current-price fallback and probe.
5. Keep EODHD as stable primary when key is available.

## Unresolved Questions

- Does KBS `stock/iss` work from AWS Lambda `ap-southeast-1` with browser-like headers?
- Does VCI work from AWS Lambda `ap-southeast-1` with browser-like headers if KBS fails?
- Is live-ish price necessary for `/stock_buy` and `/stock_sell`, or is EOD close acceptable?
