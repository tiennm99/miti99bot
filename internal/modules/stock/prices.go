package stock

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// stockPriceHTTPTimeout caps each quote request made by the default HTTP
// client. FetchPrice and FetchPrices try providers in sequence, so the bound is
// per provider attempt; a slow upstream fails over instead of stalling the reply.
const stockPriceHTTPTimeout = 3 * time.Second

const stockBrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36"

// PriceClient fetches VN stock quotes. The zero value tries KBS current
// price-board quotes first, then VCI current quotes, then SSI direct quotes.
// Setting URL moves SSI to the front and keeps KBS or VCI as a fallback only
// when its own URL is also set.
type PriceClient struct {
	HTTP   *http.Client
	URL    string // SSI direct quote endpoint base override; see the ordering above.
	KBSURL string // KBS current quote endpoint override.
	VCIURL string // VCI current quote endpoint override.

	// defaultClient memoises the zero-value HTTP client so the transport's
	// connection pool survives across stock commands.
	defaultOnce   sync.Once
	defaultClient *http.Client
}

func (c *PriceClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	c.defaultOnce.Do(func() {
		c.defaultClient = &http.Client{Timeout: stockPriceHTTPTimeout}
	})
	return c.defaultClient
}

// FetchPrice returns the current VND price for ticker from the first provider
// that yields a usable quote. The error wraps ErrNoPrice only when every
// attempted provider reported no price; transport or decode failures do not.
func (c *PriceClient) FetchPrice(ctx context.Context, ticker string) (float64, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if ticker == "" {
		return 0, errors.New("stock: ticker is empty")
	}

	var errs []providerError
	if c.ssiFirst() {
		price, err := c.fetchSSIPrice(ctx, ticker)
		if err == nil {
			return price, nil
		}
		errs = append(errs, providerError{name: "SSI direct", err: err})
	}
	if c.kbsFallbackEnabled() {
		price, err := c.fetchKBSPrice(ctx, ticker)
		if err == nil {
			return price, nil
		}
		errs = append(errs, providerError{name: "KBS", err: err})
	}
	if c.vciFallbackEnabled() {
		price, err := c.fetchVCIPrice(ctx, ticker)
		if err == nil {
			return price, nil
		}
		errs = append(errs, providerError{name: "VCI", err: err})
	}
	if !c.ssiFirst() {
		price, err := c.fetchSSIPrice(ctx, ticker)
		if err == nil {
			return price, nil
		}
		errs = append(errs, providerError{name: "SSI direct", err: err})
	}
	return 0, combineProviderErrors(ticker, errs...)
}

// FetchPrices returns current prices for the requested tickers from the first
// provider that yields at least one usable quote. Missing or invalid quotes are
// omitted from the returned map rather than retried on another provider;
// callers degrade those symbols individually.
func (c *PriceClient) FetchPrices(ctx context.Context, tickers []string) (map[string]float64, error) {
	requested := normalizeTickers(tickers)
	if len(requested) == 0 {
		return map[string]float64{}, nil
	}

	var errs []providerError
	if c.ssiFirst() {
		prices, err := c.fetchSSIPrices(ctx, requested)
		if err == nil {
			return prices, nil
		}
		errs = append(errs, providerError{name: "SSI direct", err: err})
	}
	if c.kbsFallbackEnabled() {
		prices, err := c.fetchKBSPrices(ctx, requested)
		if len(prices) > 0 {
			return prices, nil
		}
		if err == nil {
			err = fmt.Errorf("%w: KBS fallback returned no usable quotes", ErrNoPrice)
		}
		errs = append(errs, providerError{name: "KBS", err: err})
	}
	if c.vciFallbackEnabled() {
		prices, err := c.fetchVCIPrices(ctx, requested)
		if len(prices) > 0 {
			return prices, nil
		}
		if err == nil {
			err = fmt.Errorf("%w: VCI fallback returned no usable quotes", ErrNoPrice)
		}
		errs = append(errs, providerError{name: "VCI", err: err})
	}
	if !c.ssiFirst() {
		prices, err := c.fetchSSIPrices(ctx, requested)
		if err == nil {
			return prices, nil
		}
		errs = append(errs, providerError{name: "SSI direct", err: err})
	}
	return nil, combineProviderErrors(strings.Join(requested, ","), errs...)
}

// ssiFirst reports whether an explicit SSI URL puts SSI ahead of KBS and VCI.
func (c *PriceClient) ssiFirst() bool {
	return strings.TrimSpace(c.URL) != ""
}

func normalizeTickers(tickers []string) []string {
	out := make([]string, 0, len(tickers))
	for _, ticker := range tickers {
		ticker = strings.ToUpper(strings.TrimSpace(ticker))
		if ticker != "" {
			out = append(out, ticker)
		}
	}
	return out
}

// ErrNoPrice means no provider returned a usable price for the ticker.
var ErrNoPrice = errors.New("stock: no price available")

// providerError records one failed provider attempt for combineProviderErrors.
type providerError struct {
	name string
	err  error
}

// combineProviderErrors joins every attempt into one message, wrapping
// ErrNoPrice only when all attempts were no-price outcomes.
func combineProviderErrors(ticker string, errs ...providerError) error {
	allNoPrice := true
	parts := make([]string, 0, len(errs))
	for _, entry := range errs {
		if entry.err == nil {
			continue
		}
		if !errors.Is(entry.err, ErrNoPrice) {
			allNoPrice = false
		}
		parts = append(parts, entry.name+" failed ("+entry.err.Error()+")")
	}
	msg := ticker + ": " + strings.Join(parts, "; ")
	if allNoPrice {
		return fmt.Errorf("%w: %s", ErrNoPrice, msg)
	}
	return errors.New("stock: price providers failed for " + msg)
}
