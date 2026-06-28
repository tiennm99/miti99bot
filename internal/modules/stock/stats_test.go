package stock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tiennm99/miti99bot/internal/testutil"
)

func TestHandleStats_UsesSSIBatchPrices(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	requests := 0
	var gotStocks []string
	priceSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/stock/multiple" {
			t.Errorf("path = %q, want /stock/multiple", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		gotStocks = append([]string(nil), r.PostForm["stocks"]...)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"stockSymbol":"MWG","matchedPrice":70000},{"stockSymbol":"TCB","matchedPrice":30000},{"stockSymbol":"FPT","matchedPrice":120000}]}`))
	}))
	t.Cleanup(priceSrv.Close)

	store := newStockStore()
	p := NewPortfolio(now.UnixMilli())
	p.Currency["VND"] = 2335000
	p.Meta.Invested = 1000000000
	p.AddAsset("MWG", 1800)
	p.AddAsset("TCB", 4200)
	p.AddAsset("FPT", 2300)
	if err := SavePortfolio(ctx, store, 7, p); err != nil {
		t.Fatalf("SavePortfolio: %v", err)
	}

	s := &state{
		store:  store,
		prices: &PriceClient{HTTP: priceSrv.Client(), URL: priceSrv.URL},
		nowFn:  func() time.Time { return now },
	}
	rb := testutil.NewRecordingBot(t)
	if err := s.handleStats(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_stats")); err != nil {
		t.Fatalf("handleStats: %v", err)
	}

	if requests != 1 {
		t.Fatalf("requests = %d, want one SSI batch request", requests)
	}
	sort.Strings(gotStocks)
	if wantStocks := []string{"FPT", "MWG", "TCB"}; !reflect.DeepEqual(gotStocks, wantStocks) {
		t.Fatalf("stocks form values = %#v, want %#v", gotStocks, wantStocks)
	}

	text := rb.LastSent().Text()
	for _, want := range []string{
		"MWG x1800 @ 70.000 VND = 126.000.000 VND",
		"TCB x4200 @ 30.000 VND = 126.000.000 VND",
		"FPT x2300 @ 120.000 VND = 276.000.000 VND",
		"Total value: 530.335.000 VND",
		"P&L: -469.665.000 VND (-46.97%)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("stats missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "(no price)") {
		t.Fatalf("stats rendered missing prices:\n%s", text)
	}
}
