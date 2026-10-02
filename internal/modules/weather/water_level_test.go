package weather

import (
	"encoding/json"
	"os"
	"testing"
)

func loadStationMap(t *testing.T, name string) stationMap {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var m stationMap
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestParseStationMap_SkipsStationsWithoutReadings(t *testing.T) {
	stations := parseStationMap(loadStationMap(t, "vndms_lv0.json"))
	// The fixture has 9 features; Việt Lâm and the duplicate Phước Hòa and
	// Đồng Hới entries have no data, leaving 6 readings.
	if len(stations) != 6 {
		t.Fatalf("stations = %d, want 6: %+v", len(stations), stations)
	}
	var phuAn gaugeStation
	for _, s := range stations {
		if s.Name == "Việt Lâm" {
			t.Error("Việt Lâm has no reading and should be skipped")
		}
		if s.Name == "Phú An" {
			phuAn = s
		}
	}
	want := gaugeStation{Code: "71600", Name: "Phú An", River: "Sài Gòn", Province: "TP. Hồ Chí Minh",
		Latitude: phuAn.Latitude, Longitude: phuAn.Longitude, Level: 1.33, Hour: 7, Day: 2, Month: 10}
	if phuAn != want {
		t.Errorf("Phú An = %+v, want %+v", phuAn, want)
	}
}

func TestStationsNear_TanThuan(t *testing.T) {
	near := stationsNear(parseStationMap(loadStationMap(t, "vndms_lv0.json")), tanThuanPlace, nearbyRadiusKm)
	want := []string{"Phú An", "Nhà Bè", "Biên Hòa", "Thủ Dầu Một"}
	if len(near) != len(want) {
		t.Fatalf("near = %d stations, want %v", len(near), want)
	}
	for i, s := range near {
		if s.Name != want[i] {
			t.Errorf("near[%d] = %s, want %s", i, s.Name, want[i])
		}
	}
	if d := near[0].DistanceKm; d < 2.5 || d > 3.3 {
		t.Errorf("Phú An distance = %.2f km, want about 2.9", d)
	}
}
