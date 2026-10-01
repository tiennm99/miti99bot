package thoitiet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Open-Meteo endpoints. Both are free for non-commercial use with no API key
// (under 10,000 calls a day). Variables so tests can point them at an
// httptest server.
var (
	geocodeURL  = "https://geocoding-api.open-meteo.com/v1/search"
	forecastURL = "https://api.open-meteo.com/v1/forecast"
)

const (
	httpTimeout = 5 * time.Second
	maxBodySize = 1 << 20

	// geocodeCount is how many candidates to fetch so a Vietnamese match can
	// win over a same-named place abroad.
	geocodeCount = 10
	// forecastDays covers today plus the six days after it.
	forecastDays = 7

	currentFields = "temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m"
	dailyFields   = "weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum," +
		"precipitation_probability_max,uv_index_max,sunrise,sunset"
)

// place is one geocoding result.
type place struct {
	Name        string  `json:"name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	CountryCode string  `json:"country_code"`
	Country     string  `json:"country"`
	Admin1      string  `json:"admin1"`
}

type geocodeResponse struct {
	Results []place `json:"results"`
}

// currentWeather is the "current" block of a forecast response.
type currentWeather struct {
	Temperature         float64 `json:"temperature_2m"`
	ApparentTemperature float64 `json:"apparent_temperature"`
	Humidity            float64 `json:"relative_humidity_2m"`
	WeatherCode         int     `json:"weather_code"`
	WindSpeed           float64 `json:"wind_speed_10m"`
}

// dailyWeather holds parallel per-day arrays; index 0 is today in the
// location's own timezone. A null upstream value decodes as zero.
type dailyWeather struct {
	Time                     []string  `json:"time"`
	WeatherCode              []int     `json:"weather_code"`
	TemperatureMax           []float64 `json:"temperature_2m_max"`
	TemperatureMin           []float64 `json:"temperature_2m_min"`
	PrecipitationSum         []float64 `json:"precipitation_sum"`
	PrecipitationProbability []float64 `json:"precipitation_probability_max"`
	UVIndexMax               []float64 `json:"uv_index_max"`
	Sunrise                  []string  `json:"sunrise"`
	Sunset                   []string  `json:"sunset"`
}

type forecast struct {
	Current currentWeather `json:"current"`
	Daily   dailyWeather   `json:"daily"`
}

// days reports how many complete daily rows the response carries, the length
// of the shortest per-day array, so renderers can index every array safely.
func (d dailyWeather) days() int {
	return min(len(d.Time), len(d.WeatherCode), len(d.TemperatureMax), len(d.TemperatureMin),
		len(d.PrecipitationSum), len(d.PrecipitationProbability), len(d.UVIndexMax),
		len(d.Sunrise), len(d.Sunset))
}

// geocode returns Open-Meteo's matches for name, with Vietnamese labels.
func geocode(ctx context.Context, client *http.Client, name string) ([]place, error) {
	q := url.Values{}
	q.Set("name", name)
	q.Set("count", strconv.Itoa(geocodeCount))
	q.Set("language", "vi")
	q.Set("format", "json")
	var body geocodeResponse
	if err := getJSON(ctx, client, geocodeURL+"?"+q.Encode(), &body); err != nil {
		return nil, fmt.Errorf("geocode: %w", err)
	}
	return body.Results, nil
}

// fetchForecast returns current conditions and a 7-day daily forecast. The
// timezone is the location's own, so "today" is its local date.
func fetchForecast(ctx context.Context, client *http.Client, p place) (forecast, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(p.Latitude, 'f', -1, 64))
	q.Set("longitude", strconv.FormatFloat(p.Longitude, 'f', -1, 64))
	q.Set("timezone", "auto")
	q.Set("forecast_days", strconv.Itoa(forecastDays))
	q.Set("current", currentFields)
	q.Set("daily", dailyFields)
	var body forecast
	if err := getJSON(ctx, client, forecastURL+"?"+q.Encode(), &body); err != nil {
		return forecast{}, fmt.Errorf("forecast: %w", err)
	}
	if body.Daily.days() == 0 {
		return forecast{}, fmt.Errorf("forecast: no daily data")
	}
	return body, nil
}

func getJSON(ctx context.Context, client *http.Client, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodySize)).Decode(out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
