package coin

import (
	"errors"
	"strings"
	"unicode"
)

// ErrUnsupportedCoin reports a ticker that fails validation.
var ErrUnsupportedCoin = errors.New("coin: unsupported coin")

// CoinSymbol is a validated upper-case ticker plus its CoinGecko ID when the
// ticker is in knownCoinGeckoIDs.
type CoinSymbol struct {
	Symbol      string
	CoinGeckoID string
}

const maxCoinSymbolLength = 20

// knownCoinGeckoIDs pins major tickers to their CoinGecko IDs, since
// CoinGecko IDs are names ("bitcoin"), not tickers.
var knownCoinGeckoIDs = map[string]string{
	"BTC":  "bitcoin",
	"ETH":  "ethereum",
	"SOL":  "solana",
	"BNB":  "binancecoin",
	"XRP":  "ripple",
	"ADA":  "cardano",
	"DOGE": "dogecoin",
	"TON":  "the-open-network",
}

// ResolveCoinSymbol normalizes input to an upper-case ticker of 1-20 ASCII
// letters and digits with at least one letter. It does not check that the
// coin exists; providers decide that.
func ResolveCoinSymbol(input string) (CoinSymbol, error) {
	symbol := strings.ToUpper(strings.TrimSpace(input))
	if !validCoinSymbol(symbol) {
		return CoinSymbol{}, ErrUnsupportedCoin
	}
	return CoinSymbol{Symbol: symbol, CoinGeckoID: knownCoinGeckoIDs[symbol]}, nil
}

func validCoinSymbol(symbol string) bool {
	if symbol == "" || len(symbol) > maxCoinSymbolLength {
		return false
	}
	hasLetter := false
	for _, r := range symbol {
		if r > unicode.MaxASCII || (!unicode.IsLetter(r) && !unicode.IsDigit(r)) {
			return false
		}
		hasLetter = hasLetter || unicode.IsLetter(r)
	}
	return hasLetter
}
