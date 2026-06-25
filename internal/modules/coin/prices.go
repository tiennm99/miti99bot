package coin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	binanceDefaultURL   = "https://data-api.binance.vision/api/v3/ticker/price"
	coinbaseDefaultURL  = "https://api.coinbase.com/v2/exchange-rates"
	coinGeckoDefaultURL = "https://api.coingecko.com/api/v3/simple/price"
	// coinHTTPTimeout caps a single provider call, kept under the handler
	// deadline so one slow provider cannot starve the Telegram reply budget
	// (see chathelper.FetchContext).
	coinHTTPTimeout   = 3 * time.Second
	coinPriceCacheTTL = 30 * time.Second
)

var ErrNoCoinPrice = errors.New("coin: no price available")

type CoinPrice struct {
	Symbol string
	USD    float64
	Source string
}

type PriceProvider interface {
	FetchUSD(ctx context.Context, coin CoinSymbol) (CoinPrice, error)
}

type PriceClient struct {
	Providers []PriceProvider
	CacheTTL  time.Duration
	nowFn     func() time.Time

	mu    sync.Mutex
	cache map[string]cachedPrice
}

type cachedPrice struct {
	price  CoinPrice
	expiry time.Time
}

func NewPriceClientFromEnv() *PriceClient {
	httpClient := &http.Client{Timeout: coinHTTPTimeout}
	return &PriceClient{
		Providers: []PriceProvider{
			&BinanceProvider{HTTP: httpClient, URL: os.Getenv("COIN_BINANCE_API_URL")},
			&CoinbaseProvider{HTTP: httpClient, URL: os.Getenv("COIN_COINBASE_API_URL")},
			&CoinGeckoProvider{HTTP: httpClient, URL: os.Getenv("COIN_COINGECKO_API_URL")},
		},
		CacheTTL: coinPriceCacheTTL,
	}
}

func (c *PriceClient) FetchUSD(ctx context.Context, coin CoinSymbol) (CoinPrice, error) {
	if coin.Symbol == "" {
		return CoinPrice{}, ErrUnsupportedCoin
	}
	if price, ok := c.cached(coin.Symbol); ok {
		return price, nil
	}
	var errs []error
	for _, provider := range c.Providers {
		if provider == nil {
			continue
		}
		price, err := provider.FetchUSD(ctx, coin)
		if err == nil && price.USD > 0 {
			price.Symbol = coin.Symbol
			c.store(coin.Symbol, price)
			return price, nil
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return CoinPrice{}, ErrNoCoinPrice
	}
	return CoinPrice{}, fmt.Errorf("coin: all price providers failed: %w", ErrNoCoinPrice)
}

func (c *PriceClient) cached(symbol string) (CoinPrice, bool) {
	ttl := c.CacheTTL
	if ttl <= 0 {
		return CoinPrice{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		return CoinPrice{}, false
	}
	entry, ok := c.cache[symbol]
	if !ok || !c.now().Before(entry.expiry) {
		return CoinPrice{}, false
	}
	return entry.price, true
}

func (c *PriceClient) store(symbol string, price CoinPrice) {
	ttl := c.CacheTTL
	if ttl <= 0 || price.USD <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		c.cache = map[string]cachedPrice{}
	}
	c.cache[symbol] = cachedPrice{price: price, expiry: c.now().Add(ttl)}
}

func (c *PriceClient) now() time.Time {
	if c.nowFn != nil {
		return c.nowFn()
	}
	return time.Now()
}
