package thoitiet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

// forecastFixture mirrors a live Open-Meteo response for Ho Chi Minh City.
const forecastFixture = `{
	"current":{"temperature_2m":29.0,"apparent_temperature":35.7,"relative_humidity_2m":78,"weather_code":3,"wind_speed_10m":3.2},
	"daily":{
		"time":["2026-10-01","2026-10-02","2026-10-03","2026-10-04","2026-10-05","2026-10-06","2026-10-07"],
		"weather_code":[80,80,81,95,95,95,53],
		"temperature_2m_max":[32.7,31.8,32.0,32.2,32.5,31.1,31.1],
		"temperature_2m_min":[24.3,23.9,24.2,23.7,24.5,24.0,23.9],
		"precipitation_sum":[5.30,7.90,7.00,5.70,13.90,4.00,7.60],
		"precipitation_probability_max":[70,88,85,92,98,100,89],
		"uv_index_max":[8.90,8.20,8.60,8.40,8.85,7.80,8.70],
		"sunrise":["2026-10-01T05:42","2026-10-02T05:42","2026-10-03T05:42","2026-10-04T05:42","2026-10-05T05:42","2026-10-06T05:42","2026-10-07T05:42"],
		"sunset":["2026-10-01T17:44","2026-10-02T17:43","2026-10-03T17:42","2026-10-04T17:42","2026-10-05T17:41","2026-10-06T17:40","2026-10-07T17:40"]
	}
}`

const daLatGeocodeFixture = `{"results":[
	{"name":"Ðà Lạt","latitude":11.94646,"longitude":108.44193,"country_code":"VN","country":"Việt Nam","admin1":"Lam Dong"}
]}`

// fakeOpenMeteo serves both Open-Meteo endpoints for the duration of t and
// records the geocoding queries it received.
type fakeOpenMeteo struct {
	geocodeBody    string
	forecastBody   string
	forecastStatus int
	geocodeQueries []string
	forecastCalls  atomic.Int32
	lastForecastQ  map[string]string
}

func stubOpenMeteo(t *testing.T, f *fakeOpenMeteo) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			f.geocodeQueries = append(f.geocodeQueries, r.URL.Query().Get("name"))
			_, _ = w.Write([]byte(f.geocodeBody))
		case "/forecast":
			f.forecastCalls.Add(1)
			f.lastForecastQ = map[string]string{}
			for k := range r.URL.Query() {
				f.lastForecastQ[k] = r.URL.Query().Get(k)
			}
			if f.forecastStatus != 0 {
				http.Error(w, "unavailable", f.forecastStatus)
				return
			}
			_, _ = w.Write([]byte(f.forecastBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	origGeocode, origForecast := geocodeURL, forecastURL
	geocodeURL, forecastURL = server.URL+"/search", server.URL+"/forecast"
	t.Cleanup(func() { geocodeURL, forecastURL = origGeocode, origForecast })
}

func installThoitiet(t *testing.T) *testutil.RecordingBot {
	t.Helper()
	rb := testutil.NewRecordingBot(t)
	mod := New(modules.Deps{Store: storage.NewMemoryProvider().Collection("thoitiet")})
	reg := &modules.Registry{
		Modules:     []modules.Module{{Name: "thoitiet", Commands: mod.Commands}},
		AllCommands: map[string]modules.Command{},
	}
	for _, c := range mod.Commands {
		reg.AllCommands[c.Name] = c
	}
	modules.Install(rb.Bot, reg, modules.Auth{})
	return rb
}

func send(rb *testutil.RecordingBot, text string) string {
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, text))
	return rb.LastSent().Text()
}

func TestCommands_RegistrationAndParameters(t *testing.T) {
	mod := New(modules.Deps{})
	want := []string{"thoitiethomnay", "thoitiet", "thoitietngaymai", "thoitiettuannay"}
	if len(mod.Commands) != len(want) {
		t.Fatalf("commands = %d, want %d", len(mod.Commands), len(want))
	}
	for i, c := range mod.Commands {
		if c.Name != want[i] {
			t.Errorf("commands[%d] = %q, want %q", i, c.Name, want[i])
		}
		if c.Parameters != "[location...]" {
			t.Errorf("/%s parameters = %q", c.Name, c.Parameters)
		}
		if c.Visibility != modules.VisibilityPublic {
			t.Errorf("/%s is not public", c.Name)
		}
	}
}

func TestToday_DefaultsToHCMWithoutGeocoding(t *testing.T) {
	f := &fakeOpenMeteo{forecastBody: forecastFixture}
	stubOpenMeteo(t, f)
	rb := installThoitiet(t)

	want := "🌦️ Thời tiết hôm nay 01/10 — Thành phố Hồ Chí Minh\n" +
		"Hiện tại: 29°C (cảm giác 36°C), Nhiều mây ☁️\n" +
		"Độ ẩm 78%, gió 3,2 km/h\n" +
		"Cả ngày: 24–33°C, Mưa rào nhẹ 🌦️\n" +
		"Khả năng mưa 70% (5,3 mm), UV 8,9\n" +
		"Mặt trời mọc 05:42, lặn 17:44\n" +
		"Nguồn: Open-Meteo"
	for _, cmd := range []string{"/thoitiethomnay", "/thoitiet", "/thoitiet hcm", "/thoitiet Sài Gòn"} {
		if got := send(rb, cmd); got != want {
			t.Errorf("%s reply =\n%s\nwant\n%s", cmd, got, want)
		}
	}
	if len(f.geocodeQueries) != 0 {
		t.Errorf("geocode called with %v, want no calls", f.geocodeQueries)
	}
	if f.lastForecastQ["latitude"] != "10.82302" || f.lastForecastQ["timezone"] != "auto" ||
		f.lastForecastQ["forecast_days"] != "7" {
		t.Errorf("forecast query = %v", f.lastForecastQ)
	}
}

func TestTomorrow_GeocodesDiacriticLocation(t *testing.T) {
	f := &fakeOpenMeteo{geocodeBody: daLatGeocodeFixture, forecastBody: forecastFixture}
	stubOpenMeteo(t, f)
	rb := installThoitiet(t)

	got := send(rb, "/thoitietngaymai  Đà   Lạt ")
	want := "🌦️ Thời tiết ngày mai T6 02/10 — Ðà Lạt, Lam Dong\n" +
		"Dự báo: 24–32°C, Mưa rào nhẹ 🌦️\n" +
		"Khả năng mưa 88% (7,9 mm), UV 8,2\n" +
		"Mặt trời mọc 05:42, lặn 17:43\n" +
		"Nguồn: Open-Meteo"
	if got != want {
		t.Errorf("reply =\n%s\nwant\n%s", got, want)
	}
	if len(f.geocodeQueries) != 1 || f.geocodeQueries[0] != "da lat" {
		t.Errorf("geocode queries = %q, want [\"da lat\"]", f.geocodeQueries)
	}
	if f.lastForecastQ["latitude"] != "11.94646" {
		t.Errorf("forecast latitude = %q", f.lastForecastQ["latitude"])
	}
}

func TestWeek_ListsSevenDays(t *testing.T) {
	f := &fakeOpenMeteo{forecastBody: forecastFixture}
	stubOpenMeteo(t, f)
	rb := installThoitiet(t)

	want := "📅 Thời tiết 7 ngày tới — Thành phố Hồ Chí Minh\n" +
		"T5 01/10: 24–33°C 🌦️ Mưa rào nhẹ, mưa 70%\n" +
		"T6 02/10: 24–32°C 🌦️ Mưa rào nhẹ, mưa 88%\n" +
		"T7 03/10: 24–32°C 🌦️ Mưa rào, mưa 85%\n" +
		"CN 04/10: 24–32°C ⛈️ Dông, mưa 92%\n" +
		"T2 05/10: 25–33°C ⛈️ Dông, mưa 98%\n" +
		"T3 06/10: 24–31°C ⛈️ Dông, mưa 100%\n" +
		"T4 07/10: 24–31°C 🌦️ Mưa phùn, mưa 89%\n" +
		"Nguồn: Open-Meteo"
	if got := send(rb, "/thoitiettuannay"); got != want {
		t.Errorf("reply =\n%s\nwant\n%s", got, want)
	}
}

func TestAliasExpansionAndForeignPlace(t *testing.T) {
	f := &fakeOpenMeteo{
		geocodeBody: `{"results":[
			{"name":"Tokyo","latitude":35.6895,"longitude":139.69171,"country_code":"JP","country":"Nhật Bản","admin1":"Tokyo"}
		]}`,
		forecastBody: forecastFixture,
	}
	stubOpenMeteo(t, f)
	rb := installThoitiet(t)

	got := send(rb, "/thoitiettuannay Tokyo")
	if !strings.HasPrefix(got, "📅 Thời tiết 7 ngày tới — Tokyo, Nhật Bản\n") {
		t.Errorf("reply header = %q", strings.SplitN(got, "\n", 2)[0])
	}
	send(rb, "/thoitiet HN")
	if want := []string{"tokyo", "Ha Noi"}; strings.Join(f.geocodeQueries, "|") != strings.Join(want, "|") {
		t.Errorf("geocode queries = %q, want %q", f.geocodeQueries, want)
	}
}

func TestUnknownLocation(t *testing.T) {
	f := &fakeOpenMeteo{geocodeBody: `{"generationtime_ms":0.3}`}
	stubOpenMeteo(t, f)
	rb := installThoitiet(t)

	if got, want := send(rb, "/thoitiet xyzzy"), `Không tìm thấy địa điểm "xyzzy".`; got != want {
		t.Errorf("reply = %q, want %q", got, want)
	}
	if n := f.forecastCalls.Load(); n != 0 {
		t.Errorf("forecast calls = %d, want 0", n)
	}
}

func TestUpstreamFailureRepliesError(t *testing.T) {
	cases := map[string]*fakeOpenMeteo{
		"forecast non-200":  {forecastStatus: http.StatusServiceUnavailable},
		"forecast bad json": {forecastBody: "<html>"},
		"no daily rows":     {forecastBody: `{"daily":{"time":[]}}`},
		"geocode bad json":  {geocodeBody: "<html>"},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			stubOpenMeteo(t, f)
			rb := installThoitiet(t)
			cmd := "/thoitiet"
			if f.geocodeBody != "" {
				cmd = "/thoitiet Hue"
			}
			if got := send(rb, cmd); got != fetchErrorText {
				t.Errorf("reply = %q, want %q", got, fetchErrorText)
			}
		})
	}
}

func TestTomorrow_NeedsTwoDailyRows(t *testing.T) {
	oneDay := strings.NewReplacer(
		`,"2026-10-02","2026-10-03","2026-10-04","2026-10-05","2026-10-06","2026-10-07"`, "",
	).Replace(forecastFixture)
	f := &fakeOpenMeteo{forecastBody: oneDay}
	stubOpenMeteo(t, f)
	rb := installThoitiet(t)

	if got := send(rb, "/thoitietngaymai"); got != fetchErrorText {
		t.Errorf("reply = %q, want %q", got, fetchErrorText)
	}
	if got := send(rb, "/thoitiettuannay"); !strings.Contains(got, "Thời tiết 1 ngày tới") {
		t.Errorf("week reply = %q, want a one-day list", got)
	}
}
