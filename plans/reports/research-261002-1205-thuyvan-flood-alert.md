# Research + Brainstorm: `/thuyvan` flood alerts for Tân Thuận

Conducted 2026-10-02. Scope: flood (ngập lụt) alerts for Tân Thuận, Quận 7,
TP.HCM, through a `/thuyvan` command and an opt-in daily push. Both go into the
current weather module, which is renamed from `thoitiet` to `weather`.

## Recommendation

Flooding in Tân Thuận is driven mostly by high tides (triều cường) on the Sài
Gòn and Đồng Điền rivers, and sometimes by heavy rain. The nearest gauges are
Phú An (2.9 km) and Nhà Bè (6.8 km). Both have official alarm levels (báo động,
BĐ) of BĐ I 1.40 m, BĐ II 1.50 m and BĐ III 1.60 m. The design uses three
sources:

1. **Tide forecast:** the official 5-day tide bulletin from Đài KTTV Nam Bộ, a
   daily PDF with the peak forecast at Phú An and Nhà Bè. This is the alert
   trigger.
2. **Rain forecast:** the Open-Meteo daily `precipitation_sum` at Tân Thuận,
   the client the module already has. This is the second trigger.
3. **Live levels:** VNDMS readings for the stations near Tân Thuận, with their
   alarm tier. This is context, and the fallback when the bulletin is missing.

An alert fires when a forecast tide peak at Phú An or Nhà Bè is at least
**1.40 m** (BĐ I), or when the day's rain forecast is at least **50 mm**.

## Decisions (user, 2026-10-02)

- Goal: flood (ngập lụt) alerts at Tân Thuận, not a general water-level tool.
- Delivery: a `/thuyvan` command plus an opt-in daily push
  (`/thuyvan_subscribe`, `/thuyvan_unsubscribe`).
- Trigger: a tide peak at or above BĐ I, OR rain of 50 mm or more.
- `/thuyvan` takes **no parameter**. It always shows Tân Thuận: the flood
  forecast plus the live levels at nearby stations.
- Placement: inside the weather module, renamed `thoitiet` → `weather`.
- Rejected: Open-Meteo tide modelling and a `[location...]` argument.

## Sources (all verified live on 2026-10-02)

### Tide bulletin: KTTV Nam Bộ

- **Discovery.** The `https://kttvnb.vn/` homepage links today's article, for
  example
  `/index.php/100-thong-tin-kttv/thuy-van/29194-ba-n-tin-da-ba-o-tha-y-v-n-tphcm-ra-nga-y-02-10-2026`.
  The slug varies (one used `khu-va-c-tphcm`), so match
  `(\d+)-[^"]*tphcm-ra-nga-y-(\d{2})-(\d{2})-(\d{4})` and take the newest date.
  The `thuy-van` category page is not sorted by date, so do not use it.
- **PDF.** The article page links
  `https://kttvnb.vn/attachments/article/<id>/HCMC_TVHN_YYYYMMDD.pdf` (318 KB,
  issued at about 09:24 ICT). The PDF path needs the article ID, so it cannot
  be guessed from the date. Plain `http://kttv-nb.org.vn` also serves the files,
  but its TLS certificate is broken, so use `https://kttvnb.vn`.
- **Text extraction** with `github.com/ledongthuc/pdf` (`GetTextByRow`, pure
  Go) works. The labels come out with odd letter spacing (`P h ú  An`), but the
  forecast number rows are clean and come in a fixed order: 5 rows for Phú An,
  5 for Nhà Bè, then 5 for Thủ Dầu Một.

  ```text
  02/ 10 1.33 07.00 1.29 21.00 - 1.79 15.00 - 0.04 01.00
  ```

  Each row is the date, peak 1 (height and time), peak 2, trough 1 and
  trough 2. `ct` means no second peak that day. Minus signs may be split from
  the number (`- 1.79`) or appear after it (`1.78 -`). Only the peaks are
  needed, and they are always positive, so the parser can drop the trough
  columns.
- The same PDF prints the BĐ I/II/III thresholds (1.40/1.50/1.60 m), and VNDMS
  `detailRain` confirms them for both stations.
- Today's forecast is a peak of 1.33 m at Phú An on 02/10, falling to 0.68 m by
  06/10, so no alert. The 28–30/09 peaks reached 1.56–1.60 m, BĐ II–III.

### Live levels: VNDMS

- `GET https://vndms.gov.vn/water_level?lv=0` needs a
  `Referer: https://vndms.gov.vn/` header, or it returns 403. It gives all 458
  stations as GeoJSON, with the fields inside `popupInfo` HTML (name, code,
  province, river, and `Mực nước (1.33(m) 7-02/10)` or `Không có số liệu`).
  `lv=1..3` returns only the stations at that alarm tier.
- The response is 32 KB gzipped (432 KB decoded) and takes 0.2–1.2 s.
- Reporting stations within 30 km of Tân Thuận today: Phú An 2.9 km, Nhà Bè
  6.8 km, Biên Hòa 25.0 km, Thủ Dầu Một 26.1 km and Bến Lức 29.1 km.

### Rain: Open-Meteo

The module's existing `fetchForecast` already returns daily `precipitation_sum`
and `precipitation_probability_max`. Call it with a fixed Tân Thuận place.

### Tân Thuận point

Open-Meteo geocoding gives `Tân Thuận`, Quận Bảy, at
`10.74111, 106.71806`. Store it as a constant, as `hcmPlace` is stored, because
geocoding "Tan Thuan" by name returns 14 Vietnamese places.

## Design

```text
/thuyvan ─┬─ tide bulletin (homepage → article → PDF → peak rows)
          ├─ Open-Meteo forecast @ Tân Thuận (rain, days 0..4)
          └─ VNDMS lv=0..3 (stations ≤ 30 km, sorted by distance)
          ─► merge into a per-day risk view ─► Reply

cron 10:30 ICT ─► same fetch ─► any day at risk? ─► push to subscribers
                                 no  ─► send nothing
```

- **Fetching.** The three sources run concurrently under one fetch context.
  Each one fails on its own: the reply omits a failed source with a short note
  and never shows guessed numbers. If the bulletin is missing (for example
  before 09:24) or fails to parse, the reply uses yesterday's bulletin when the
  homepage still links it, labelled with its issue date, or else shows only
  the live levels and the rain.
- **Parsing safety.** A bulletin counts as valid only if it yields exactly 5
  dated rows for each of Phú An and Nhà Bè, with peaks between 0 and 3 m.
  Anything else is a parse failure, logged with the PDF URL. Tests use the
  captured `HCMC_TVHN_20261002.pdf` as a fixture.
- **Risk per day.** Take the highest of the two peaks at Phú An and Nhà Bè and
  map it to a tier: below BĐ I, BĐ I, BĐ II or BĐ III. Rain of 50 mm or more
  flags the day too. Rain forecasts beyond about 5 days are not reliable, so
  only the bulletin's 5 days count.
- **Push.** A cron at 10:30 ICT, after the bulletin is issued, sends one
  message per subscribed (chat, topic) only when at least one of the next 5
  days is at risk. It needs no dedupe state, because it fires once a day.
  Subscribers are stored like `lol`'s subscriber list, including the pruning
  of chats that blocked the bot. `lol/subscribers.go` and its pruning code
  would then have a second user, so move them to a shared
  `internal/modules/util` package instead of copying them.

Sample `/thuyvan` reply:

```text
🌊 Thuỷ văn — Tân Thuận (Q.7)
Dự báo đỉnh triều (Phú An / Nhà Bè):
02/10: 1.33 / 1.34 m (07:00) — dưới BĐ I
03/10: 1.18 / 1.19 m (08:00)
04/10: 1.01 / 1.03 m (09:00)
05/10: 0.84 / 0.88 m
06/10: 1.13 / 0.70 m
Mưa hôm nay: 5.3 mm (70%)
Mực nước hiện tại:
Phú An (Sài Gòn, 2.9 km): 1.33 m, 7h 02/10
Nhà Bè (Đồng Điền, 6.8 km): 1.18 m, 7h 02/10
⚠️ Thủ Dầu Một (Sài Gòn, 26.1 km): 1.56 m, 7h 02/10 — BĐ II
BĐ I/II/III: 1.40 / 1.50 / 1.60 m
Nguồn: Đài KTTV Nam Bộ, VNDMS, Open-Meteo
```

Sample push, sent only on risk days:

```text
⚠️ Cảnh báo ngập — Tân Thuận (Q.7)
29/09: đỉnh triều 1.60 m lúc 17:00 tại Nhà Bè — BĐ III
30/09: đỉnh triều 1.56 m — BĐ II
Mưa 29/09: 62 mm
Nguồn: Đài KTTV Nam Bộ, Open-Meteo
```

## Changes

- Rename `internal/modules/thoitiet` to `internal/modules/weather`: the
  package, its doc, the log `module` field, and the `cmd/server/main.go` import
  and catalog key `"weather"`.
- The module gains storage for its subscriber list, in the `weather`
  collection, with no migration. Add the `CollectionName` constant the way the
  other modules with storage do.
- New files: `flood.go` (command, cron and risk), `tide_bulletin.go`
  (discovery and PDF parsing), `water_level.go` (VNDMS), plus tests and
  fixtures.
- Add the dependency `github.com/ledongthuc/pdf`. It has no tagged releases,
  so `go get @latest` records a pseudo-version.
- Add three public commands: `thuyvan`, `thuyvan_subscribe` and
  `thuyvan_unsubscribe`. Update `cmd/server/command_menu_test.go` and the
  README module row.
- **Breaking:** `modules.Build` rejects unknown module names, so if Coolify's
  `MODULES` lists `thoitiet`, the bot **fails to start** after the rename.
  Check that value before pushing. Commit the rename on its own as
  `refactor(weather)!:`.

## Risks

| Risk | Mitigation |
|---|---|
| The bulletin layout changes and breaks the PDF row parsing | Strict row-count and range check, fallback to live levels, a logged URL, fixture tests |
| The homepage drops the article link or changes the slug | Loose regex; a missing link means "no bulletin" and the fallback |
| The VNDMS Referer requirement tightens, or the domain moves again (it moved in 2025–26) | URL var, a clean failure note in the reply |
| 50 mm/day misses short cloudbursts (40 mm in an hour floods streets too) | Use the daily total for v1; the hourly maximum is a follow-up if alerts prove too quiet |
| The bulletin's 5 days end before a long high-tide spell | Each day's push carries the next 5 days |

## Changes after review (2026-10-02)

- A bulletin issued more than a day before today counts as missing, because
  a stalled homepage would otherwise produce a valid-looking bulletin whose
  dates no longer cover the window.
- If the bulletin is missing and the rain forecast shows no risk, the push
  returns an error instead of reporting "no risk", since tide is the main
  signal.
- A 12:30 ICT retry cron reruns the push. The day is claimed only when an
  alert goes out, so the retry sends nothing after a successful 10:30 push.
- `/thuyvan` notes a failed rain forecast as well as a failed bulletin or
  gauge list.
- Known limit: `/thuyvan` is not cached, so each call makes 8 upstream
  requests. That is acceptable at this bot's usage; add a short cache if VNDMS
  starts rate-limiting.

## Next steps

1. Check the Coolify `MODULES` value for the rename.
2. Write the plan under `plans/261002-1205-thuyvan-flood-alert/` with phases:
   the rename, moving the subscriber code to a shared package, the parsers and
   fixtures, then the command, cron and push.
3. Run `go test ./internal/modules/... ./cmd/server/...`, `go vet ./...` and
   `golangci-lint run`.

## Unresolved questions

- Push time 10:30 ICT: is that fine, or do you want a second, evening push
  before the evening high tides?
- Should the push go out only when there is risk (as designed), or every day
  as a summary?
