package stock

import (
	"context"
	"errors"
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
		"stock_cash_dividend",
		"stock_share_dividend",
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
			name: "cash dividend",
			text: "/stock_cash_dividend 1500 TCB extra",
			run: func(ctx context.Context, rb *testutil.RecordingBot, upd *models.Update) error {
				return s.handleCashDividend(ctx, rb.Bot, upd)
			},
			want: "Usage: /stock_cash_dividend <vnd_per_share> <TICKER>",
		},
		{
			name: "share dividend",
			text: "/stock_share_dividend 100:10 TCB extra",
			run: func(ctx context.Context, rb *testutil.RecordingBot, upd *models.Update) error {
				return s.handleShareDividend(ctx, rb.Bot, upd)
			},
			want: "Usage: /stock_share_dividend <owned:new> <TICKER>",
		},
		{
			name: "dividend",
			text: "/stock_dividend 1500 100:10 TCB extra",
			run: func(ctx context.Context, rb *testutil.RecordingBot, upd *models.Update) error {
				return s.handleDividend(ctx, rb.Bot, upd)
			},
			want: "Usage: /stock_dividend <vnd_per_share> <owned:new> <TICKER>",
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

func TestDividendHandlersRejectInvalidNumbers(t *testing.T) {
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
	if err := s.handleCashDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_cash_dividend 1.5 TCB")); err != nil {
		t.Fatalf("cash dividend: %v", err)
	}
	rb.AssertSentText(t, "positive whole number")

	rb.Reset()
	if err := s.handleShareDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_share_dividend 4.0:1 TCB")); err != nil {
		t.Fatalf("share dividend: %v", err)
	}
	rb.AssertSentText(t, "owned:new")

	rb.Reset()
	if err := s.handleDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_dividend Inf 4:1 TCB")); err != nil {
		t.Fatalf("combined dividend: %v", err)
	}
	rb.AssertSentText(t, "positive whole number")
}

type countingPortfolioStore struct {
	Store
	puts   int
	putErr error
}

func (s *countingPortfolioStore) Put(ctx context.Context, id string, p Portfolio) error {
	s.puts++
	if s.putErr != nil {
		return s.putErr
	}
	return s.Store.Put(ctx, id, p)
}

func seedStockPortfolio(t *testing.T, store Store, userID int64, held int64, balance float64) {
	t.Helper()
	p := NewPortfolio(123)
	p.Assets["TCB"] = held
	p.Currency["VND"] = balance
	if err := SavePortfolio(context.Background(), store, userID, p); err != nil {
		t.Fatalf("seed portfolio: %v", err)
	}
}

func TestHandleCashDividendAllowsRepeatedManualAdjustments(t *testing.T) {
	ctx := context.Background()
	store := newStockStore()
	seedStockPortfolio(t, store, 7, 139, 1000)
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	for i := 0; i < 2; i++ {
		if err := s.handleCashDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_cash_dividend 1500 TCB")); err != nil {
			t.Fatalf("handleCashDividend call %d: %v", i+1, err)
		}
	}
	p, err := LoadPortfolio(ctx, store, 7, 999)
	if err != nil {
		t.Fatalf("load portfolio: %v", err)
	}
	if got, want := p.Currency["VND"], float64(418000); got != want {
		t.Fatalf("balance = %v, want %v", got, want)
	}
}

func TestHandleCashDividendRejectsInexactBalanceSum(t *testing.T) {
	ctx := context.Background()
	base := newStockStore()
	seedStockPortfolio(t, base, 7, 1, 1)
	store := &countingPortfolioStore{Store: base}
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	if err := s.handleCashDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_cash_dividend 9007199254740992 TCB")); err != nil {
		t.Fatalf("handleCashDividend: %v", err)
	}
	if store.puts != 0 {
		t.Fatalf("store writes = %d, want 0", store.puts)
	}
	p, _ := LoadPortfolio(ctx, base, 7, 999)
	if p.Assets["TCB"] != 1 || p.Currency["VND"] != 1 {
		t.Fatalf("portfolio changed: %+v", p)
	}
	rb.AssertSentText(t, "Dividend amount is too large.")
}

func TestHandleShareDividendPreservesRatioAndFloors(t *testing.T) {
	ctx := context.Background()
	store := newStockStore()
	seedStockPortfolio(t, store, 7, 139, 0)
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	if err := s.handleShareDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_share_dividend 100:10 TCB")); err != nil {
		t.Fatalf("handleShareDividend: %v", err)
	}
	p, _ := LoadPortfolio(ctx, store, 7, 999)
	if got, want := p.Assets["TCB"], int64(152); got != want {
		t.Fatalf("holding = %d, want %d", got, want)
	}
	rb.AssertSentText(t, "Share dividend (100:10): +13 TCB")
}

func TestHandleShareDividendFormatsExactLargeQuantities(t *testing.T) {
	ctx := context.Background()
	store := newStockStore()
	seedStockPortfolio(t, store, 7, 9_007_199_254_740_993, 0)
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	if err := s.handleShareDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_share_dividend 1:1 TCB")); err != nil {
		t.Fatalf("handleShareDividend: %v", err)
	}
	rb.AssertSentText(t, "Share dividend (1:1): +9.007.199.254.740.993 TCB")
	rb.AssertSentText(t, "Holding: 9.007.199.254.740.993 → 18.014.398.509.481.986")
}

func TestHandleShareDividendRejectsZeroEntitlement(t *testing.T) {
	ctx := context.Background()
	base := newStockStore()
	seedStockPortfolio(t, base, 7, 9, 0)
	store := &countingPortfolioStore{Store: base}
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	if err := s.handleShareDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_share_dividend 100:10 TCB")); err != nil {
		t.Fatalf("handleShareDividend: %v", err)
	}
	if store.puts != 0 {
		t.Fatalf("store writes = %d, want 0", store.puts)
	}
	p, _ := LoadPortfolio(ctx, base, 7, 999)
	if p.Assets["TCB"] != 9 {
		t.Fatalf("holding changed to %d", p.Assets["TCB"])
	}
	rb.AssertSentText(t, "Minimum holding: 10")
}

func TestHandleShareDividendFormatsExactLargeMinimum(t *testing.T) {
	ctx := context.Background()
	store := newStockStore()
	seedStockPortfolio(t, store, 7, 1, 0)
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	if err := s.handleShareDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_share_dividend 9007199254740993:1 TCB")); err != nil {
		t.Fatalf("handleShareDividend: %v", err)
	}
	rb.AssertSentText(t, "Minimum holding: 9.007.199.254.740.993.")
}

func TestHandleCombinedDividendUsesPreEventHoldingAndOneSave(t *testing.T) {
	ctx := context.Background()
	base := newStockStore()
	seedStockPortfolio(t, base, 7, 139, 1000)
	store := &countingPortfolioStore{Store: base}
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	if err := s.handleDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_dividend 1500 100:10 TCB")); err != nil {
		t.Fatalf("handleDividend: %v", err)
	}
	if store.puts != 1 {
		t.Fatalf("store writes = %d, want 1", store.puts)
	}
	p, _ := LoadPortfolio(ctx, base, 7, 999)
	if p.Assets["TCB"] != 152 || p.Currency["VND"] != 209500 {
		t.Fatalf("portfolio = %+v", p)
	}
	rb.AssertSentText(t, "Dividend for TCB (100:10)")
	rb.AssertSentText(t, "Cash: 1.500 VND × 139 = 208.500 VND")
}

func TestHandleCombinedDividendRejectsInexactBalanceSum(t *testing.T) {
	ctx := context.Background()
	base := newStockStore()
	seedStockPortfolio(t, base, 7, 1, 1)
	store := &countingPortfolioStore{Store: base}
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	if err := s.handleDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_dividend 9007199254740992 1:1 TCB")); err != nil {
		t.Fatalf("handleDividend: %v", err)
	}
	if store.puts != 0 {
		t.Fatalf("store writes = %d, want 0", store.puts)
	}
	p, _ := LoadPortfolio(ctx, base, 7, 999)
	if p.Assets["TCB"] != 1 || p.Currency["VND"] != 1 {
		t.Fatalf("portfolio changed: %+v", p)
	}
	rb.AssertSentText(t, "Dividend amount is too large.")
}

func TestHandleCombinedDividendCreditsCashWhenSharesRoundToZero(t *testing.T) {
	ctx := context.Background()
	store := newStockStore()
	seedStockPortfolio(t, store, 7, 9, 100)
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	if err := s.handleDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_dividend 1500 100:10 TCB")); err != nil {
		t.Fatalf("handleDividend: %v", err)
	}
	p, _ := LoadPortfolio(ctx, store, 7, 999)
	if p.Assets["TCB"] != 9 || p.Currency["VND"] != 13600 {
		t.Fatalf("portfolio = %+v", p)
	}
	rb.AssertSentText(t, "Shares: +0")
}

func TestDividendSaveFailureLeavesStoredPortfolioUnchanged(t *testing.T) {
	ctx := context.Background()
	base := newStockStore()
	seedStockPortfolio(t, base, 7, 139, 1000)
	store := &countingPortfolioStore{Store: base, putErr: errors.New("forced write failure")}
	s := &state{store: store, nowFn: func() time.Time { return time.UnixMilli(123) }}
	rb := testutil.NewRecordingBot(t)

	if err := s.handleDividend(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/stock_dividend 1500 100:10 TCB")); err != nil {
		t.Fatalf("handleDividend: %v", err)
	}
	p, _ := LoadPortfolio(ctx, base, 7, 999)
	if p.Assets["TCB"] != 139 || p.Currency["VND"] != 1000 {
		t.Fatalf("stored portfolio changed: %+v", p)
	}
	rb.AssertSentText(t, "Could not save portfolio")
}

func modDepsForTest() modules.Deps {
	return modules.Deps{Store: storage.NewMemoryProvider().Collection("stock")}
}
