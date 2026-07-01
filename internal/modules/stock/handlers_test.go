package stock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

func TestModuleRegistersExpectedCommands(t *testing.T) {
	mod := New(modDepsForTest())
	got := map[string]bool{}
	for _, cmd := range mod.Commands {
		got[cmd.Name] = true
	}
	for _, name := range []string{
		"stock_price",
		"stock_topup",
		"stock_buy",
		"stock_sell",
		"stock_bonus",
		"stock_dividend",
		"stock_portfolio",
	} {
		if !got[name] {
			t.Fatalf("missing command %s", name)
		}
	}
}

func TestHandlePrice(t *testing.T) {
	priceSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/stock/TCB" {
			t.Errorf("path = %q, want /stock/TCB", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"matchedPrice":30000}}`))
	}))
	t.Cleanup(priceSrv.Close)

	s := &state{
		store:  newStockStore(),
		prices: &PriceClient{HTTP: priceSrv.Client(), URL: priceSrv.URL},
		nowFn:  func() time.Time { return time.UnixMilli(123) },
	}
	rb := testutil.NewRecordingBot(t)
	if err := s.handlePrice(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/stock_price tcb")); err != nil {
		t.Fatalf("handlePrice: %v", err)
	}

	if got := rb.LastSent().Text(); !strings.Contains(got, "TCB price: 30.000 VND") {
		t.Fatalf("price reply = %q", got)
	}
}

func TestHandlePriceUsage(t *testing.T) {
	s := &state{prices: &PriceClient{}}
	rb := testutil.NewRecordingBot(t)
	if err := s.handlePrice(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/stock_price")); err != nil {
		t.Fatalf("handlePrice: %v", err)
	}
	rb.AssertSentText(t, "Usage: /stock_price <TICKER>")
}

func modDepsForTest() modules.Deps {
	return modules.Deps{Store: storage.NewMemoryProvider().Collection("stock")}
}
