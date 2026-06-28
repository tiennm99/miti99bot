---
status: complete
created: 2026-06-29
topic: world-cup-schedule-module
---

# World Cup Schedule Module Plan

## Goal

Add a `wc` module based on `lolschedule` for World Cup 2026 schedule lookup and
daily Telegram digest subscriptions.

## Requirements

- Commands: `/wc [date]`, `/wc_today`, `/wc_week`, `/wc_subscribe`,
  `/wc_unsubscribe`.
- Daily push: 08:00 ICT via in-process cron, separate from `lolschedule`.
- Provider: football-data.org with `WC_FOOTBALL_DATA_TOKEN`.
- Live score: best-effort from provider `status` and `score`; commands fetch
  live provider data and use cache only when provider fails.
- Storage: module-local typed `DocStore` records, flattened Mongo documents.

## Files

- Create `internal/modules/wc/*`.
- Update `cmd/server/main.go`.
- Update `telegram-commands.json`.
- Update `README.md`, `.env.example`, and `docs/deploy-coolify-selfhosted.md`.

## Acceptance Criteria

- `go test ./internal/modules/wc` passes.
- `go test ./cmd/server ./internal/modules` passes after registration updates.
- `go test ./...` passes.
- `go vet ./...` passes.
- Missing token returns a friendly command error instead of panicking.
- Live fetch happens on each command/cron call when upstream is available.
- Cached/stale matches work only when upstream fails.
- `/wc_subscribe` uses its own subscriber list and preserves forum topic IDs.

## Out Of Scope

- Paid API-Football fallback.
- Static no-key fallback.
- High-frequency live score polling.

## Unresolved Questions

None.
