package gold

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tiennm99/miti99bot/internal/log"
)

// XAU/USD provider defaults. All three are free, keyless, and verified to
// answer datacenter IPs (goldprice.org was dropped: it 403s cloud/datacenter IPs).
const (
	goldAPIDefaultURL    = "https://api.gold-api.com/price/XAU"
	swissquoteDefaultURL = "https://forex-data-feed.swissquote.com/public-quotes/bboquotes/instrument/XAU/USD"
	nbpDefaultURL        = "https://api.nbp.pl/api/cenyzlota?format=json"
)

type xauProvider struct {
	name  string
	url   string
	fetch func(ctx context.Context, url string) (float64, error)
}

func (c *GoldPriceClient) providers() []xauProvider {
	pick := func(override, def string) string {
		if s := strings.TrimSpace(override); s != "" {
			return s
		}
		return def
	}
	return []xauProvider{
		{name: "gold-api.com", url: pick(c.GoldURL, goldAPIDefaultURL), fetch: c.fetchGoldAPI},
		{name: "swissquote", url: pick(c.SwissquoteURL, swissquoteDefaultURL), fetch: c.fetchSwissquote},
		{name: "nbp", url: pick(c.NBPURL, nbpDefaultURL), fetch: c.fetchNBP},
	}
}

// fetchXAUUSD walks the provider chain and returns the first USD/oz price.
// Per-provider failures are logged; if every provider fails the joined error
// deliberately does NOT wrap ErrNoGoldPrice so callers treat it as a
// retryable fetch failure, not an empty-data reply.
func (c *GoldPriceClient) fetchXAUUSD(ctx context.Context) (float64, error) {
	var failures []string
	for _, p := range c.providers() {
		price, err := p.fetch(ctx, p.url)
		if err == nil {
			return price, nil
		}
		log.Warn("gold_price_provider_failed", "provider", p.name, "err", err)
		failures = append(failures, fmt.Sprintf("%s: %v", p.name, err))
	}
	return 0, fmt.Errorf("gold: all price providers failed: %s", strings.Join(failures, "; "))
}

func (c *GoldPriceClient) providerGet(ctx context.Context, url string, dst any) error {
	if err := validateEndpoint(url); err != nil {
		return err
	}
	resp, err := c.getJSON(ctx, url)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

// fetchGoldAPI parses gold-api.com: {"price": 4073.6, "currency": "USD"}.
func (c *GoldPriceClient) fetchGoldAPI(ctx context.Context, url string) (float64, error) {
	var body struct {
		Currency string  `json:"currency"`
		Price    float64 `json:"price"`
	}
	if err := c.providerGet(ctx, url, &body); err != nil {
		return 0, err
	}
	if body.Currency != "" && body.Currency != "USD" {
		return 0, fmt.Errorf("unexpected currency %q", body.Currency)
	}
	if body.Price <= 0 {
		return 0, ErrNoGoldPrice
	}
	return body.Price, nil
}

// fetchSwissquote parses the public best-bid/offer feed:
// [{"spreadProfilePrices":[{"bid":4074.41,"ask":4075.10}, ...]}, ...]
// and returns the mid price of the first quoted profile.
func (c *GoldPriceClient) fetchSwissquote(ctx context.Context, url string) (float64, error) {
	var body []struct {
		SpreadProfilePrices []struct {
			Bid float64 `json:"bid"`
			Ask float64 `json:"ask"`
		} `json:"spreadProfilePrices"`
	}
	if err := c.providerGet(ctx, url, &body); err != nil {
		return 0, err
	}
	for _, platform := range body {
		for _, q := range platform.SpreadProfilePrices {
			if q.Bid > 0 && q.Ask > 0 {
				return (q.Bid + q.Ask) / 2, nil
			}
		}
	}
	return 0, ErrNoGoldPrice
}

// fetchNBP parses the Polish central bank daily fixing
// [{"data":"2026-06-11","cena":492.71}] (PLN per gram) and converts to USD/oz
// using the shared FX table. Daily granularity — last-resort fallback only.
func (c *GoldPriceClient) fetchNBP(ctx context.Context, url string) (float64, error) {
	var body []struct {
		PLNPerGram float64 `json:"cena"`
	}
	if err := c.providerGet(ctx, url, &body); err != nil {
		return 0, err
	}
	if len(body) == 0 || body[0].PLNPerGram <= 0 {
		return 0, ErrNoGoldPrice
	}
	plnPerUSD, err := c.fetchFXRate(ctx, "PLN")
	if err != nil {
		return 0, fmt.Errorf("PLN rate: %w", err)
	}
	return body[0].PLNPerGram * gramsPerTroyOunce / plnPerUSD, nil
}
