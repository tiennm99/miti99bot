---
type: research-report
topic: vietnam-stock-dividend-event-api
conducted_at: 2026-07-21T16:54:00+07:00
status: complete
---

# Research Report: Vietnam Stock Dividend Event API

## Executive Summary

Use SSI iBoard's live corporate-actions JSON endpoint for the first free
implementation, isolated behind a provider interface. It supplies symbol/date
filters, pagination, stable event IDs, cash values, share ratios, and relevant
dates. It is undocumented and has no SLA, so it is suitable for a replaceable
prototype adapter—not a permanent production contract.

VSDC remains the authoritative verification source, but no documented public
JSON API was found. SSI FastConnect's documented API omits corporate actions.
For contractual reliability, FiinGroup's commercial API Datafeed is the better
future provider.

## Methodology

- Conducted: 2026-07-21 (Asia/Saigon)
- Sources checked: VSDC, SSI iBoard, SSI FastConnect, VNDirect Finfo, TCBS,
  FiinGroup
- Validation: official documentation plus direct live HTTP requests
- Scope: Vietnamese listed-stock cash/share dividends; no automatic portfolio
  mutation, entitlement ownership, tax, or coin events

## Findings

### SSI iBoard

Live endpoint:

```http
GET https://iboard-api.ssi.com.vn/statistics/company/corporate-actions
    ?symbol=ACB
    &fromDate=01/06/2026
    &toDate=30/06/2026
    &page=1
    &pageSize=50
```

The verified response returned HTTP 200 without authentication and contained
`data` plus `paging`. Useful fields:

| Field | Use |
|---|---|
| `CorId` | Stable provider event ID |
| `symbol` | Ticker |
| `eventListCode` | Broad event type |
| `eventName`, `eventTitle`, `eventDescription` | Classification and user display |
| `exrightDate` | Ex-right date |
| `recordDate` | Record date |
| `issueDate` | Payment/trading date when supplied |
| `publicDate` | Publication time |
| `value` | Cash VND per share for `DIV` |
| `ratio` | Decimal event ratio |

Observed mappings:

- `DIV`: cash dividend; `value` is VND/share.
- `ISS`: issuance; accept only when title/description explicitly identifies a
  share dividend. Other issuances must not be treated as dividends.

Verified ACB examples included a cash event (`CorId=2612974`, `value=700`,
`ratio=0.07`) and share event (`CorId=2612975`, `ratio=0.13`) with ex-right and
record dates in June 2026.

Limitations: undocumented endpoint, no published SLA/rate limits/terms, and
server-side `eventListCode` filtering was ignored. Filtering must be local.

### Other providers

| Provider | Result | Decision |
|---|---|---|
| VSDC | Official notices contain the full legal event text; no documented JSON API found | Verification/fallback HTML source |
| SSI FastConnect | Documented, authenticated market-data APIs omit corporate actions | Not sufficient |
| VNDirect Finfo | Unfiltered `/v4/events` responded, but filtered requests failed and no current official contract was found | Do not depend on it |
| TCBS | Public guidance points users to VSDC; no documented corporate-action API found | Not suitable |
| FiinGroup | Commercial standardized securities API/Datafeed | Production option requiring contract |

## Recommended Design

Define a replaceable provider contract returning normalized events:

```text
DividendEvent
  providerID
  symbol
  kind: cash | shares | mixed | ambiguous
  exDate
  recordDate
  paymentDate
  vndPerShare
  ownedShares
  newShares
  title
  sourceURL
```

SSI adapter behavior:

1. Query ticker and date window with bounded pages.
2. Require every page to succeed.
3. Deduplicate by `CorId`.
4. Accept `DIV`; accept `ISS` only with explicit dividend wording.
5. Validate dates, positive values, and ratios before presenting an event.
6. Display events for user confirmation; never apply automatically.
7. Keep malformed/ambiguous items informational only.

Use a one-day overlap around the stored cursor to avoid boundary loss. The
project owner chose the existing behavior where applying a manual stock
dividend advances `dividendCheckedAt`; the fetch feature must preserve that
contract unless separately redesigned.

## Risks

- SSI can change or remove the endpoint without notice.
- Decimal ratios require exact conversion to the bot's `owned:new` integer
  format; do not use binary floating-point for entitlement math.
- An `ISS` event is not necessarily a dividend.
- Publication, ex-right, record, payment, and tradable-share dates are distinct.
- Multiple historical events can exist in one cursor window; the UI must show
  stable IDs and dates so users can apply the correct event manually.

## References

- [SSI iBoard live corporate-actions example](https://iboard-api.ssi.com.vn/statistics/company/corporate-actions?symbol=ACB&fromDate=01%2F06%2F2026&toDate=30%2F06%2F2026&page=1&pageSize=50)
- [SSI FastConnect API documentation](https://guide.ssi.com.vn/ssi-products/fastconnect-data/api-specs)
- [VSDC official notices](https://www.vsd.vn/vi/)
- [TCBS dividend guidance](https://help.tcbs.com.vn/tra-cuu-co-tuc/)
- [FiinGroup API Datafeed](https://fiingroup.vn/vi/giai-phap-du-lieu-chung-khoan-api-datafeed-microsite.html)

## Next Steps

1. Approve SSI iBoard as the initial replaceable provider.
2. Specify the Telegram command and event-selection interaction.
3. Implement exact ratio normalization, pagination, overlap, deduplication,
   fixtures, and failure behavior.
4. Reassess a licensed feed if this becomes production-critical.

## Unresolved Questions

- Which command should fetch events, and should it inspect one ticker or every
  stock asset?
- Should selecting an event prefill guidance only, or execute the existing
  manual dividend command after explicit confirmation?
