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
	Source      string    // "vnappmob-sjc" or "xau-fallback"
	SJC         *SJCPrice // non-nil when Source == "vnappmob-sjc"
}

// SJCPrice holds VNAppMob SJC buy/sell quotes per lượng (VND).
type SJCPrice struct {
	Buy  float64
	Sell float64
}

// GoldPriceClient fetches XAU/USD through a chain of free providers (see
// price_providers.go) and converts to VND via a cached USD FX-rate table.
type GoldPriceClient struct {
	HTTP *http.Client

	// Per-provider URL overrides; empty means the provider default.
	GoldURL       string // primary: gold-api.com
	SwissquoteURL string
	NBPURL        string
	FXURL         string

	defaultOnce   sync.Once
	defaultClient *http.Client
	nowFn         func() time.Time

	mu       sync.Mutex
	fxRates  map[string]float64
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
	usdToVND, err := c.fetchFXRate(ctx, "VND")
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

// FetchLuongPrices returns the same representative spot price for both buy and
// sell because the XAU/USD fallback has no bid/ask spread.
func (c *GoldPriceClient) FetchLuongPrices(ctx context.Context) (float64, float64, error) {
	p, err := c.FetchLuongPrice(ctx)
	return p, p, err
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

func (c *GoldPriceClient) fxURL() string {
	if strings.TrimSpace(c.FXURL) != "" {
		return strings.TrimSpace(c.FXURL)
	}
	return fxDefaultURL
}

type fxResponse struct {
	Result             string             `json:"result"`
	Rates              map[string]float64 `json:"rates"`
	TimeNextUpdateUnix int64              `json:"time_next_update_unix"`
}

// fetchFXRate returns the USD→code rate from a cached full rate table so one
// FX call serves both the VND conversion and the NBP fallback (PLN).
func (c *GoldPriceClient) fetchFXRate(ctx context.Context, code string) (float64, error) {
	c.mu.Lock()
	now := c.now()
	if rate := c.fxRates[code]; rate > 0 && now.Before(c.fxExpiry) {
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
		return 0, fmt.Errorf("gold: FX status %d: %w", resp.StatusCode, ErrNoGoldPrice)
	}
	var body fxResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("gold: FX decode: %w", err)
	}
	if body.Result != "" && body.Result != "success" {
		return 0, ErrNoGoldPrice
	}
	rate := body.Rates[code]
	if rate <= 0 {
		return 0, ErrNoGoldPrice
	}
	expiry := now.Add(fxFallbackCacheTTL)
	if body.TimeNextUpdateUnix > now.Unix() {
		expiry = time.Unix(body.TimeNextUpdateUnix, 0)
	}
	c.mu.Lock()
	c.fxRates = body.Rates
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
