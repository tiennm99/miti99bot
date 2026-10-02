# Plan: `/thuyvan` flood alerts for Tân Thuận

Status: implemented, not yet committed (2026-10-02)
Design: [research report](../reports/research-261002-1205-thuyvan-flood-alert.md)

## Outcome

`/thuyvan` (no parameter) shows Tân Thuận's flood risk: a 5-day tide-peak
forecast at Phú An and Nhà Bè from the KTTV Nam Bộ bulletin, the rain forecast
from Open-Meteo, and live VNDMS levels at stations within 30 km. Chats that opt
in with `/thuyvan_subscribe` get a 10:30 ICT push, but only when some day has a
tide peak of at least 1.40 m (BĐ I) or at least 50 mm of rain.

## Constraints and non-goals

- The `thoitiet` module is renamed to `weather`. The command names stay the
  same.
- Subscriber storage and push fan-out move out of `lol` into a shared helper,
  and `lol` behaviour stays the same.
- Non-goals: other locations, tide modelling, or an evening push.

## Phases

1. [x] Rename `internal/modules/thoitiet` to `internal/modules/weather`
   (package, catalog key, README).
2. [x] Extract `internal/modules/util/subscription`: the subscriber list, the
   terminal-error classifier, the per-day claim and fan-out with pruning.
   Move `lol` onto it.
3. [x] Tide bulletin: homepage → article → PDF → peak rows, with a strict
   validator and a real PDF fixture.
4. [x] VNDMS: parse the popups, tag alarm tiers, keep stations within 30 km of
   Tân Thuận, using a trimmed fixture.
5. [x] `/thuyvan`, the subscribe and unsubscribe commands, and the 10:30 ICT
   cron. Wire up the menu test and the README.
6. [x] Run `go test ./...`, `go vet ./...` and `golangci-lint run`, then
   review.

## Acceptance

- `/thuyvan` renders today's bulletin (fixture) with the BĐ tiers. When a
  source fails, its section is replaced by a note.
- The push is sent only on risk days, once per ICT day, and prunes dead chats.
- The `lol` tests pass unchanged in behaviour.

## Risk

- The bulletin PDF layout could drift. The strict row validator turns that
  into a missing-bulletin note.
- `MODULES=thoitiet` fails startup after the rename. Check Coolify before
  pushing.
