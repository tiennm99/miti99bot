package wc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const ictOffset = 7 * time.Hour

const formatHint = "Use dd, dd-mm, dd/mm, ddmm, dd-mm-yyyy, dd/mm/yyyy, or ddmmyyyy."

// IctLocation is the fixed-offset UTC+7 timezone for schedule display.
var IctLocation = time.FixedZone("ICT", int(ictOffset/time.Second))

type parseDateResult struct {
	OK    bool
	Date  time.Time
	Error string
}

var digitsOnly = regexp.MustCompile(`^\d+$`)

func ictDayStartOf(now time.Time) time.Time {
	ict := now.In(IctLocation)
	dayStart := time.Date(ict.Year(), ict.Month(), ict.Day(), 0, 0, 0, 0, IctLocation)
	return dayStart.UTC()
}

func ictWeekStartOf(now time.Time) time.Time {
	day := ictDayStartOf(now).In(IctLocation)
	daysFromMonday := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -daysFromMonday).UTC()
}

func addDays(date time.Time, days int) time.Time {
	return date.Add(time.Duration(days) * 24 * time.Hour)
}

func splitParts(trimmed string) ([]string, string) {
	if strings.ContainsAny(trimmed, "-/") {
		normalized := strings.ReplaceAll(trimmed, "/", "-")
		parts := strings.Split(normalized, "-")
		if len(parts) < 1 || len(parts) > 3 {
			return nil, fmt.Sprintf(`Invalid date %q. %s`, trimmed, formatHint)
		}
		for _, p := range parts {
			if p == "" || !digitsOnly.MatchString(p) {
				return nil, fmt.Sprintf(`Invalid date %q. %s`, trimmed, formatHint)
			}
		}
		return parts, ""
	}

	if !digitsOnly.MatchString(trimmed) {
		return nil, fmt.Sprintf(`Invalid date %q. %s`, trimmed, formatHint)
	}
	switch len(trimmed) {
	case 1, 2:
		return []string{trimmed}, ""
	case 4:
		return []string{trimmed[:2], trimmed[2:]}, ""
	case 8:
		return []string{trimmed[:2], trimmed[2:4], trimmed[4:]}, ""
	default:
		return nil, fmt.Sprintf(`Invalid date %q. %s`, trimmed, formatHint)
	}
}

// ParseScheduleDate parses a /wc date argument. Empty input means today in ICT.
func ParseScheduleDate(input string, now time.Time) parseDateResult {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return parseDateResult{OK: true, Date: ictDayStartOf(now)}
	}

	parts, errMsg := splitParts(trimmed)
	if errMsg != "" {
		return parseDateResult{Error: errMsg}
	}

	ictNow := now.In(IctLocation)
	day, _ := strconv.Atoi(parts[0])
	month := int(ictNow.Month())
	year := ictNow.Year()
	if len(parts) >= 2 {
		month, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		year, _ = strconv.Atoi(parts[2])
	}

	if day < 1 || day > 31 {
		return parseDateResult{Error: fmt.Sprintf(`Invalid day %q - must be 1-31.`, parts[0])}
	}
	if month < 1 || month > 12 {
		monthStr := ""
		if len(parts) >= 2 {
			monthStr = parts[1]
		}
		return parseDateResult{Error: fmt.Sprintf(`Invalid month %q - must be 1-12.`, monthStr)}
	}
	if year < 1970 || year > 2100 {
		yearStr := ""
		if len(parts) >= 3 {
			yearStr = parts[2]
		}
		return parseDateResult{Error: fmt.Sprintf(`Invalid year %q.`, yearStr)}
	}

	candidate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, IctLocation)
	if candidate.Year() != year || int(candidate.Month()) != month || candidate.Day() != day {
		return parseDateResult{Error: fmt.Sprintf(`Invalid date - %d/%d/%d does not exist.`, day, month, year)}
	}
	return parseDateResult{OK: true, Date: candidate.UTC()}
}
