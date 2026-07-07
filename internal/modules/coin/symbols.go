package coin

import (
	"errors"
	"strings"
	"unicode"
)

var ErrUnsupportedCoin = errors.New("coin: unsupported coin")

type CoinSymbol struct {
	Symbol      string
	CoinGeckoID string
}

const maxCoinSymbolLength = 20

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
