package stock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ssiQueryDefaultURL is the SSI iBoard stock-query endpoint base.
const ssiQueryDefaultURL = "https://iboard-query.ssi.com.vn"

// ssiHTTPTimeout caps a stock quote request. Kept under the handler deadline
// so a slow upstream cannot starve the Telegram reply budget.
const ssiHTTPTimeout = 3 * time.Second

// ssiErrorBodyLimit keeps upstream diagnostics useful without dumping large responses.
const ssiErrorBodyLimit = 512

// PriceClient is the SSI iBoard stock quote fetcher. Zero value uses the
// default SSI URL + a timeout-bound HTTP client; tests inject HTTP + URL.
type PriceClient struct {
	HTTP *http.Client
	URL  string

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
		c.defaultClient = &http.Client{Timeout: ssiHTTPTimeout}
	})
	return c.defaultClient
}

func (c *PriceClient) baseURL() string {
	if c.URL != "" {
		return strings.TrimRight(c.URL, "/")
	}
	return ssiQueryDefaultURL
}

type ssiSingleResponse struct {
	Data ssiStockQuote `json:"data"`
}

type ssiMultipleResponse struct {
	Data []ssiStockQuote `json:"data"`
}

type ssiStockQuote struct {
	StockSymbol  string  `json:"stockSymbol"`
	MatchedPrice float64 `json:"matchedPrice"`
}

// FetchPrice returns SSI's current matched price in VND for ticker, or
// ErrNoPrice if SSI returns no usable quote. Network / decode errors are
// returned wrapped.
func (c *PriceClient) FetchPrice(ctx context.Context, ticker string) (float64, error) {
	if ticker == "" {
		return 0, errors.New("stock: ticker is empty")
	}
	full := c.baseURL() + "/stock/" + url.PathEscape(ticker)
	req, err := newSSIRequest(ctx, http.MethodGet, full, nil)
	if err != nil {
		return 0, err
	}

	var body ssiSingleResponse
	if err := c.doJSON(req, &body); err != nil {
		return 0, err
	}
	price := body.Data.MatchedPrice
	if price <= 0 {
		return 0, fmt.Errorf("%w: SSI quote has no matchedPrice for %s", ErrNoPrice, strings.ToUpper(ticker))
	}
	return price, nil
}

// FetchPrices returns current matched prices for the requested tickers using
// SSI's batch quote endpoint. Missing or invalid quotes are omitted from the
// returned map; callers can degrade those symbols individually.
func (c *PriceClient) FetchPrices(ctx context.Context, tickers []string) (map[string]float64, error) {
	if len(tickers) == 0 {
		return map[string]float64{}, nil
	}
	form := url.Values{}
	requested := make([]string, 0, len(tickers))
	for _, ticker := range tickers {
		ticker = strings.TrimSpace(ticker)
		if ticker == "" {
			continue
		}
		ticker = strings.ToUpper(ticker)
		form.Add("stocks", ticker)
		requested = append(requested, ticker)
	}
	if len(form) == 0 {
		return nil, errors.New("stock: no tickers to fetch")
	}

	req, err := newSSIRequest(ctx, http.MethodPost, c.baseURL()+"/stock/multiple", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var body ssiMultipleResponse
	if err := c.doJSON(req, &body); err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(body.Data))
	for _, quote := range body.Data {
		symbol := strings.ToUpper(strings.TrimSpace(quote.StockSymbol))
		if symbol == "" || quote.MatchedPrice <= 0 {
			continue
		}
		out[symbol] = quote.MatchedPrice
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: SSI batch returned no usable quotes for %s (data_len=%d)", ErrNoPrice, strings.Join(requested, ","), len(body.Data))
	}
	return out, nil
}

func newSSIRequest(ctx context.Context, method, full string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return nil, fmt.Errorf("stock: build SSI request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://iboard.ssi.com.vn")
	req.Header.Set("Referer", "https://iboard.ssi.com.vn/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (miti99bot)")
	return req, nil
}

func (c *PriceClient) doJSON(req *http.Request, dst any) error {
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("stock: SSI request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, ssiErrorBodyLimit))
		return fmt.Errorf("%w: SSI status %d body %q", ErrNoPrice, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("stock: SSI decode: %w", err)
	}
	return nil
}

// ErrNoPrice means SSI returned no usable price for the ticker - either the
// symbol is unknown, the market hasn't traded recently, or the data was
// invalid. Used by symbol resolution to detect "is this a real ticker".
var ErrNoPrice = errors.New("stock: no price available")
