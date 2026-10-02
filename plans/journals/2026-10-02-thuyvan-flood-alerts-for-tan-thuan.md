---
title: thuyvan flood alerts for Tan Thuan
date: 2026-10-02
summary: Added /thuyvan tide/rain/gauge flood view and risk-only push; renamed thoitiet to weather; shared subscription helper
---

# thuyvan flood alerts for Tan Thuan

## What happened

Added `/thuyvan` to the weather module, which was renamed from `thoitiet`. It
shows Tân Thuận's flood risk from three sources: the KTTV Nam Bộ 5-day tide
bulletin PDF (Phú An and Nhà Bè peaks), the Open-Meteo rain forecast, and VNDMS
river gauges within 30 km. `/thuyvan_subscribe` adds a 10:30 ICT push, sent
only when a peak reaches báo động I (1.40 m) or rain reaches 50 mm, with a
12:30 retry. The `lol` subscriber, claim and fan-out code moved to
`internal/modules/util/subscription`.

## Lessons

- The bulletin PDF's text layer scrambles labels ("P n A ú h"), but each
  forecast row keeps its values in order after the date token. Keying on the
  date token instead of X positions handles the swapped-date rows.
- In some bulletins the station label sits on its own text row, 1 pt from the
  middle forecast row. All 11 real bulletins from April to October 2026 parse
  after allowing for that.
- VNDMS returns 403 without a same-site Referer, and its old `dmc.gov.vn`
  domains now serve a "domain moved" page.
- The review caught that a missing or stale bulletin silently turned into "no
  risk". It now errors and is retried at 12:30.

## Next steps

- Check the Coolify `MODULES` value before pushing: if it lists `thoitiet`,
  the bot fails at startup after the rename.
- Commit and push once that check is done.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
