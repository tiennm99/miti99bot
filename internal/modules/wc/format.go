package wc

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

func formatIctTime(t time.Time) string {
	d := t.In(IctLocation)
	return fmt.Sprintf("%02d:%02d", d.Hour(), d.Minute())
}

func formatIctDayLabel(t time.Time) string {
	weekdays := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	d := t.In(IctLocation)
	return fmt.Sprintf("%s %s %d", weekdays[d.Weekday()], months[d.Month()-1], d.Day())
}

func ictDayKey(t time.Time) string {
	d := t.In(IctLocation)
	return fmt.Sprintf("%04d-%02d-%02d", d.Year(), int(d.Month()), d.Day())
}

func teamLabel(t Team) string {
	for _, s := range []string{t.TLA, t.ShortName, t.Name} {
		s = strings.TrimSpace(s)
		if s != "" && !strings.EqualFold(s, "TBD") {
			return s
		}
	}
	return "TBD"
}

func stageLabel(stage, group string) string {
	if strings.TrimSpace(group) != "" {
		return titleToken(group)
	}
	switch stage {
	case "GROUP_STAGE":
		return "Group Stage"
	case "LAST_16":
		return "Last 16"
	case "QUARTER_FINALS":
		return "Quarter-finals"
	case "SEMI_FINALS":
		return "Semi-finals"
	case "THIRD_PLACE":
		return "Third Place"
	case "FINAL":
		return "Final"
	default:
		return titleToken(stage)
	}
}

func titleToken(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "_", " "))
	if s == "" {
		return ""
	}
	parts := strings.Fields(strings.ToLower(s))
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func scorePair(score Score) (int, int, bool) {
	if score.FullTime.Home != nil && score.FullTime.Away != nil {
		return *score.FullTime.Home, *score.FullTime.Away, true
	}
	if score.HalfTime.Home != nil && score.HalfTime.Away != nil {
		return *score.HalfTime.Home, *score.HalfTime.Away, true
	}
	return 0, 0, false
}

func formatMatchLine(m Match) string {
	t, err := time.Parse(time.RFC3339, m.UTCDate)
	if err != nil {
		t = time.Time{}
	}
	home := html.EscapeString(teamLabel(m.HomeTeam))
	away := html.EscapeString(teamLabel(m.AwayTeam))
	meta := stageLabel(m.Stage, m.Group)
	if m.Venue != "" {
		if meta != "" {
			meta += " - "
		}
		meta += m.Venue
	}
	if meta != "" {
		meta = " (" + html.EscapeString(meta) + ")"
	}

	homeGoals, awayGoals, hasScore := scorePair(m.Score)
	switch m.Status {
	case "IN_PLAY", "PAUSED":
		if hasScore {
			return fmt.Sprintf("LIVE %s %d-%d %s%s", home, homeGoals, awayGoals, away, meta)
		}
		return fmt.Sprintf("LIVE %s vs %s%s", home, away, meta)
	case "FINISHED", "AWARDED":
		if m.Score.Winner == "HOME_TEAM" {
			home = "<b>" + home + "</b>"
		}
		if m.Score.Winner == "AWAY_TEAM" {
			away = "<b>" + away + "</b>"
		}
		if hasScore {
			return fmt.Sprintf("FT %s %d-%d %s%s", home, homeGoals, awayGoals, away, meta)
		}
		return fmt.Sprintf("FT %s vs %s%s", home, away, meta)
	case "POSTPONED", "SUSPENDED", "CANCELLED":
		return fmt.Sprintf("%s %s vs %s%s", strings.ReplaceAll(m.Status, "_", " "), home, away, meta)
	default:
		return fmt.Sprintf("%s %s vs %s%s", formatIctTime(t), home, away, meta)
	}
}

// RenderToday renders all World Cup matches on one ICT day.
func RenderToday(matches []Match, day time.Time) string {
	header := "<b>World Cup - " + html.EscapeString(formatIctDayLabel(day)) + "</b> (ICT)"
	if len(matches) == 0 {
		return header + "\nNo matches today."
	}
	lines := make([]string, len(matches))
	for i, m := range matches {
		lines[i] = formatMatchLine(m)
	}
	return header + "\n" + strings.Join(lines, "\n")
}

// RenderWeek renders matches grouped by ICT day.
func RenderWeek(matches []Match, from, to time.Time) string {
	fromLbl := html.EscapeString(formatIctDayLabel(from))
	toLbl := html.EscapeString(formatIctDayLabel(to.Add(-time.Millisecond)))
	header := "<b>World Cup - " + fromLbl + " -> " + toLbl + "</b> (ICT)"
	if len(matches) == 0 {
		return header + "\nNo matches this week."
	}

	type dayBucket struct {
		Label string
		Lines []string
	}
	days := map[string]*dayBucket{}
	for _, m := range matches {
		t, err := time.Parse(time.RFC3339, m.UTCDate)
		if err != nil {
			continue
		}
		key := ictDayKey(t)
		d, ok := days[key]
		if !ok {
			d = &dayBucket{Label: formatIctDayLabel(t)}
			days[key] = d
		}
		d.Lines = append(d.Lines, formatMatchLine(m))
	}
	keys := make([]string, 0, len(days))
	for k := range days {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sections := make([]string, len(keys))
	for i, k := range keys {
		d := days[k]
		sections[i] = "<i>" + html.EscapeString(d.Label) + "</i>\n" + strings.Join(d.Lines, "\n")
	}
	return header + "\n\n" + strings.Join(sections, "\n\n")
}
