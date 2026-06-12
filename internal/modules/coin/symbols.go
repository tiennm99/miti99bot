package coin

import (
	"errors"
	"strings"
)

var ErrUnsupportedCoin = errors.New("coin: unsupported coin")

type CoinSymbol struct {
	Symbol      string
	CoinGeckoID string
}

var supportedCoins = map[string]CoinSymbol{
	"BTC":  {Symbol: "BTC", CoinGeckoID: "bitcoin"},
	"ETH":  {Symbol: "ETH", CoinGeckoID: "ethereum"},
	"SOL":  {Symbol: "SOL", CoinGeckoID: "solana"},
	"BNB":  {Symbol: "BNB", CoinGeckoID: "binancecoin"},
	"XRP":  {Symbol: "XRP", CoinGeckoID: "ripple"},
	"ADA":  {Symbol: "ADA", CoinGeckoID: "cardano"},
	"DOGE": {Symbol: "DOGE", CoinGeckoID: "dogecoin"},
	"TON":  {Symbol: "TON", CoinGeckoID: "the-open-network"},
}

func ResolveCoinSymbol(input string) (CoinSymbol, error) {
	symbol := strings.ToUpper(strings.TrimSpace(input))
	if symbol == "" {
		return CoinSymbol{}, ErrUnsupportedCoin
	}
	coin, ok := supportedCoins[symbol]
	if !ok {
		return CoinSymbol{}, ErrUnsupportedCoin
	}
	return coin, nil
}
