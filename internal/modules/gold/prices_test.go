package gold

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newChainTestClient pins every provider URL to the test server so no test
// ever falls through to a real network endpoint.
func newChainTestClient(srv *httptest.Server) *GoldPriceClient {
	return &GoldPriceClient{
		GoldURL:       srv.URL + "/gold",
		SwissquoteURL: srv.URL + "/swissquote",
		NBPURL:        srv.URL + "/nbp",
		FXURL:         srv.URL + "/fx",
	}
}

func TestGoldPriceClient_FetchLuongPrice(t *testing.T) {
	now := time.Unix(100, 0)
	var fxHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gold":
			_, _ = w.Write([]byte(`{"currency":"USD","price":2000}`))
		case "/fx":
			atomic.AddInt32(&fxHits, 1)
			_, _ = w.Write([]byte(`{"result":"success","rates":{"VND":25000},"time_next_update_unix":1000}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := newChainTestClient(srv)
	c.nowFn = func() time.Time { return now }

	got, err := c.FetchLuongPrice(context.Background())
	if err != nil {
		t.Fatalf("FetchLuongPrice: %v", err)
	}
	want := 2000 * 25000 * (gramsPerLuong / gramsPerTroyOunce)
	if math.Abs(got-want) > 0.01 {
		t.Errorf("price: got %v, want %v", got, want)
	}
	if _, err := c.FetchLuongPrice(context.Background()); err != nil {
		t.Fatalf("FetchLuongPrice cached: %v", err)
	}
	if atomic.LoadInt32(&fxHits) != 1 {
		t.Errorf("FX hits: got %d, want 1", fxHits)
	}
}

func TestGoldPriceClient_FallbackToSwissquote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gold":
			w.WriteHeader(http.StatusForbidden) // primary blocked, like goldprice.org was
		case "/swissquote":
			_, _ = w.Write([]byte(`[{"spreadProfilePrices":[{"bid":4000,"ask":4010}]}]`))
		case "/fx":
			_, _ = w.Write([]byte(`{"result":"success","rates":{"VND":25000}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	p, err := newChainTestClient(srv).FetchPrice(context.Background())
	if err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	if p.XAUUSD != 4005 { // mid of bid/ask
		t.Errorf("XAUUSD: got %v, want 4005", p.XAUUSD)
	}
}

func TestGoldPriceClient_FallbackToNBP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gold", "/swissquote":
			w.WriteHeader(http.StatusInternalServerError)
		case "/nbp":
			_, _ = w.Write([]byte(`[{"data":"2026-06-11","cena":500}]`))
		case "/fx":
			_, _ = w.Write([]byte(`{"result":"success","rates":{"VND":25000,"PLN":4}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	p, err := newChainTestClient(srv).FetchPrice(context.Background())
	if err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	want := 500 * gramsPerTroyOunce / 4 // PLN/gram → USD/oz via USD→PLN rate
	if math.Abs(p.XAUUSD-want) > 0.01 {
		t.Errorf("XAUUSD: got %v, want %v", p.XAUUSD, want)
	}
}

func TestGoldPriceClient_AllProvidersFailIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := newChainTestClient(srv).FetchPrice(context.Background())
	if err == nil {
		t.Fatal("want error when every provider fails")
	}
	// A full-chain outage is a fetch failure, not a benign "no data" reply.
	if errors.Is(err, ErrNoGoldPrice) {
		t.Fatalf("got ErrNoGoldPrice, want retryable error: %v", err)
	}
}

func TestGoldPriceClient_InvalidProviderResponses(t *testing.T) {
	cases := []struct {
		name       string
		gold       string
		swissquote string
		nbp        string
	}{
		{name: "empty bodies", gold: `{}`, swissquote: `[]`, nbp: `[]`},
		{name: "wrong currency and zero quotes", gold: `{"currency":"EUR","price":2000}`, swissquote: `[{"spreadProfilePrices":[{"bid":0,"ask":0}]}]`, nbp: `[{"cena":0}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/gold":
					_, _ = w.Write([]byte(tc.gold))
				case "/swissquote":
					_, _ = w.Write([]byte(tc.swissquote))
				case "/nbp":
					_, _ = w.Write([]byte(tc.nbp))
				case "/fx":
					_, _ = w.Write([]byte(`{"result":"success","rates":{"VND":25000,"PLN":4}}`))
				}
			}))
			defer srv.Close()
			if _, err := newChainTestClient(srv).FetchPrice(context.Background()); err == nil {
				t.Fatal("want error for invalid provider data")
			}
		})
	}
}

func TestGoldPriceClient_MissingFXRateReturnsNoPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gold" {
			_, _ = w.Write([]byte(`{"currency":"USD","price":2000}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"success","rates":{}}`))
	}))
	defer srv.Close()
	_, err := newChainTestClient(srv).FetchPrice(context.Background())
	if !errors.Is(err, ErrNoGoldPrice) {
		t.Fatalf("got %v, want ErrNoGoldPrice", err)
	}
}

func TestGoldPriceClient_OverflowPriceReturnsNoPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gold" {
			_, _ = w.Write([]byte(`{"currency":"USD","price":1e308}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"success","rates":{"VND":1e308}}`))
	}))
	defer srv.Close()
	_, err := newChainTestClient(srv).FetchLuongPrice(context.Background())
	if !errors.Is(err, ErrNoGoldPrice) {
		t.Fatalf("got %v, want ErrNoGoldPrice", err)
	}
}

func TestGoldPriceClient_FXRateLimitedIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gold" {
			_, _ = w.Write([]byte(`{"currency":"USD","price":2000}`))
			return
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := newChainTestClient(srv).FetchLuongPrice(context.Background())
	if err == nil || errors.Is(err, ErrNoGoldPrice) {
		t.Fatalf("got %v, want retryable non-ErrNoGoldPrice", err)
	}
}

func TestGoldPriceClient_FetchPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gold":
			_, _ = w.Write([]byte(`{"currency":"USD","price":3000}`))
		case "/fx":
			_, _ = w.Write([]byte(`{"result":"success","rates":{"VND":25000}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	p, err := newChainTestClient(srv).FetchPrice(context.Background())
	if err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	if p.XAUUSD != 3000 {
		t.Errorf("XAUUSD: got %v, want 3000", p.XAUUSD)
	}
	if p.USDVND != 25000 {
		t.Errorf("USDVND: got %v, want 25000", p.USDVND)
	}
	want := 3000 * 25000 * (gramsPerLuong / gramsPerTroyOunce)
	if math.Abs(p.VNDPerLuong-want) > 0.01 {
		t.Errorf("VNDPerLuong: got %v, want %v", p.VNDPerLuong, want)
	}
}

func TestValidateEndpoint(t *testing.T) {
	if err := validateEndpoint("https://example.com/path"); err != nil {
		t.Fatalf("https should pass: %v", err)
	}
	if err := validateEndpoint("http://localhost:1234/path"); err != nil {
		t.Fatalf("localhost http should pass: %v", err)
	}
	if err := validateEndpoint("http://127.0.0.1:1234/path"); err != nil {
		t.Fatalf("loopback http should pass: %v", err)
	}
	if err := validateEndpoint("http://example.com/path"); err == nil {
		t.Fatal("remote http should fail")
	}
}
