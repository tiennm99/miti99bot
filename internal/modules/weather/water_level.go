package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// vndmsURL is the Vietnam Disaster Monitoring System (Cục Quản lý đê điều và
// PCTT), which republishes KTTV river gauge readings. Its station-map
// endpoint is undocumented and answers 403 without a same-site Referer. A
// variable so tests can point it at an httptest server.
var vndmsURL = "https://vndms.gov.vn"

const (
	// maxWaterLevelSize caps one station-map response; the full list is
	// about 430 KB decoded.
	maxWaterLevelSize = 4 << 20
	// alarmTiers is the number of báo động levels (I, II, III). The map
	// endpoint's lv=N lists only the stations at tier N; lv=0 lists all.
	alarmTiers = 3
)

var (
	popupNameRE     = regexp.MustCompile(`Tên trạm: <b>([^<]+)</b>`)
	popupCodeRE     = regexp.MustCompile(`Mã trạm: <b>([^<]+)</b>`)
	popupProvinceRE = regexp.MustCompile(`Địa điểm: <b>([^<]+)</b>`)
	popupRiverRE    = regexp.MustCompile(`Sông: <b>([^<]+)</b>`)
	// popupLevelRE matches "Mực nước (1.33(m) 7-02/10)": metres, then the
	// reading's hour and day/month. Stations without data print
	// "Mực nước :Không có số liệu" instead and are skipped.
	popupLevelRE = regexp.MustCompile(`Mực nước \((-?\d+(?:\.\d+)?)\(m\) (\d{1,2})-(\d{1,2})/(\d{1,2})\)`)
)

// gaugeStation is one river gauge with its latest reading.
type gaugeStation struct {
	Code, Name, River, Province string
	Latitude, Longitude         float64
	Level                       float64 // metres, in the station's own datum
	Hour, Day, Month            int     // reading time, local
	Tier                        int     // báo động tier 1–3, or 0 below alarm
}

type stationMap struct {
	Features []struct {
		Properties struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longtitude"` // sic
			Popup     string  `json:"popupInfo"`
		} `json:"properties"`
	} `json:"features"`
}

// fetchGaugeStations returns every station with a current reading, each
// tagged with its alarm tier. The four map requests run concurrently, and
// any failure fails the whole lookup so a missing tier list can never read as
// "below alarm".
func fetchGaugeStations(ctx context.Context, client *http.Client) ([]gaugeStation, error) {
	maps := make([]stationMap, alarmTiers+1)
	errs := make([]error, alarmTiers+1)
	var wg sync.WaitGroup
	for lv := range maps {
		wg.Go(func() {
			maps[lv], errs[lv] = fetchStationMap(ctx, client, lv)
		})
	}
	wg.Wait()
	for lv, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("water level lv=%d: %w", lv, err)
		}
	}
	stations := parseStationMap(maps[0])
	if len(stations) == 0 {
		return nil, fmt.Errorf("water level: no station readings")
	}
	tiers := map[string]int{}
	for lv := 1; lv <= alarmTiers; lv++ {
		for _, s := range parseStationMap(maps[lv]) {
			tiers[s.Code] = max(tiers[s.Code], lv)
		}
	}
	for i := range stations {
		stations[i].Tier = tiers[stations[i].Code]
	}
	return stations, nil
}

func fetchStationMap(ctx context.Context, client *http.Client, lv int) (stationMap, error) {
	body, err := getBody(ctx, client, vndmsURL+"/water_level?lv="+strconv.Itoa(lv), maxWaterLevelSize,
		http.Header{"Referer": {vndmsURL + "/"}, "Accept": {"application/json"}})
	if err != nil {
		return stationMap{}, err
	}
	var m stationMap
	if err := json.Unmarshal(body, &m); err != nil {
		return stationMap{}, fmt.Errorf("decode: %w", err)
	}
	return m, nil
}

// parseStationMap reads the station fields out of each feature's popup HTML,
// keeping only stations that have a code and a current reading.
func parseStationMap(m stationMap) []gaugeStation {
	var out []gaugeStation
	for _, f := range m.Features {
		p := f.Properties.Popup
		lm := popupLevelRE.FindStringSubmatch(p)
		code := popupField(popupCodeRE, p)
		if lm == nil || code == "" {
			continue
		}
		level, err := strconv.ParseFloat(lm[1], 64)
		if err != nil {
			continue
		}
		hour, _ := strconv.Atoi(lm[2])
		day, _ := strconv.Atoi(lm[3])
		month, _ := strconv.Atoi(lm[4])
		out = append(out, gaugeStation{
			Code:      code,
			Name:      popupField(popupNameRE, p),
			River:     popupField(popupRiverRE, p),
			Province:  popupField(popupProvinceRE, p),
			Latitude:  f.Properties.Latitude,
			Longitude: f.Properties.Longitude,
			Level:     level,
			Hour:      hour,
			Day:       day,
			Month:     month,
		})
	}
	return out
}

func popupField(re *regexp.Regexp, popup string) string {
	m := re.FindStringSubmatch(popup)
	if m == nil {
		return ""
	}
	return html.UnescapeString(strings.TrimSpace(m[1]))
}

// nearbyStation is a station with its distance from a reference point.
type nearbyStation struct {
	gaugeStation
	DistanceKm float64
}

// stationsNear returns the stations within radiusKm of p, nearest first.
func stationsNear(stations []gaugeStation, p place, radiusKm float64) []nearbyStation {
	var out []nearbyStation
	for _, s := range stations {
		if d := distanceKm(p.Latitude, p.Longitude, s.Latitude, s.Longitude); d <= radiusKm {
			out = append(out, nearbyStation{gaugeStation: s, DistanceKm: d})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DistanceKm < out[j].DistanceKm })
	return out
}

// distanceKm is the haversine great-circle distance.
func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLon := rad(lat2-lat1), rad(lon2-lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}
