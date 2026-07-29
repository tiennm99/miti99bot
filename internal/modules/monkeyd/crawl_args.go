package monkeyd

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Font size bounds for the optional argument. The body font drives the heading
// and title sizes too, and the default page is only 78mm wide between margins,
// so the useful range is narrow: at 6pt a line holds about 72 characters, at
// 24pt about 18.
const (
	minFontSize = 6.0
	maxFontSize = 24.0
)

var errTooManyArgs = errors.New("too many arguments")

// crawlArgs is a validated /monkeyd_crawl invocation.
type crawlArgs struct {
	// NovelURL is normalised and known to be on an allowed host.
	NovelURL string

	// FontSize is the requested body font size in points, or 0 to leave the
	// crawler's default in place.
	FontSize float64
}

// parseCrawlArgs validates the text after the command: a novel URL, optionally
// followed by a body font size.
func parseCrawlArgs(arg string) (crawlArgs, error) {
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return crawlArgs{}, errNotAURL
	}
	if len(fields) > 2 {
		return crawlArgs{}, errTooManyArgs
	}

	novelURL, err := normalizeNovelURL(fields[0])
	if err != nil {
		return crawlArgs{}, err
	}
	parsed := crawlArgs{NovelURL: novelURL}

	if len(fields) == 2 {
		if parsed.FontSize, err = parseFontSize(fields[1]); err != nil {
			return crawlArgs{}, err
		}
	}
	return parsed, nil
}

// parseFontSize accepts a font size in points, including half points, and
// rejects anything outside the readable range.
func parseFontSize(raw string) (float64, error) {
	size, err := strconv.ParseFloat(raw, 64)
	// ParseFloat accepts "NaN" and "Inf", neither of which is a font size, and
	// both would reach the renderer as a silently broken layout.
	if err != nil || math.IsNaN(size) || math.IsInf(size, 0) {
		return 0, fmt.Errorf("font size %q is not a number", raw)
	}
	if size < minFontSize || size > maxFontSize {
		return 0, fmt.Errorf("font size must be between %g and %g points", minFontSize, maxFontSize)
	}
	return size, nil
}
