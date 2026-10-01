# Plan: `thoitiet` weather module

Status: done (2026-10-01). Design and decisions:
[research report](../reports/research-261001-0900-thoitiet-weather-module.md).

## Outcome

Public commands `/thoitiethomnay` (alias `/thoitiet`), `/thoitietngaymai`, and
`/thoitiettuannay`, each taking `[location...]` and defaulting to Ho Chi Minh
City, backed by Open-Meteo geocoding and forecast APIs.

## Steps

1. `internal/modules/thoitiet`: `thoitiet.go` (registration + handlers),
   `api_client.go` (geocode + forecast), `location.go` (diacritic stripping,
   aliases, result choice), `format.go` (WMO labels, three renderers).
2. Tests for location parsing, formatting, and handlers against `httptest`.
3. Wire `"thoitiet": thoitiet.New` in `cmd/server/main.go`; pin the new
   parameter strings in `cmd/server/command_menu_test.go`.
4. README module table row.

## Acceptance criteria

- No argument shows Ho Chi Minh City without a geocoding request.
- `Đà Lạt`, `Da Lat`, `hcm`, `saigon`, and foreign names like `Tokyo` resolve.
- `/thoitiettuannay` lists 7 days starting today in the location's timezone.
- Unknown location and upstream failure reply with Vietnamese error text.
- `go test ./...`, `go vet ./...`, and `golangci-lint run` pass.
