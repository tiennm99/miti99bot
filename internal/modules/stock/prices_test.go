package stock

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestPriceClient(t *testing.T, handler http.HandlerFunc) (*PriceClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &PriceClient{
		HTTP: &http.Client{Timeout: 2 * time.Second},
		URL:  srv.URL,
	}, srv
}

func TestPriceClient_HappyPath(t *testing.T) {
	c, _ := newTestPriceClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/stock/TCB" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Origin"); got != "https://iboard.ssi.com.vn" {
			t.Errorf("origin = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"stockSymbol":"TCB","matchedPrice":24500}}`))
	})
	got, err := c.FetchPrice(context.Background(), "TCB")
	if err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	if got != 24500 {
		t.Errorf("price: got %v, want 24500", got)
	}
}

func TestPriceClient_BatchHappyPath(t *testing.T) {
	c, _ := newTestPriceClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/stock/multiple" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("content-type = %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		stocks := r.Form["stocks"]
		if strings.Join(stocks, ",") != "TCB,FPT" {
			t.Errorf("stocks = %v, want [TCB FPT]", stocks)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"stockSymbol":"TCB","matchedPrice":24500},{"stockSymbol":"FPT","matchedPrice":120000}]}`))
	})
	got, err := c.FetchPrices(context.Background(), []string{"TCB", "FPT"})
	if err != nil {
		t.Fatalf("FetchPrices: %v", err)
	}
	if got["TCB"] != 24500 || got["FPT"] != 120000 {
		t.Errorf("prices = %+v", got)
	}
}

func TestPriceClient_BatchOmitsInvalidQuotes(t *testing.T) {
	c, _ := newTestPriceClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"stockSymbol":"TCB","matchedPrice":24500},{"stockSymbol":"BAD","matchedPrice":0},{"stockSymbol":"NEG","matchedPrice":-1}]}`))
	})
	got, err := c.FetchPrices(context.Background(), []string{"TCB", "BAD", "NEG"})
	if err != nil {
		t.Fatalf("FetchPrices: %v", err)
	}
	if len(got) != 1 || got["TCB"] != 24500 {
		t.Errorf("prices = %+v, want only TCB", got)
	}
}

func TestPriceClient_NoData_ReturnsErrNoPrice(t *testing.T) {
	c, _ := newTestPriceClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":null}`))
	})
	_, err := c.FetchPrice(context.Background(), "NOPE")
	if !errors.Is(err, ErrNoPrice) {
		t.Errorf("got %v, want ErrNoPrice", err)
	}
}

func TestPriceClient_BatchNoUsableData_ReturnsErrNoPrice(t *testing.T) {
	c, _ := newTestPriceClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"stockSymbol":"NOPE","matchedPrice":0}]}`))
	})
	_, err := c.FetchPrices(context.Background(), []string{"NOPE"})
	if !errors.Is(err, ErrNoPrice) {
		t.Errorf("got %v, want ErrNoPrice", err)
	}
	if !strings.Contains(err.Error(), "SSI batch returned no usable quotes for NOPE (data_len=1)") {
		t.Errorf("error = %q, want batch diagnostic", err.Error())
	}
}

func TestPriceClient_4xx_ReturnsErrNoPrice(t *testing.T) {
	c, _ := newTestPriceClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("blocked by upstream"))
	})
	_, err := c.FetchPrice(context.Background(), "BADTICKER")
	if !errors.Is(err, ErrNoPrice) {
		t.Errorf("got %v, want ErrNoPrice", err)
	}
	if !strings.Contains(err.Error(), "SSI status 404 body") || !strings.Contains(err.Error(), "blocked by upstream") {
		t.Errorf("error = %q, want status/body diagnostic", err.Error())
	}
}

func TestPriceClient_NegativeMatchedPrice_ReturnsErrNoPrice(t *testing.T) {
	c, _ := newTestPriceClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"stockSymbol":"WEIRD","matchedPrice":-1}}`))
	})
	_, err := c.FetchPrice(context.Background(), "WEIRD")
	if !errors.Is(err, ErrNoPrice) {
		t.Errorf("got %v, want ErrNoPrice", err)
	}
}

func TestPriceClient_EmptyTicker(t *testing.T) {
	c := &PriceClient{}
	_, err := c.FetchPrice(context.Background(), "")
	if err == nil {
		t.Error("empty ticker: expected error, got nil")
	}
}
