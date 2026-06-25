package stock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// vciDefaultURL is Vietcap's public current quote endpoint.
const vciDefaultURL = "https://trading.vietcap.com.vn/api/price/symbols/getList"

type vciRequest struct {
	Symbols []string `json:"symbols"`
}

type vciQuote struct {
	ListingInfo struct {
		Symbol string `json:"symbol"`
	} `json:"listingInfo"`
	MatchPrice struct {
		Price float64 `json:"matchPrice"`
	} `json:"matchPrice"`
}

func (c *PriceClient) vciFallbackEnabled() bool {
	return strings.TrimSpace(c.VCIURL) != "" || !c.ssiFirst()
}

func (c *PriceClient) fetchVCIPrice(ctx context.Context, ticker string) (float64, error) {
	prices, err := c.fetchVCIPrices(ctx, []string{ticker})
	if err != nil {
		return 0, err
	}
	price := prices[strings.ToUpper(strings.TrimSpace(ticker))]
	if price <= 0 {
		return 0, fmt.Errorf("%w: VCI returned no usable quote for %s", ErrNoPrice, strings.ToUpper(ticker))
	}
	return price, nil
}

func (c *PriceClient) fetchVCIPrices(ctx context.Context, tickers []string) (map[string]float64, error) {
	requested := normalizeTickers(tickers)
	if len(requested) == 0 {
		return map[string]float64{}, nil
	}

	payload, err := json.Marshal(vciRequest{Symbols: requested})
	if err != nil {
		return nil, fmt.Errorf("stock: build VCI payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.vciURL(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("stock: build VCI request: %w", err)
	}
	setVCIHeaders(req)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("stock: VCI request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: VCI status %d", ErrNoPrice, resp.StatusCode)
	}

	var body []vciQuote
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("stock: VCI decode: %w", err)
	}
	out := make(map[string]float64, len(body))
	for _, quote := range body {
		symbol := strings.ToUpper(strings.TrimSpace(quote.ListingInfo.Symbol))
		if symbol == "" || quote.MatchPrice.Price <= 0 {
			continue
		}
		out[symbol] = quote.MatchPrice.Price
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: VCI returned no usable quotes for %s", ErrNoPrice, strings.Join(requested, ","))
	}
	return out, nil
}

func (c *PriceClient) vciURL() string {
	if strings.TrimSpace(c.VCIURL) != "" {
		return strings.TrimRight(c.VCIURL, "/")
	}
	return vciDefaultURL
}

func setVCIHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,vi-VN;q=0.8,vi;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DNT", "1")
	req.Header.Set("Origin", "https://trading.vietcap.com.vn")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "https://trading.vietcap.com.vn/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("User-Agent", stockBrowserUserAgent)
}
