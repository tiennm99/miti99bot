package gold

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	goldDefaultURL     = "https://data-asg.goldprice.org/dbXRates/USD"
	fxDefaultURL       = "https://open.er-api.com/v6/latest/USD"
	goldHTTPTimeout    = 10 * time.Second
	fxFallbackCacheTTL = time.Hour
	gramsPerLuong      = 37.5
	gramsPerTroyOunce  = 31.1034768
)

var ErrNoGoldPrice = errors.New("gold: no price available")

type GoldPrice struct {
	XAUUSD      float64
	USDVND      float64
	VNDPerLuong float64
}

type GoldPriceClient struct {
	HTTP    *http.Client
	GoldURL string
	FXURL   string

	defaultOnce   sync.Once
	defaultClient *http.Client
	nowFn         func() time.Time

	mu       sync.Mutex
	fxRate   float64
	fxExpiry time.Time
}

func NewGoldPriceClientFromEnv() *GoldPriceClient {
	return &GoldPriceClient{
		GoldURL: strings.TrimSpace(os.Getenv("GOLD_PRICE_API_URL")),
		FXURL:   strings.TrimSpace(os.Getenv("GOLD_FX_API_URL")),
	}
}

func (c *GoldPriceClient) FetchPrice(ctx context.Context) (GoldPrice, error) {
	xauUSD, err := c.fetchXAUUSD(ctx)
	if err != nil {
		return GoldPrice{}, err
	}
	usdToVND, err := c.fetchUSDVND(ctx)
	if err != nil {
		return GoldPrice{}, err
	}
	vndPerLuong := xauUSD * usdToVND * (gramsPerLuong / gramsPerTroyOunce)
	if vndPerLuong <= 0 || math.IsNaN(vndPerLuong) || math.IsInf(vndPerLuong, 0) {
		return GoldPrice{}, ErrNoGoldPrice
	}
	return GoldPrice{XAUUSD: xauUSD, USDVND: usdToVND, VNDPerLuong: vndPerLuong}, nil
}

func (c *GoldPriceClient) FetchLuongPrice(ctx context.Context) (float64, error) {
	p, err := c.FetchPrice(ctx)
	if err != nil {
		return 0, err
	}
	return p.VNDPerLuong, nil
}

func (c *GoldPriceClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	c.defaultOnce.Do(func() {
		c.defaultClient = &http.Client{Timeout: goldHTTPTimeout}
	})
	return c.defaultClient
}

func (c *GoldPriceClient) now() time.Time {
	if c.nowFn != nil {
		return c.nowFn()
	}
	return time.Now()
}

func (c *GoldPriceClient) goldURL() string {
	if strings.TrimSpace(c.GoldURL) != "" {
		return strings.TrimSpace(c.GoldURL)
	}
	return goldDefaultURL
}

func (c *GoldPriceClient) fxURL() string {
	if strings.TrimSpace(c.FXURL) != "" {
		return strings.TrimSpace(c.FXURL)
	}
	return fxDefaultURL
}

type goldResponse struct {
	Items []goldItem `json:"items"`
}

type goldItem struct {
	Currency string  `json:"curr"`
	XAUPrice float64 `json:"xauPrice"`
}

type fxResponse struct {
	Result             string             `json:"result"`
	Rates              map[string]float64 `json:"rates"`
	TimeNextUpdateUnix int64              `json:"time_next_update_unix"`
}

func (c *GoldPriceClient) fetchXAUUSD(ctx context.Context) (float64, error) {
	endpoint := c.goldURL()
	if err := validateEndpoint(endpoint); err != nil {
		return 0, err
	}
	resp, err := c.getJSON(ctx, endpoint)
	if err != nil {
		return 0, fmt.Errorf("gold: GoldPrice request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, ErrNoGoldPrice
	}
	var body goldResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("gold: GoldPrice decode: %w", err)
	}
	if len(body.Items) == 0 {
		return 0, ErrNoGoldPrice
	}
	item := body.Items[0]
	if item.Currency != "" && item.Currency != "USD" {
		return 0, ErrNoGoldPrice
	}
	if item.XAUPrice <= 0 {
		return 0, ErrNoGoldPrice
	}
	return item.XAUPrice, nil
}

func (c *GoldPriceClient) fetchUSDVND(ctx context.Context) (float64, error) {
	c.mu.Lock()
	now := c.now()
	if c.fxRate > 0 && now.Before(c.fxExpiry) {
		rate := c.fxRate
		c.mu.Unlock()
		return rate, nil
	}
	c.mu.Unlock()

	endpoint := c.fxURL()
	if err := validateEndpoint(endpoint); err != nil {
		return 0, err
	}
	resp, err := c.getJSON(ctx, endpoint)
	if err != nil {
		return 0, fmt.Errorf("gold: FX request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests {
		return 0, errors.New("gold: FX rate limited")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, ErrNoGoldPrice
	}
	var body fxResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("gold: FX decode: %w", err)
	}
	if body.Result != "" && body.Result != "success" {
		return 0, ErrNoGoldPrice
	}
	rate := body.Rates["VND"]
	if rate <= 0 {
		return 0, ErrNoGoldPrice
	}
	expiry := now.Add(fxFallbackCacheTTL)
	if body.TimeNextUpdateUnix > now.Unix() {
		expiry = time.Unix(body.TimeNextUpdateUnix, 0)
	}
	c.mu.Lock()
	c.fxRate = rate
	c.fxExpiry = expiry
	c.mu.Unlock()
	return rate, nil
}

func (c *GoldPriceClient) getJSON(ctx context.Context, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (miti99bot)")
	return c.httpClient().Do(req)
}
