package monkeyd

import (
	"errors"
	"net/url"
	"strings"
)

// allowedHosts are the hostnames the crawler's extractor understands. It is
// written against monkeydd.com's specific markup — the chapter list, the
// in-chapter dropdown, and the CSS rules that supply part of the chapter text —
// so another host would parse to nothing useful. Refusing it up front also
// keeps the command from being used to make the bot fetch arbitrary URLs.
var allowedHosts = map[string]bool{
	"monkeydd.com":     true,
	"www.monkeydd.com": true,
}

// AllowedHostsHint lists the accepted hosts for user-facing usage text.
const AllowedHostsHint = "monkeydd.com"

var (
	errNotAURL      = errors.New("that does not look like a URL")
	errHostNotAllow = errors.New("only " + AllowedHostsHint + " novel URLs are supported")
	errNoPath       = errors.New("that URL has no novel path — link the novel's own page")
)

// normalizeNovelURL validates a user-supplied novel URL and returns the form to
// crawl. A missing scheme is filled in with https, since people paste bare
// hostnames; anything else that is not a plain http(s) URL on an allowed host
// is rejected.
func normalizeNovelURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errNotAURL
	}
	// A bare "monkeydd.com/x.html" parses as a path with no host, so give it a
	// scheme before parsing rather than trying to interpret the result.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errNotAURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errNotAURL
	}
	// Credentials in the URL are never needed here and would be logged with the
	// crawl, so treat them as malformed input.
	if parsed.User != nil {
		return "", errNotAURL
	}
	// Hostname() drops any port, which the allowlist must not be fooled by.
	if !allowedHosts[strings.ToLower(parsed.Hostname())] {
		return "", errHostNotAllow
	}
	if strings.Trim(parsed.Path, "/") == "" {
		return "", errNoPath
	}
	// Rebuild from the parsed parts so the crawl uses a canonical host and no
	// fragment; query strings are kept because the site may need them.
	canonical := url.URL{
		Scheme:   parsed.Scheme,
		Host:     strings.ToLower(parsed.Host),
		Path:     parsed.Path,
		RawQuery: parsed.RawQuery,
	}
	return canonical.String(), nil
}
