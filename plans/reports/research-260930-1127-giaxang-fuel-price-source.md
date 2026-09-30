# /giaxang — Vietnam retail fuel price source and design

Researched: 2026-09-30 11:27 (Asia/Saigon)

## Outcome

Use Petrolimex's own CMS JSON endpoint. It returns all six retail products with
Zone 1 and Zone 2 prices in one unauthenticated GET, was verified live from this
Oracle ARM64 host with plain `curl` (no special headers), and its numbers match
today's press reports (E10 RON 95-III 27,080 / 27,620 đ, effective 2026-09-24).

## Source evaluation

| Source | Verdict | Evidence |
|---|---|---|
| Petrolimex CMS JSON (`portals.petrolimex.com.vn/~apis/portals/cms.item/search`) | **Use** | Live 200 response, 6 items, `Zone1Price`/`Zone2Price` numeric, `LastModified` per item. First-party data. |
| VNAppMob (already used by `gold`) | Not available | Docs list only Province, Gold v2, Exchange rate v2; `/api/v2/petrol*` returns 404. |
| petrolimex.com.vn HTML | Reject | Price table is rendered client-side (jQuery + crypto-js + rsa.js); nothing to parse server-side. |
| Third-party aggregators (vietfuel-api, toanqng/fuel JSON on GitHub) | Reject as primary | Extra hop on someone's hobby infrastructure that itself scrapes Petrolimex. |
| News sites (thanhnien, baomoi) | Reject | HTML scraping, unstable markup. |

## Endpoint contract

```text
GET https://portals.petrolimex.com.vn/~apis/portals/cms.item/search
    ?object-identity=search
    &x-request=<base64 JSON filter>
```

The `x-request` value is base64 of a fixed filter (SystemID
`6783dc1271ff449e95b74a9520964169`, RepositoryID
`a95451e23b474fe5886bfb7cf843f53c`, RepositoryEntityID
`3801378fe1e045b1afa10de7c5776124`, `Status=Published`, sorted by
`LastModified` descending). Build it from a JSON literal in code rather than
pasting an opaque base64 blob, so the IDs stay readable.

Fields used from each `Objects[]` entry:

| Field | Example | Use |
|---|---|---|
| `Title` | `Xăng E10 RON 95-III` | Product label |
| `Zone1Price` | `27080` | VND/liter, vùng 1 |
| `Zone2Price` | `27620` | VND/liter, vùng 2 |
| `DIsplayOrder` (sic) | `4` | Sort order |
| `LastModified` | `2026-09-24T07:47:51.496Z` | "Áp dụng từ" timestamp (max over items) |

Current product list: Xăng E10 RON 95-V, Xăng E10 RON 95-III, Xăng E5 RON 92-II,
DO 0,001S-V, DO 0,05S-II, Dầu hỏa 2-K. Render whatever titles the API returns;
do not hardcode the list (it changed with the 2026 E10 rollout).

## Brainstorm contract

- **Outcome:** `/giaxang` in the `misc` module replies with the current
  Petrolimex retail price table (all products, both zones, VND/liter) plus the
  effective timestamp in Asia/Saigon time.
- **Constraints:** Go, follow existing `misc`/`gold` patterns; HTTP timeout under
  the handler deadline via `chathelper.FetchContext`; https-only endpoint with a
  test-overridable base URL (`httptest`); command metadata per
  `docs/command-parameter-conventions.md`; update README module table.
- **Non-goals:** price history, change alerts/cron, PVOil or other retailers,
  per-province lookup, storing prices in MongoDB.
- **Acceptance criteria:** handler test against an `httptest` server asserts the
  exact rendered text; upstream failure (non-200, bad JSON, empty list) yields a
  friendly Vietnamese error reply, not silence; registration/help tests pass;
  `go test ./...`, `go vet ./...`, `golangci-lint run` clean.

## Trade-offs

| Approach | Load-bearing assumption | Fails first when |
|---|---|---|
| A. Fetch Petrolimex on every call (recommended) | Endpoint stays public and unauthenticated | Petrolimex adds auth/encryption to `~apis` or changes the IDs |
| B. Fetch + in-memory cache (e.g. 10 min) | Same as A | Same as A; also adds staleness logic for little gain at chat volume |
| C. Third-party aggregator JSON | Hobby project stays maintained | Maintainer stops updating (toanqng last push 2026-09-24, vietfuel 2026-07-16) |

Recommend A. Prices change at most weekly (Thursday adjustments), traffic is
chat-scale, and the fetch is one small GET, so a cache is not needed yet.

Better approaches: none — recommended direction is the requested one (first-party
JSON endpoint verified live).

## Proposed reply format

```text
⛽ Giá bán lẻ xăng dầu Petrolimex
Áp dụng từ 15:00 24/09/2026

Xăng E10 RON 95-V     28.080 | 28.640
Xăng E10 RON 95-III   27.080 | 27.620
Xăng E5 RON 92-II     26.390 | 26.910
DO 0,001S-V           32.090 | 32.730
DO 0,05S-II           30.490 | 31.090
Dầu hỏa 2-K           30.020 | 30.620

đ/lít — Vùng 1 | Vùng 2
```

Render inside `<pre>` (HTML) so columns align. Note: `LastModified` is when
Petrolimex edited the record (07:47Z = 14:47 ICT), which is shortly before the
official 15:00 effective time — label it "Cập nhật" rather than claiming the
exact effective time.

## Risks

- Unofficial API: Petrolimex may change IDs or add request signing (their site
  already loads rsa.js/crypto-js). Mitigation: clear error reply + log; the
  source lives behind one small client so swapping is cheap.
- Petrolimex reportedly blocks some cloud IP ranges (Google). Verified working
  from this Oracle host; the production Coolify host should be checked once
  after deploy.

## Next steps

1. Add `misc/giaxang_command.go` (client + formatter + command) and register it
   in `misc.New`.
2. Add handler tests with `httptest`; update README module row and package doc.
3. Run tests/vet/lint, commit `feat(misc): add /giaxang retail fuel price`.

## Sources

- Petrolimex endpoint (live probe, 2026-09-30)
- [chiraitori/vngasprice](https://github.com/chiraitori/vngasprice) — Go scraper that documents the endpoint
- [thanhnien.vn — Giá xăng dầu hôm nay 25.9.2026](https://thanhnien.vn/gia-xang-dau-hom-nay-2592026-xang-e10-len-muc-cao-nhat-28640-dong-lit-185260925083720959.htm) — cross-check of prices
- [TranQui004/vietfuel-api](https://github.com/TranQui004/vietfuel-api), [toanqng/fuel](https://github.com/toanqng/fuel) — aggregator alternatives
- [VNAppMob Open API docs](https://api.vnappmob.com/) — no fuel endpoint

## Unresolved questions

- Visibility: public (like `/random`) or protected? Recommended: public.
- Show both zones, or Zone 1 only for a shorter reply? Recommended: both.
