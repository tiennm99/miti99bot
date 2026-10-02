package weather

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// hcmPlace is the default location. Its coordinates are Open-Meteo's own
// geocoding result for the city, so the default needs no geocoding request.
var hcmPlace = place{
	Name:        "Thành phố Hồ Chí Minh",
	Latitude:    10.82302,
	Longitude:   106.62965,
	CountryCode: "VN",
	Country:     "Việt Nam",
}

// hcmAliases are normalized spellings that mean Ho Chi Minh City. Geocoding
// finds nothing for the short forms and a Bình Thạnh ward for "saigon".
var hcmAliases = map[string]bool{
	"hcm":                   true,
	"tphcm":                 true,
	"tp hcm":                true,
	"tp.hcm":                true,
	"sg":                    true,
	"saigon":                true,
	"sai gon":               true,
	"ho chi minh":           true,
	"tp ho chi minh":        true,
	"thanh pho ho chi minh": true,
}

// queryAliases expands common short forms the geocoder does not know.
var queryAliases = map[string]string{
	"hn": "Ha Noi",
	"dn": "Da Nang",
}

// stripMarks removes combining marks after NFD decomposition, which drops
// Vietnamese tone and vowel marks ("Lạt" -> "Lat").
var stripMarks = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// normalizeQuery lowercases a location, removes Vietnamese diacritics, and
// collapses whitespace. Đ is not a decomposable letter, so it is mapped by
// hand, together with the look-alike Ð (U+00D0) that GeoNames uses in names
// such as "Ðà Lạt"; leaving either in place makes the geocoder miss the city.
func normalizeQuery(s string) string {
	s = strings.NewReplacer("Đ", "D", "đ", "d", "Ð", "D", "ð", "d").Replace(s)
	if stripped, _, err := transform.String(stripMarks, s); err == nil {
		s = stripped
	}
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// pickPlace prefers the first Vietnamese match, since most lookups are for
// Vietnamese places and the geocoder may rank a same-named place abroad first,
// and otherwise falls back to the top result.
func pickPlace(results []place) (place, bool) {
	for _, p := range results {
		if p.CountryCode == "VN" {
			return p, true
		}
	}
	if len(results) == 0 {
		return place{}, false
	}
	return results[0], true
}

// displayName labels a place with its province when that adds information,
// and with its country when it is outside Vietnam. The province is skipped
// when it only respells the name, as in "Hà Nội" with admin1 "Hanoi".
func displayName(p place) string {
	parts := []string{p.Name}
	if p.Admin1 != "" && compactName(p.Admin1) != compactName(p.Name) {
		parts = append(parts, p.Admin1)
	}
	if p.CountryCode != "VN" && p.Country != "" {
		parts = append(parts, p.Country)
	}
	return strings.Join(parts, ", ")
}

func compactName(s string) string { return strings.ReplaceAll(normalizeQuery(s), " ", "") }
