package stock

import (
	"errors"
	"regexp"
	"strings"
)

// tickerRe restricts stock tickers to ASCII alphanumeric, 1-16 chars. This
// keeps provider lookups predictable and rejects unicode-lookalike inputs.
var tickerRe = regexp.MustCompile(`^[A-Z0-9]{1,16}$`)

// ErrUnknownTicker means the user input is not a valid stock ticker shape.
var ErrUnknownTicker = errors.New("stock: unknown ticker")

// normalizeStockSymbol trims and upper-cases ticker and checks it against
// tickerRe. Empty input also returns ErrUnknownTicker, so callers need only one
// error path for both empty and malformed tickers.
func normalizeStockSymbol(ticker string) (string, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if !tickerRe.MatchString(ticker) {
		return "", ErrUnknownTicker
	}
	return ticker, nil
}
