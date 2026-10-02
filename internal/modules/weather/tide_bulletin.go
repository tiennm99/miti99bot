package weather

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"golang.org/x/text/unicode/norm"
)

// ictZone is Vietnam's time zone; the bulletin's dates and times are local.
var ictZone = time.FixedZone("ICT", 7*60*60)

// kttvnbURL is Đài KTTV Nam Bộ's site. Its homepage links each day's
// "bản tin dự báo thủy văn TPHCM" article, and the article links the 5-day
// tide bulletin PDF. A variable so tests can point it at an httptest server.
var kttvnbURL = "https://kttvnb.vn"

const (
	// maxPageSize caps the homepage and article HTML (about 50 KB each).
	maxPageSize = 1 << 20
	// maxBulletinSize caps the bulletin PDF (about 320 KB).
	maxBulletinSize = 4 << 20
	// bulletinDays is how many forecast days each station block carries.
	bulletinDays = 5
	// maxTideHeight bounds a plausible peak; anything outside (0, max] means
	// the parser read the wrong column.
	maxTideHeight = 3.0
)

// bulletinStations are the station blocks of the forecast table, in the
// order the PDF prints them. Only the first two are used, but all three are
// checked so a reordered table fails instead of mislabelling peaks.
var bulletinStations = []string{"Phú An", "Nhà Bè", "Thủ Dầu Một"}

var (
	// articleLinkRE matches the homepage link to a TPHCM hydrology bulletin
	// article, capturing the issue date. The slug varies
	// ("...-tphcm-ra-nga-y-02-10-2026", "...-khu-va-c-tphcm-ra-nga-y-..."), so
	// only its tail is matched.
	articleLinkRE = regexp.MustCompile(`href="([^"]*tphcm-ra-nga-y-(\d{2})-(\d{2})-(\d{4}))"`)
	// bulletinLinkRE matches the article's PDF attachment link.
	bulletinLinkRE = regexp.MustCompile(`href="([^"]*/HCMC_TVHN_(\d{8})\.pdf)"`)

	dayTokenRE    = regexp.MustCompile(`^\d{1,2}/$`)
	monthTokenRE  = regexp.MustCompile(`^\d{1,2}$`)
	heightTokenRE = regexp.MustCompile(`^\d+\.\d+$`)
	timeTokenRE   = regexp.MustCompile(`^([01]?\d|2[0-3])\.[0-5]\d$`)
)

// tidePeak is one forecast high tide. Time is "HH:MM" local time.
type tidePeak struct {
	Height float64
	Time   string
}

// tideDay is one forecast day at one station: up to two high tides.
type tideDay struct {
	Date  time.Time
	Peaks []tidePeak
}

// maxPeak returns the day's highest tide, or false when the day has none.
func (d tideDay) maxPeak() (tidePeak, bool) {
	if len(d.Peaks) == 0 {
		return tidePeak{}, false
	}
	return slices.MaxFunc(d.Peaks, func(a, b tidePeak) int {
		switch {
		case a.Height < b.Height:
			return -1
		case a.Height > b.Height:
			return 1
		}
		return 0
	}), true
}

// tideBulletin is the parsed forecast table: one entry per station in
// bulletinStations order, each with bulletinDays days.
type tideBulletin struct {
	Issued   time.Time
	Stations [][]tideDay
}

// fetchTideBulletin finds the newest TPHCM bulletin linked from the KTTV Nam
// Bộ homepage and parses its forecast table. Before the day's bulletin is
// published (about 09:20 ICT) that is the previous day's.
func fetchTideBulletin(ctx context.Context, client *http.Client) (tideBulletin, error) {
	home, err := getBody(ctx, client, kttvnbURL+"/", maxPageSize, nil)
	if err != nil {
		return tideBulletin{}, fmt.Errorf("tide bulletin homepage: %w", err)
	}
	articleURL, err := newestArticle(home)
	if err != nil {
		return tideBulletin{}, err
	}
	article, err := getBody(ctx, client, articleURL, maxPageSize, nil)
	if err != nil {
		return tideBulletin{}, fmt.Errorf("tide bulletin article: %w", err)
	}
	pdfURL, issued, err := bulletinLink(article)
	if err != nil {
		return tideBulletin{}, err
	}
	data, err := getBody(ctx, client, pdfURL, maxBulletinSize, nil)
	if err != nil {
		return tideBulletin{}, fmt.Errorf("tide bulletin pdf: %w", err)
	}
	b, err := parseTideBulletin(data, issued)
	if err != nil {
		return tideBulletin{}, fmt.Errorf("tide bulletin %s: %w", pdfURL, err)
	}
	return b, nil
}

// newestArticle returns the absolute URL of the latest-dated bulletin article
// linked from the homepage.
func newestArticle(home []byte) (string, error) {
	var best string
	var bestDate time.Time
	for _, m := range articleLinkRE.FindAllSubmatch(home, -1) {
		d, err := time.Parse("02-01-2006", string(m[2])+"-"+string(m[3])+"-"+string(m[4]))
		if err != nil || !d.After(bestDate) {
			continue
		}
		best, bestDate = string(m[1]), d
	}
	if best == "" {
		return "", errors.New("tide bulletin: no article link on homepage")
	}
	return resolveKTTVNB(best)
}

// bulletinLink returns the article's PDF URL and the issue date encoded in
// its file name.
func bulletinLink(article []byte) (string, time.Time, error) {
	m := bulletinLinkRE.FindSubmatch(article)
	if m == nil {
		return "", time.Time{}, errors.New("tide bulletin: no PDF link in article")
	}
	issued, err := time.ParseInLocation("20060102", string(m[2]), ictZone)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("tide bulletin: issue date: %w", err)
	}
	u, err := resolveKTTVNB(string(m[1]))
	return u, issued, err
}

// resolveKTTVNB makes a possibly relative link absolute against kttvnbURL.
func resolveKTTVNB(link string) (string, error) {
	base, err := url.Parse(kttvnbURL + "/")
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(link)
	if err != nil {
		return "", fmt.Errorf("tide bulletin link %q: %w", link, err)
	}
	return base.ResolveReference(ref).String(), nil
}

// parseTideBulletin extracts the forecast table from the bulletin PDF.
//
// The PDF's text layer garbles labels (letters split and reordered), but each
// forecast row keeps its date and values in reading order:
//
//	02/ 10 1.33 07.00 1.29 21.00 - 1.79 15.00 - 0.04 01.00
//
// that is date, high 1 (height, time), high 2, low 1, low 2, with "ct" for a
// missing high. The date tokens can come out swapped ("10 04/"), with the
// station label in front on a block's middle row. Rows are matched by their
// date token, and the result is accepted only if it has bulletinDays
// consecutive dates for every station in bulletinStations, the station's
// label is an anagram of its name, and every height is plausible. The label
// sits in front of the block's middle row, or on its own text row a point
// away from it.
func parseTideBulletin(data []byte, issued time.Time) (b tideBulletin, err error) {
	// The pdf package panics on some malformed files instead of returning an
	// error; a bad download must not take the handler down with it.
	defer func() {
		if p := recover(); p != nil {
			b, err = tideBulletin{}, fmt.Errorf("read pdf: %v", p)
		}
	}()
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return tideBulletin{}, errors.New("not a PDF")
	}
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return tideBulletin{}, fmt.Errorf("open pdf: %w", err)
	}
	var rows []bulletinRow
	var labels []textLine
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		textRows, err := page.GetTextByRow()
		if err != nil {
			return tideBulletin{}, fmt.Errorf("read page %d: %w", i, err)
		}
		for _, tr := range textRows {
			var tokens []string
			for _, t := range tr.Content {
				tokens = append(tokens, strings.Fields(t.S)...)
			}
			line := textLine{page: i, y: tr.Position, text: strings.Join(tokens, "")}
			if row, ok := parseBulletinRow(tokens, issued); ok {
				row.line = line
				rows = append(rows, row)
			} else {
				labels = append(labels, line)
			}
		}
	}
	return assembleBulletin(rows, labels, issued)
}

// textLine is one text row of the PDF and where it sits.
type textLine struct {
	page int
	y    int64
	text string
}

// labelRowGap is how far, in PDF points, a station label on its own row may
// sit from the forecast row it labels.
const labelRowGap = 3

// bulletinRow is one parsed forecast row plus the label text in front of it.
type bulletinRow struct {
	label string
	day   tideDay
	line  textLine
}

// parseBulletinRow reads one text row. It reports false for rows that are not
// forecast rows (headers, observed data, notes).
func parseBulletinRow(tokens []string, issued time.Time) (bulletinRow, bool) {
	dayIdx := slices.IndexFunc(tokens, dayTokenRE.MatchString)
	if dayIdx < 0 {
		return bulletinRow{}, false
	}
	monthIdx := -1
	for _, i := range []int{dayIdx + 1, dayIdx - 1} {
		if i >= 0 && i < len(tokens) && monthTokenRE.MatchString(tokens[i]) {
			monthIdx = i
			break
		}
	}
	if monthIdx < 0 {
		return bulletinRow{}, false
	}
	day, _ := strconv.Atoi(strings.TrimSuffix(tokens[dayIdx], "/"))
	month, _ := strconv.Atoi(tokens[monthIdx])
	date, ok := bulletinDate(day, month, issued)
	if !ok {
		return bulletinRow{}, false
	}
	first := min(dayIdx, monthIdx)
	values := tokens[max(dayIdx, monthIdx)+1:]
	if len(values) < 4 {
		return bulletinRow{}, false
	}
	var peaks []tidePeak
	for i := 0; i < 4; i += 2 {
		h, t := values[i], values[i+1]
		if h == "ct" && t == "ct" {
			continue
		}
		if !heightTokenRE.MatchString(h) || !timeTokenRE.MatchString(t) {
			return bulletinRow{}, false
		}
		height, _ := strconv.ParseFloat(h, 64)
		hour, minute, _ := strings.Cut(t, ".")
		if len(hour) == 1 {
			hour = "0" + hour
		}
		peaks = append(peaks, tidePeak{Height: height, Time: hour + ":" + minute})
	}
	return bulletinRow{label: strings.Join(tokens[:first], ""), day: tideDay{Date: date, Peaks: peaks}}, true
}

// bulletinDate builds a forecast date from the row's day and month. The table
// has no year: a month far before the issue month means the forecast crossed
// into the next year.
func bulletinDate(day, month int, issued time.Time) (time.Time, bool) {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	year := issued.Year()
	if month < int(issued.Month())-6 {
		year++
	}
	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, ictZone)
	if d.Day() != day {
		return time.Time{}, false
	}
	return d, true
}

func assembleBulletin(rows []bulletinRow, labels []textLine, issued time.Time) (tideBulletin, error) {
	want := len(bulletinStations) * bulletinDays
	if len(rows) != want {
		return tideBulletin{}, fmt.Errorf("found %d forecast rows, want %d", len(rows), want)
	}
	b := tideBulletin{Issued: issued}
	for s, name := range bulletinStations {
		block := rows[s*bulletinDays : (s+1)*bulletinDays]
		if !labelled(block[bulletinDays/2], labels, name) {
			return tideBulletin{}, fmt.Errorf("block %d is not labelled %s", s, name)
		}
		days := make([]tideDay, 0, bulletinDays)
		for i, row := range block {
			if !row.day.Date.Equal(rows[i].day.Date) || (i > 0 && !row.day.Date.Equal(days[i-1].Date.AddDate(0, 0, 1))) {
				return tideBulletin{}, fmt.Errorf("%s: dates are not %d consecutive days", name, bulletinDays)
			}
			for _, p := range row.day.Peaks {
				if p.Height <= 0 || p.Height > maxTideHeight {
					return tideBulletin{}, fmt.Errorf("%s: implausible peak %.2f m", name, p.Height)
				}
			}
			days = append(days, row.day)
		}
		b.Stations = append(b.Stations, days)
	}
	return b, nil
}

// labelled reports whether row carries name as its label, either in front of
// its values or on a separate text row right next to it.
func labelled(row bulletinRow, labels []textLine, name string) bool {
	if sameLetters(row.label, name) {
		return true
	}
	for _, l := range labels {
		gap := l.y - row.line.y
		if l.page == row.line.page && gap >= -labelRowGap && gap <= labelRowGap && sameLetters(l.text, name) {
			return true
		}
	}
	return false
}

// sameLetters reports whether a and b use the same letters, ignoring spaces
// and order; the PDF text layer scrambles a label's letters but keeps them.
func sameLetters(a, b string) bool {
	letters := func(s string) []rune {
		r := []rune(strings.Join(strings.Fields(norm.NFC.String(s)), ""))
		slices.Sort(r)
		return r
	}
	return slices.Equal(letters(a), letters(b))
}
