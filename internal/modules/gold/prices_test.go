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

func TestGoldPriceClient_FetchLuongPrice(t *testing.T) {
	now := time.Unix(100, 0)
	var fxHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gold":
			_, _ = w.Write([]byte(`{"items":[{"curr":"USD","xauPrice":2000}]}`))
		case "/fx":
			atomic.AddInt32(&fxHits, 1)
			_, _ = w.Write([]byte(`{"result":"success","rates":{"VND":25000},"time_next_update_unix":1000}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &GoldPriceClient{GoldURL: srv.URL + "/gold", FXURL: srv.URL + "/fx", nowFn: func() time.Time { return now }}

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

func TestGoldPriceClient_InvalidResponses(t *testing.T) {
	cases := []struct {
		name string
		gold string
		fx   string
	}{
		{name: "missing gold", gold: `{"items":[]}`, fx: `{"result":"success","rates":{"VND":25000}}`},
		{name: "wrong currency", gold: `{"items":[{"curr":"EUR","xauPrice":2000}]}`, fx: `{"result":"success","rates":{"VND":25000}}`},
		{name: "missing fx", gold: `{"items":[{"curr":"USD","xauPrice":2000}]}`, fx: `{"result":"success","rates":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/gold" {
					_, _ = w.Write([]byte(tc.gold))
					return
				}
				_, _ = w.Write([]byte(tc.fx))
			}))
			defer srv.Close()
			c := &GoldPriceClient{GoldURL: srv.URL + "/gold", FXURL: srv.URL + "/fx"}
			_, err := c.FetchLuongPrice(context.Background())
			if !errors.Is(err, ErrNoGoldPrice) {
				t.Errorf("got %v, want ErrNoGoldPrice", err)
			}
		})
	}
}

func TestGoldPriceClient_OverflowPriceReturnsNoPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gold" {
			_, _ = w.Write([]byte(`{"items":[{"curr":"USD","xauPrice":1e308}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"success","rates":{"VND":1e308}}`))
	}))
	defer srv.Close()
	c := &GoldPriceClient{GoldURL: srv.URL + "/gold", FXURL: srv.URL + "/fx"}
	_, err := c.FetchLuongPrice(context.Background())
	if !errors.Is(err, ErrNoGoldPrice) {
		t.Fatalf("got %v, want ErrNoGoldPrice", err)
	}
}

func TestGoldPriceClient_FXRateLimitedIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gold" {
			_, _ = w.Write([]byte(`{"items":[{"curr":"USD","xauPrice":2000}]}`))
			return
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := &GoldPriceClient{GoldURL: srv.URL + "/gold", FXURL: srv.URL + "/fx"}
	_, err := c.FetchLuongPrice(context.Background())
	if err == nil || errors.Is(err, ErrNoGoldPrice) {
		t.Fatalf("got %v, want retryable non-ErrNoGoldPrice", err)
	}
}

func TestGoldPriceClient_FetchPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gold":
			_, _ = w.Write([]byte(`{"items":[{"curr":"USD","xauPrice":3000}]}`))
		case "/fx":
			_, _ = w.Write([]byte(`{"result":"success","rates":{"VND":25000}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &GoldPriceClient{GoldURL: srv.URL + "/gold", FXURL: srv.URL + "/fx"}
	p, err := c.FetchPrice(context.Background())
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
