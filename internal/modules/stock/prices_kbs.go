package stock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// kbsDefaultURL is KBS's public current price-board endpoint.
const kbsDefaultURL = "https://kbbuddywts.kbsec.com.vn/iis-server/investment/stock/iss"

type kbsRequest struct {
	Code string `json:"code"`
}

type kbsQuote struct {
	Symbol string  `json:"SB"`
	Price  float64 `json:"CP"`
}

func (c *PriceClient) kbsFallbackEnabled() bool {
	return strings.TrimSpace(c.KBSURL) != "" || !c.ssiFirst()
}

func (c *PriceClient) fetchKBSPrice(ctx context.Context, ticker string) (float64, error) {
	prices, err := c.fetchKBSPrices(ctx, []string{ticker})
	if err != nil {
		return 0, err
	}
	price := prices[strings.ToUpper(strings.TrimSpace(ticker))]
	if price <= 0 {
		return 0, fmt.Errorf("%w: KBS returned no usable quote for %s", ErrNoPrice, strings.ToUpper(ticker))
	}
	return price, nil
}

func (c *PriceClient) fetchKBSPrices(ctx context.Context, tickers []string) (map[string]float64, error) {
	requested := normalizeTickers(tickers)
	if len(requested) == 0 {
		return map[string]float64{}, nil
	}

	payload, err := json.Marshal(kbsRequest{Code: strings.Join(requested, ",")})
	if err != nil {
		return nil, fmt.Errorf("stock: build KBS payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.kbsURL(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("stock: build KBS request: %w", err)
	}
	setKBSHeaders(req)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("stock: KBS request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: KBS status %d", ErrNoPrice, resp.StatusCode)
	}

	var body []kbsQuote
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("stock: KBS decode: %w", err)
	}
	out := make(map[string]float64, len(body))
	for _, quote := range body {
		symbol := strings.ToUpper(strings.TrimSpace(quote.Symbol))
		if symbol == "" || quote.Price <= 0 {
			continue
		}
		out[symbol] = quote.Price
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: KBS returned no usable quotes for %s", ErrNoPrice, strings.Join(requested, ","))
	}
	return out, nil
}

func (c *PriceClient) kbsURL() string {
	if strings.TrimSpace(c.KBSURL) != "" {
		return strings.TrimRight(c.KBSURL, "/")
	}
	return kbsDefaultURL
}

func setKBSHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,vi;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DNT", "1")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", stockBrowserUserAgent)
	req.Header.Set("x-lang", "vi")
}
