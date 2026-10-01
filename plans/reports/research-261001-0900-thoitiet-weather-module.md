# Research + Brainstorm: `thoitiet` weather module

Conducted 2026-10-01. Scope: weather data source and command design for a new
`thoitiet` module with today, tomorrow, and this-week forecasts, an optional
location argument, and Ho Chi Minh City as the default.

## Recommendation

Build a new `internal/modules/thoitiet` module on **Open-Meteo** (forecast +
geocoding). It needs no API key, no new env var, and no storage. All payload
fields below were verified with live requests on 2026-10-01.

## Commands

| Command | Parameters | Behaviour |
|---|---|---|
| `/thoitiethomnay` | `[location...]` | Current conditions + today's forecast |
| `/thoitiet` | `[location...]` | Alias of `/thoitiethomnay` (same handler) |
| `/thoitietngaymai` | `[location...]` | Tomorrow's daily forecast |
| `/thoitiettuannay` | `[location...]` | One line per day for the next 7 days, starting today |

`[location...]` follows `docs/command-parameter-conventions.md` (optional
remaining text, so `/thoitiet Đà Lạt` works). An empty argument means Ho Chi
Minh City. The alias is registered as a second `modules.Command` sharing the
handler, the same pattern as `/trongtruonghop` + `/tth` in `misc`. Stats will
count the two names separately, which matches that existing alias.

## Data source comparison

| Source | Key | Free limit | VN geocoding | Verdict |
|---|---|---|---|---|
| Open-Meteo | None | 10,000/day, 5,000/h, 600/min, non-commercial | Yes, `language=vi` returns Vietnamese names | **Pick** |
| OpenWeatherMap | Required | 1,000/day (One Call 3.0) | Yes | Needs secret + card on file |
| WeatherAPI.com | Required | 1M/month | Yes, `lang=vi` | Needs secret; no benefit here |

A personal Telegram bot is non-commercial and nowhere near the limits, so
Open-Meteo is fine without caching. Its data is CC BY 4.0, so replies end with a
short `Nguồn: Open-Meteo` credit.

## Verified API behaviour

Geocoding: `GET https://geocoding-api.open-meteo.com/v1/search?name=<q>&count=10&language=vi`

| Query | Result |
|---|---|
| `Ho Chi Minh`, `Hồ Chí Minh` | Thành phố Hồ Chí Minh (10.823, 106.630) |
| `Ha Noi`, `Hanoi` | Hà Nội |
| `Da Lat` | Ðà Lạt |
| `Đà Lạt` (typed with diacritics) | **Wrong: "Đã Tịch", Quảng Trị** |
| `Da Nang`, `Can Tho`, `Nha Trang`, `Hue`, `Vung Tau`, `Tokyo` | Correct |
| `hcm`, `Thu Duc` | No results |
| `Saigon` | A ward in Bình Thạnh, not the city |

Two consequences for the design. First, strip Vietnamese diacritics (including
`Đ/đ` → `D/d`) before querying, because GeoNames stores Đà Lạt with the
look-alike `Ð` (U+00D0) and diacritic input misses it. Second, keep a small
alias map for common shorthand: `hcm`, `tphcm`, `sg`, `saigon`, `sai gon` → HCM;
`hn` → Hà Nội; `dn` → Đà Nẵng. Pick the first `country_code=VN` result, else the
first result, so foreign cities still work in a single request.

Forecast: `GET https://api.open-meteo.com/v1/forecast?latitude=..&longitude=..&timezone=auto&forecast_days=7`
with

- `current=temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m`
- `daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,precipitation_probability_max,uv_index_max,sunrise,sunset`

`timezone=auto` makes "today" the location's own date. HCM returns
`utc_offset_seconds=25200`. Weather is a WMO code that maps to a Vietnamese
label plus emoji (about 28 codes, for example 0 Trời quang ☀️, 3 Nhiều mây ☁️,
61–65 Mưa 🌧️, 80–82 Mưa rào 🌦️, 95–99 Dông ⛈️).

## Design

```text
handler(range) ── parse args ──► resolveLocation(q)
                                   ├─ empty        → HCM constant (no geocoding call)
                                   ├─ alias map    → constant
                                   └─ geocode(stripDiacritics(q))
                 ──► fetchForecast(lat, lon) ──► format(range) ──► send HTML
```

Files, mirroring `lol` and `giaxang`:

- `thoitiet.go`: `New(deps)` and the four command registrations.
- `api_client.go`: geocode + forecast with an `*http.Client` timeout,
  `io.LimitReader`, and package-level URL vars so tests can use `httptest`.
- `location.go`: diacritic stripping, alias map, result selection.
- `format.go`: WMO table and the today/tomorrow/week renderers.
- `handlers.go` and tests for each file.
- Wiring: `cmd/server/main.go` catalog entry `"thoitiet": thoitiet.New`, the
  expected `cmd/server/command_menu_test.go` entries, and the README module table.

Sample `/thoitiet` reply:

```text
🌤 Thời tiết hôm nay — Thành phố Hồ Chí Minh
Hiện tại: 29°C (cảm giác 35.7°C), Nhiều mây ☁️, độ ẩm 78%, gió 3 km/h
Cả ngày: 24–33°C, Mưa rào 🌦️, khả năng mưa 70% (5.3 mm), UV 8.9
Mặt trời: 05:42 – 17:44
Nguồn: Open-Meteo
```

Errors: unknown location → `Không tìm thấy địa điểm "<q>".`; upstream failure →
`Không lấy được dữ liệu thời tiết. Thử lại sau nhé.` (same tone as `/giaxang`).

## Rejected options

- Caching forecasts in Mongo: the limits make it unnecessary, and it adds a
  storage schema for no user-visible gain.
- A per-user saved default location: not requested.
- A keyed provider (OWM, WeatherAPI): adds a secret and setup for no gain.

## Next steps

1. Write the plan under `plans/261001-…-thoitiet-weather-module/` and implement.
2. Run `go test ./internal/modules/thoitiet/... ./cmd/server/...`, `go vet ./...`,
   and `golangci-lint run`.

## Decisions

- `/thoitiet` is an alias of `/thoitiethomnay` (user request, 2026-10-01).
- `/thoitiettuannay` covers the next 7 days starting today, not the Mon–Sun
  calendar week (user choice, 2026-10-01).
