package gold

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/storage"
)

const (
	vnappmobDefaultURL    = "https://api.vnappmob.com"
	vnappmobKeyCacheKey   = "vnappmob:api_key"
	vnappmobRefreshBuffer = 24 * time.Hour
	vnappmobHTTPTimeout   = 3 * time.Second // kept under the handler deadline; see chathelper.FetchContext
)

// VNAppMobClient fetches Vietnam SJC gold prices from api.vnappmob.com.
// It self-manages a free JWT API key, caching it in KV and refreshing it
// before expiry or when the SJC endpoint returns 403.
type VNAppMobClient struct {
	HTTP    *http.Client
	BaseURL string          // optional override; default https://api.vnappmob.com
	Token   string          // optional env override (GOLD_VNAPP_API_KEY)
	KV      storage.KVStore // module-scoped KV store

	nowFn func() time.Time
	mu    sync.Mutex
}

// NewVNAppMobClientFromEnv creates a client reading GOLD_VNAPP_API_URL and
// GOLD_VNAPP_API_KEY from the environment. The API key env var is intended
// for local dev or SSM injection; when empty the client refreshes the key
// automatically via the VNAppMob refresh endpoint.
func NewVNAppMobClientFromEnv(kv storage.KVStore) *VNAppMobClient {
	return &VNAppMobClient{
		BaseURL: strings.TrimSpace(os.Getenv("GOLD_VNAPP_API_URL")),
		Token:   strings.TrimSpace(os.Getenv("GOLD_VNAPP_API_KEY")),
		KV:      kv,
	}
}

func (c *VNAppMobClient) now() time.Time {
	if c.nowFn != nil {
		return c.nowFn()
	}
	return time.Now()
}

func (c *VNAppMobClient) baseURL() string {
	if s := strings.TrimSpace(c.BaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	return vnappmobDefaultURL
}

func (c *VNAppMobClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: vnappmobHTTPTimeout}
}

// FetchSJCPrice returns the VNAppMob SJC buy/sell price per lượng in VND.
// On 403 it refreshes the API key once and retries.
func (c *VNAppMobClient) FetchSJCPrice(ctx context.Context) (buy, sell float64, err error) {
	key, err := c.getKey(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("vnappmob: get key: %w", err)
	}

	buy, sell, err = c.fetchSJC(ctx, key)
	if err == nil {
		return buy, sell, nil
	}
	var statusErr *httpStatusError
	if !errors.As(err, &statusErr) || (statusErr.StatusCode != http.StatusForbidden && statusErr.StatusCode != http.StatusUnauthorized) {
		return 0, 0, err
	}

	log.Warn("vnappmob_sjc_auth_failed", "status", statusErr.StatusCode, "msg", "refreshing key after auth failure")
	if refreshErr := c.refreshKey(ctx); refreshErr != nil {
		return 0, 0, fmt.Errorf("vnappmob: 403 refresh failed: %w", refreshErr)
	}
	key, err = c.getKey(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("vnappmob: get key after refresh: %w", err)
	}
	return c.fetchSJC(ctx, key)
}

func (c *VNAppMobClient) fetchSJC(ctx context.Context, key string) (buy, sell float64, err error) {
	endpoint := c.baseURL() + "/api/v2/gold/sjc"
	if err := validateEndpoint(endpoint); err != nil {
		return 0, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("vnappmob: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (miti99bot)")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("vnappmob: SJC request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, 0, &httpStatusError{
			StatusCode: resp.StatusCode,
			msg:        fmt.Sprintf("vnappmob: SJC status %d", resp.StatusCode),
		}
	}

	var body struct {
		Results []struct {
			Buy1L  json.Number `json:"buy_1l"`
			Sell1L json.Number `json:"sell_1l"`
		} `json:"results"`
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		return 0, 0, fmt.Errorf("vnappmob: SJC decode: %w", err)
	}
	if len(body.Results) == 0 {
		return 0, 0, ErrNoGoldPrice
	}
	r := body.Results[0]
	buy, err = r.Buy1L.Float64()
	if err != nil {
		return 0, 0, ErrNoGoldPrice
	}
	sell, err = r.Sell1L.Float64()
	if err != nil {
		return 0, 0, ErrNoGoldPrice
	}
	if buy <= 0 || sell <= 0 || math.IsNaN(buy) || math.IsNaN(sell) || math.IsInf(buy, 0) || math.IsInf(sell, 0) {
		return 0, 0, ErrNoGoldPrice
	}
	return buy, sell, nil
}

type httpStatusError struct {
	StatusCode int
	msg        string
}

func (e *httpStatusError) Error() string { return e.msg }

// getKey returns a valid API key, refreshing from KV or the remote endpoint
// when the current key is missing or close to expiry.
func (c *VNAppMobClient) getKey(ctx context.Context) (string, error) {
	if c.Token != "" {
		return c.Token, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var cached struct {
		Token string `json:"token"`
		Exp   int64  `json:"exp"`
	}
	if err := c.KV.GetJSON(ctx, vnappmobKeyCacheKey, &cached); err == nil {
		if cached.Token != "" && !c.isExpired(cached.Exp) {
			return cached.Token, nil
		}
	} else if !errors.Is(err, storage.ErrNotFound) {
		log.Warn("vnappmob_kv_read_failed", "err", err)
	}

	if err := c.refreshKeyLocked(ctx); err != nil {
		return "", err
	}

	// Re-read from KV to get the freshly stored key.
	if err := c.KV.GetJSON(ctx, vnappmobKeyCacheKey, &cached); err != nil {
		return "", fmt.Errorf("vnappmob: read refreshed key: %w", err)
	}
	if cached.Token == "" {
		return "", fmt.Errorf("vnappmob: refreshed key is empty")
	}
	return cached.Token, nil
}

// refreshKey acquires a new JWT from VNAppMob and stores it in KV.
// It is safe to call concurrently; last-write-wins is acceptable because all
// callers receive a valid key.
func (c *VNAppMobClient) refreshKey(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refreshKeyLocked(ctx)
}

func (c *VNAppMobClient) refreshKeyLocked(ctx context.Context) error {
	endpoint := c.baseURL() + "/api/request_api_key?scope=gold"
	if err := validateEndpoint(endpoint); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("vnappmob: build refresh request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (miti99bot)")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("vnappmob: refresh request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("vnappmob: refresh status %d", resp.StatusCode)
	}

	var token string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("vnappmob: refresh read body: %w", err)
	}
	// The live endpoint wraps the JWT in {"results":"<jwt>"}. Keep fallbacks
	// for a JSON-quoted string or a raw JWT in case the format changes.
	token = strings.TrimSpace(string(body))
	var wrapped struct {
		Results string `json:"results"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil {
		token = strings.TrimSpace(wrapped.Results)
	} else {
		var quoted string
		if err := json.Unmarshal(body, &quoted); err == nil {
			token = strings.TrimSpace(quoted)
		}
	}
	if token == "" {
		return fmt.Errorf("vnappmob: refresh returned empty token")
	}

	exp, err := jwtExp(token)
	if err != nil {
		log.Warn("vnappmob_jwt_parse_failed", "err", err)
		// Store anyway; next call will refresh if expiry cannot be verified.
	}

	if err := c.KV.PutJSON(ctx, vnappmobKeyCacheKey, struct {
		Token string `json:"token"`
		Exp   int64  `json:"exp"`
	}{Token: token, Exp: exp}); err != nil {
		return fmt.Errorf("vnappmob: cache key: %w", err)
	}
	log.Info("vnappmob_key_refreshed", "exp", exp)
	return nil
}

// jwtExp extracts the exp claim from a JWT's payload segment using only the
// standard library. It does not verify the signature.
func jwtExp(token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("vnappmob: JWT has %d parts, want 3", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("vnappmob: decode JWT payload: %w", err)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, fmt.Errorf("vnappmob: unmarshal JWT claims: %w", err)
	}
	if claims.Exp == 0 {
		return 0, fmt.Errorf("vnappmob: JWT missing exp claim")
	}
	return claims.Exp, nil
}

// isExpired reports whether a key expiring at exp (Unix seconds) should be
// refreshed now. A 24-hour buffer avoids midnight edge cases and clock skew.
func (c *VNAppMobClient) isExpired(exp int64) bool {
	if exp == 0 {
		return true
	}
	return c.now().After(time.Unix(exp, 0).Add(-vnappmobRefreshBuffer))
}
