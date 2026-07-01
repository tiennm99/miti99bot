package stock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"

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

func TestMutableHandlersRejectExtraArgs(t *testing.T) {
	ctx := context.Background()
	s := &state{
		store:  newStockStore(),
		prices: &PriceClient{},
		nowFn:  func() time.Time { return time.UnixMilli(123) },
	}
	cases := []struct {
		name string
		text string
		run  func(context.Context, *testutil.RecordingBot, *models.Update) error
		want string
	}{
		{
			name: "topup",
			text: "/stock_topup 1000000 extra",
			run: func(ctx context.Context, rb *testutil.RecordingBot, upd *models.Update) error {
				return s.handleTopup(ctx, rb.Bot, upd)
			},
			want: "Usage: /stock_topup <amount>",
		},
		{
			name: "buy",
			text: "/stock_buy 100 TCB extra",
			run: func(ctx context.Context, rb *testutil.RecordingBot, upd *models.Update) error {
				return s.handleBuy(ctx, rb.Bot, upd)
			},
			want: "Usage: /stock_buy <qty> <TICKER>",
		},
		{
			name: "sell",
			text: "/stock_sell 100 TCB extra",
			run: func(ctx context.Context, rb *testutil.RecordingBot, upd *models.Update) error {
				return s.handleSell(ctx, rb.Bot, upd)
			},
			want: "Usage: /stock_sell <qty> <TICKER>",
		},
		{
			name: "bonus",
			text: "/stock_bonus 100 TCB extra",
			run: func(ctx context.Context, rb *testutil.RecordingBot, upd *models.Update) error {
				return s.handleBonus(ctx, rb.Bot, upd)
			},
			want: "Usage: /stock_bonus <qty> <TICKER>",
		},
		{
			name: "dividend",
			text: "/stock_dividend 1500 TCB extra",
			run: func(ctx context.Context, rb *testutil.RecordingBot, upd *models.Update) error {
				return s.handleDividend(ctx, rb.Bot, upd)
			},
			want: "Usage: /stock_dividend <amount_per_share> <TICKER>",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rb := testutil.NewRecordingBot(t)
			if err := tc.run(ctx, rb, testutil.NewPrivateMessage(7, tc.text)); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			rb.AssertSentText(t, tc.want)
		})
	}

	p, err := LoadPortfolio(ctx, s.store, 7, 999)
	if err != nil {
		t.Fatalf("LoadPortfolio: %v", err)
	}
	if p.Currency["VND"] != 0 || len(p.Assets) != 0 {
		t.Fatalf("invalid commands mutated portfolio: %+v", p)
	}
}

func TestMutableHandlersRejectNonFiniteVND(t *testing.T) {
	ctx := context.Background()
	s := &state{
		store:  newStockStore(),
		prices: &PriceClient{},
		nowFn:  func() time.Time { return time.UnixMilli(123) },
	}

	rb := testutil.NewRecordingBot(t)
	if err := s.handleTopup(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_topup NaN")); err != nil {
		t.Fatalf("topup: %v", err)
	}
	rb.AssertSentText(t, "positive finite")

	rb.Reset()
	if err := s.handleDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_dividend Inf TCB")); err != nil {
		t.Fatalf("dividend: %v", err)
	}
	rb.AssertSentText(t, "positive finite")
}

func modDepsForTest() modules.Deps {
	return modules.Deps{Store: storage.NewMemoryProvider().Collection("stock")}
}
