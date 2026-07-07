package coin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestBinanceProviderFetchUSD(t *testing.T) {
	coin := mustCoin(t, "BTC")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("symbol"); got != "BTCUSDT" {
			t.Fatalf("symbol = %q, want BTCUSDT", got)
		}
		_, _ = w.Write([]byte(`{"symbol":"BTCUSDT","price":"67000.25"}`))
	}))
	defer srv.Close()
	price, err := (&BinanceProvider{URL: srv.URL}).FetchUSD(context.Background(), coin)
	if err != nil {
		t.Fatalf("FetchUSD: %v", err)
	}
	if price.USD != 67000.25 || price.Source != "Binance" || price.Symbol != "BTC" {
		t.Fatalf("price = %+v", price)
	}
}

func TestBinanceProviderFetchesUnlistedTicker(t *testing.T) {
	coin := CoinSymbol{Symbol: "ENA"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("symbol"); got != "ENAUSDT" {
			t.Fatalf("symbol = %q, want ENAUSDT", got)
		}
		_, _ = w.Write([]byte(`{"symbol":"ENAUSDT","price":"0.077"}`))
	}))
	defer srv.Close()
	price, err := (&BinanceProvider{URL: srv.URL}).FetchUSD(context.Background(), coin)
	if err != nil {
		t.Fatalf("FetchUSD: %v", err)
	}
	if price.USD != 0.077 || price.Source != "Binance" || price.Symbol != "ENA" {
		t.Fatalf("price = %+v", price)
	}
}

func TestBinanceRateLimitDoesNotTrySecondPair(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := (&BinanceProvider{URL: srv.URL}).FetchUSD(context.Background(), mustCoin(t, "BTC"))
	if err == nil {
		t.Fatal("want rate-limit error")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", hits.Load())
	}
}

func TestCoinbaseProviderFetchUSD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("currency"); got != "ETH" {
			t.Fatalf("currency = %q, want ETH", got)
		}
		_, _ = w.Write([]byte(`{"data":{"currency":"ETH","rates":{"USD":"3500.5"}}}`))
	}))
	defer srv.Close()
	price, err := (&CoinbaseProvider{URL: srv.URL}).FetchUSD(context.Background(), mustCoin(t, "ETH"))
	if err != nil {
		t.Fatalf("FetchUSD: %v", err)
	}
	if price.USD != 3500.5 || price.Source != "Coinbase" {
		t.Fatalf("price = %+v", price)
	}
}

func TestCoinGeckoProviderFetchUSD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("ids"); got != "solana" {
			t.Fatalf("ids = %q, want solana", got)
		}
		_, _ = w.Write([]byte(`{"solana":{"usd":150.75,"last_updated_at":1711356300}}`))
	}))
	defer srv.Close()
	price, err := (&CoinGeckoProvider{URL: srv.URL}).FetchUSD(context.Background(), mustCoin(t, "SOL"))
	if err != nil {
		t.Fatalf("FetchUSD: %v", err)
	}
	if price.USD != 150.75 || price.Source != "CoinGecko" {
		t.Fatalf("price = %+v", price)
	}
}

func TestCoinGeckoProviderFetchesUnmappedTickerByLowercaseID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("ids"); got != "re" {
			t.Fatalf("ids = %q, want re", got)
		}
		_, _ = w.Write([]byte(`{"re":{"usd":0.64}}`))
	}))
	defer srv.Close()
	price, err := (&CoinGeckoProvider{URL: srv.URL}).FetchUSD(context.Background(), CoinSymbol{Symbol: "RE"})
	if err != nil {
		t.Fatalf("FetchUSD: %v", err)
	}
	if price.USD != 0.64 || price.Source != "CoinGecko" {
		t.Fatalf("price = %+v", price)
	}
}

func TestCoinGeckoProviderSearchesUnmappedTickerWhenIDDiffers(t *testing.T) {
	var simpleIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/simple":
			id := r.URL.Query().Get("ids")
			simpleIDs = append(simpleIDs, id)
			if id == "ethena" {
				_, _ = w.Write([]byte(`{"ethena":{"usd":0.077}}`))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		case "/search":
			if got := r.URL.Query().Get("query"); got != "ENA" {
				t.Fatalf("query = %q, want ENA", got)
			}
			_, _ = w.Write([]byte(`{"coins":[{"id":"ethena-usde","symbol":"USDE","market_cap_rank":27},{"id":"ethena","symbol":"ENA","market_cap_rank":85}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	price, err := (&CoinGeckoProvider{
		URL:       srv.URL + "/simple",
		SearchURL: srv.URL + "/search",
	}).FetchUSD(context.Background(), CoinSymbol{Symbol: "ENA"})
	if err != nil {
		t.Fatalf("FetchUSD: %v", err)
	}
	if price.USD != 0.077 || price.Source != "CoinGecko" {
		t.Fatalf("price = %+v", price)
	}
	if len(simpleIDs) != 2 || simpleIDs[0] != "ena" || simpleIDs[1] != "ethena" {
		t.Fatalf("simple ids = %v, want [ena ethena]", simpleIDs)
	}
}

type fakeProvider struct {
	price CoinPrice
	err   error
	hits  atomic.Int32
}

func (p *fakeProvider) FetchUSD(context.Context, CoinSymbol) (CoinPrice, error) {
	p.hits.Add(1)
	return p.price, p.err
}

func TestPriceClientFallbackAndCache(t *testing.T) {
	first := &fakeProvider{err: ErrNoCoinPrice}
	second := &fakeProvider{price: CoinPrice{USD: 123, Source: "Coinbase"}}
	third := &fakeProvider{price: CoinPrice{USD: 456, Source: "CoinGecko"}}
	now := time.Unix(100, 0)
	client := &PriceClient{
		Providers: []PriceProvider{first, second, third},
		CacheTTL:  time.Minute,
		nowFn:     func() time.Time { return now },
	}
	coin := mustCoin(t, "BTC")
	got, err := client.FetchUSD(context.Background(), coin)
	if err != nil {
		t.Fatalf("FetchUSD: %v", err)
	}
	if got.USD != 123 || got.Source != "Coinbase" {
		t.Fatalf("got %+v", got)
	}
	if _, err := client.FetchUSD(context.Background(), coin); err != nil {
		t.Fatalf("FetchUSD cached: %v", err)
	}
	if first.hits.Load() != 1 || second.hits.Load() != 1 || third.hits.Load() != 0 {
		t.Fatalf("hits first=%d second=%d third=%d", first.hits.Load(), second.hits.Load(), third.hits.Load())
	}
}

func TestPriceClientFallsThroughToCoinGecko(t *testing.T) {
	client := &PriceClient{Providers: []PriceProvider{
		&fakeProvider{err: ErrNoCoinPrice},
		&fakeProvider{err: ErrNoCoinPrice},
		&fakeProvider{price: CoinPrice{USD: 42, Source: "CoinGecko"}},
	}}
	got, err := client.FetchUSD(context.Background(), mustCoin(t, "DOGE"))
	if err != nil {
		t.Fatalf("FetchUSD: %v", err)
	}
	if got.Source != "CoinGecko" || got.USD != 42 {
		t.Fatalf("got %+v", got)
	}
}

func TestPriceClientAllProvidersFail(t *testing.T) {
	client := &PriceClient{Providers: []PriceProvider{&fakeProvider{err: ErrNoCoinPrice}}}
	_, err := client.FetchUSD(context.Background(), mustCoin(t, "BTC"))
	if !errors.Is(err, ErrNoCoinPrice) {
		t.Fatalf("got %v, want ErrNoCoinPrice", err)
	}
}

func TestValidateEndpoint(t *testing.T) {
	if err := validateEndpoint("https://example.com/path"); err != nil {
		t.Fatalf("https should pass: %v", err)
	}
	if err := validateEndpoint("http://localhost:1234/path"); err != nil {
		t.Fatalf("localhost http should pass: %v", err)
	}
	if err := validateEndpoint("http://example.com/path"); err == nil {
		t.Fatal("remote http should fail")
	}
}

func mustCoin(t *testing.T, symbol string) CoinSymbol {
	t.Helper()
	coin, err := ResolveCoinSymbol(symbol)
	if err != nil {
		t.Fatalf("ResolveCoinSymbol(%q): %v", symbol, err)
	}
	return coin
}
