package thoitiet

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// sourceLine credits the data source, as Open-Meteo's CC BY 4.0 licence asks.
const sourceLine = "Nguồn: Open-Meteo"

type condition struct {
	label string
	emoji string
}

// wmoConditions maps WMO weather interpretation codes, the "weather_code"
// field Open-Meteo returns, to a Vietnamese label.
var wmoConditions = map[int]condition{
	0:  {"Trời quang", "☀️"},
	1:  {"Ít mây", "🌤️"},
	2:  {"Mây rải rác", "⛅"},
	3:  {"Nhiều mây", "☁️"},
	45: {"Sương mù", "🌫️"},
	48: {"Sương mù đóng băng", "🌫️"},
	51: {"Mưa phùn nhẹ", "🌦️"},
	53: {"Mưa phùn", "🌦️"},
	55: {"Mưa phùn dày", "🌧️"},
	56: {"Mưa phùn băng nhẹ", "🌧️"},
	57: {"Mưa phùn băng", "🌧️"},
	61: {"Mưa nhẹ", "🌧️"},
	63: {"Mưa vừa", "🌧️"},
	65: {"Mưa to", "🌧️"},
	66: {"Mưa băng nhẹ", "🌧️"},
	67: {"Mưa băng", "🌧️"},
	71: {"Tuyết nhẹ", "🌨️"},
	73: {"Tuyết vừa", "🌨️"},
	75: {"Tuyết dày", "❄️"},
	77: {"Tuyết hạt", "🌨️"},
	80: {"Mưa rào nhẹ", "🌦️"},
	81: {"Mưa rào", "🌦️"},
	82: {"Mưa rào rất to", "⛈️"},
	85: {"Mưa tuyết nhẹ", "🌨️"},
	86: {"Mưa tuyết", "🌨️"},
	95: {"Dông", "⛈️"},
	96: {"Dông kèm mưa đá nhẹ", "⛈️"},
	99: {"Dông kèm mưa đá", "⛈️"},
}

func conditionFor(code int) condition {
	if c, ok := wmoConditions[code]; ok {
		return c
	}
	return condition{"Không rõ", "🌡️"}
}

// vnWeekdays are the short Vietnamese weekday names, indexed by time.Weekday.
var vnWeekdays = [...]string{"CN", "T2", "T3", "T4", "T5", "T6", "T7"}

// formatToday renders current conditions plus today's daily summary.
func formatToday(p place, f forecast) string {
	c := f.Current
	cur := conditionFor(c.WeatherCode)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s Thời tiết hôm nay %s — %s\n", conditionFor(f.Daily.WeatherCode[0]).emoji,
		shortDate(f.Daily.Time[0]), displayName(p))
	fmt.Fprintf(&sb, "Hiện tại: %s (cảm giác %s), %s %s\n", temp(c.Temperature), temp(c.ApparentTemperature),
		cur.label, cur.emoji)
	fmt.Fprintf(&sb, "Độ ẩm %d%%, gió %s km/h\n", round(c.Humidity), decimal(c.WindSpeed))
	writeDay(&sb, "Cả ngày", f.Daily, 0)
	sb.WriteString(sourceLine)
	return sb.String()
}

// formatTomorrow renders tomorrow's daily summary.
func formatTomorrow(p place, f forecast) string {
	d := f.Daily
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s Thời tiết ngày mai %s %s — %s\n", conditionFor(d.WeatherCode[1]).emoji,
		weekday(d.Time[1]), shortDate(d.Time[1]), displayName(p))
	writeDay(&sb, "Dự báo", d, 1)
	sb.WriteString(sourceLine)
	return sb.String()
}

// formatWeek renders one line per day for every day in the forecast.
func formatWeek(p place, f forecast) string {
	d := f.Daily
	n := d.days()
	var sb strings.Builder
	fmt.Fprintf(&sb, "📅 Thời tiết %d ngày tới — %s\n", n, displayName(p))
	for i := range n {
		c := conditionFor(d.WeatherCode[i])
		fmt.Fprintf(&sb, "%s %s: %s–%s %s %s, mưa %d%%\n", weekday(d.Time[i]), shortDate(d.Time[i]),
			tempValue(d.TemperatureMin[i]), temp(d.TemperatureMax[i]), c.emoji, c.label,
			round(d.PrecipitationProbability[i]))
	}
	sb.WriteString(sourceLine)
	return sb.String()
}

// writeDay renders the daily lines shared by the today and tomorrow views.
func writeDay(sb *strings.Builder, label string, d dailyWeather, i int) {
	c := conditionFor(d.WeatherCode[i])
	fmt.Fprintf(sb, "%s: %s–%s, %s %s\n", label, tempValue(d.TemperatureMin[i]), temp(d.TemperatureMax[i]),
		c.label, c.emoji)
	fmt.Fprintf(sb, "Khả năng mưa %d%% (%s mm), UV %s\n", round(d.PrecipitationProbability[i]),
		decimal(d.PrecipitationSum[i]), decimal(d.UVIndexMax[i]))
	fmt.Fprintf(sb, "Mặt trời mọc %s, lặn %s\n", clock(d.Sunrise[i]), clock(d.Sunset[i]))
}

func round(v float64) int { return int(math.Round(v)) }

func tempValue(v float64) string { return strconv.Itoa(round(v)) }

func temp(v float64) string { return tempValue(v) + "°C" }

// decimal renders one decimal place with the Vietnamese comma separator.
func decimal(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 1, 64), ".", ",", 1)
}

// shortDate turns Open-Meteo's "2026-10-01" into "01/10".
func shortDate(iso string) string {
	t, err := time.Parse(time.DateOnly, iso)
	if err != nil {
		return iso
	}
	return t.Format("02/01")
}

func weekday(iso string) string {
	t, err := time.Parse(time.DateOnly, iso)
	if err != nil {
		return ""
	}
	return vnWeekdays[t.Weekday()]
}

// clock turns Open-Meteo's local "2026-10-01T05:42" into "05:42".
func clock(iso string) string {
	t, err := time.Parse("2006-01-02T15:04", iso)
	if err != nil {
		return iso
	}
	return t.Format("15:04")
}
