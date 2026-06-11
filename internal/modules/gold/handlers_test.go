package gold

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

type fakePriceFetcher struct {
	price float64
	err   error
}

func (f fakePriceFetcher) FetchLuongPrice(context.Context) (float64, error) {
	return f.price, f.err
}

func (f fakePriceFetcher) FetchPrice(context.Context) (GoldPrice, error) {
	if f.err != nil {
		return GoldPrice{}, f.err
	}
	return GoldPrice{XAUUSD: 3000, USDVND: 25000, VNDPerLuong: f.price}, nil
}

func newTestState(price float64, err error) *state {
	return &state{
		kv:     storage.NewMemoryKVStore(),
		prices: fakePriceFetcher{price: price, err: err},
		nowFn:  func() time.Time { return time.UnixMilli(123) },
	}
}

func TestParsePositiveFinite(t *testing.T) {
	bad := []string{"", "0", "-1", "NaN", "Inf", "+Inf", "-Inf", "1e9999"}
	for _, in := range bad {
		if got, ok := parsePositiveFinite(in); ok {
			t.Fatalf("parsePositiveFinite(%q) = %v, true; want false", in, got)
		}
	}
	if got, ok := parsePositiveFinite("0.5"); !ok || got != 0.5 {
		t.Fatalf("parsePositiveFinite valid = %v, %v", got, ok)
	}
}

func TestModuleRegistersExpectedCommands(t *testing.T) {
	mod := New(modDepsForTest())
	got := map[string]bool{}
	for _, cmd := range mod.Commands {
		got[cmd.Name] = true
	}
	for _, name := range []string{"gold_price", "gold_topup", "gold_buy", "gold_sell", "gold_stats"} {
		if !got[name] {
			t.Fatalf("missing command %s", name)
		}
	}
}

func TestHandleTopup(t *testing.T) {
	ctx := context.Background()
	s := newTestState(1000, nil)
	rb := testutil.NewRecordingBot(t)
	if err := s.handleTopup(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/gold_topup 5000000")); err != nil {
		t.Fatalf("handleTopup: %v", err)
	}
	rb.AssertSentText(t, "Topped up 5.000.000 VND")
	p, err := LoadPortfolio(ctx, s.kv, 7, 999)
	if err != nil {
		t.Fatalf("LoadPortfolio: %v", err)
	}
	if p.VND != 5_000_000 || p.Meta.Invested != 5_000_000 {
		t.Fatalf("portfolio: got %+v", p)
	}
}

func TestHandleBuyAndSell(t *testing.T) {
	ctx := context.Background()
	s := newTestState(2_000_000, nil)
	rb := testutil.NewRecordingBot(t)
	if err := s.handleTopup(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/gold_topup 5000000")); err != nil {
		t.Fatalf("topup: %v", err)
	}
	rb.Reset()
	if err := s.handleBuy(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/gold_buy 1.25")); err != nil {
		t.Fatalf("buy: %v", err)
	}
	rb.AssertSentText(t, "Bought 1.25 luong gold")
	p, _ := LoadPortfolio(ctx, s.kv, 7, 999)
	if p.Luong != 1.25 || p.VND != 2_500_000 {
		t.Fatalf("after buy: %+v", p)
	}
	rb.Reset()
	if err := s.handleSell(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/gold_sell 1.25")); err != nil {
		t.Fatalf("sell: %v", err)
	}
	rb.AssertSentText(t, "Sold 1.25 luong gold")
	p, _ = LoadPortfolio(ctx, s.kv, 7, 999)
	if p.Luong != 0 || p.VND != 5_000_000 {
		t.Fatalf("after sell: %+v", p)
	}
}

func TestHandleBuyInsufficientVND(t *testing.T) {
	s := newTestState(2_000_000, nil)
	rb := testutil.NewRecordingBot(t)
	if err := s.handleBuy(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/gold_buy 1")); err != nil {
		t.Fatalf("buy: %v", err)
	}
	rb.AssertSentText(t, "Insufficient VND")
}

func TestHandleSellInsufficientGold(t *testing.T) {
	s := newTestState(2_000_000, nil)
	rb := testutil.NewRecordingBot(t)
	if err := s.handleSell(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/gold_sell 1")); err != nil {
		t.Fatalf("sell: %v", err)
	}
	rb.AssertSentText(t, "Insufficient gold")
}

func TestPriceErrorDoesNotMutatePortfolio(t *testing.T) {
	ctx := context.Background()
	s := newTestState(0, errors.New("upstream down"))
	rb := testutil.NewRecordingBot(t)
	if err := s.handleBuy(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/gold_buy 1")); err != nil {
		t.Fatalf("buy: %v", err)
	}
	rb.AssertSentText(t, "Could not fetch gold price")
	p, err := LoadPortfolio(ctx, s.kv, 7, 999)
	if err != nil {
		t.Fatalf("LoadPortfolio: %v", err)
	}
	if p.VND != 0 || p.Luong != 0 {
		t.Fatalf("unexpected mutation: %+v", p)
	}
}

func TestStatsWithAndWithoutPrice(t *testing.T) {
	ctx := context.Background()
	s := newTestState(2_000_000, nil)
	rb := testutil.NewRecordingBot(t)
	_ = s.handleTopup(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/gold_topup 5000000"))
	_ = s.handleBuy(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/gold_buy 1"))
	rb.Reset()
	if err := s.handleStats(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/gold_stats")); err != nil {
		t.Fatalf("stats: %v", err)
	}
	text := rb.LastSent().Text()
	for _, want := range []string{"Gold Account Summary", "Gold: 1 luong", "Price: 2.000.000 VND/luong", "P&L:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("stats missing %q in %q", want, text)
		}
	}
	s.prices = fakePriceFetcher{err: ErrNoGoldPrice}
	rb.Reset()
	if err := s.handleStats(ctx, rb.Bot, testutil.NewPrivateMessage(7, "/gold_stats")); err != nil {
		t.Fatalf("stats no price: %v", err)
	}
	rb.AssertSentText(t, "Price: no price")
}

func modDepsForTest() modules.Deps {
	return modules.Deps{KV: storage.NewMemoryKVStore()}
}

func TestHandlePrice(t *testing.T) {
	s := newTestState(90_000_000, nil)
	rb := testutil.NewRecordingBot(t)
	if err := s.handlePrice(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/gold_price")); err != nil {
		t.Fatalf("handlePrice: %v", err)
	}
	text := rb.LastSent().Text()
	for _, want := range []string{"Gold Spot Price", "XAU:", "USD/oz", "VND:", "/luong"} {
		if !strings.Contains(text, want) {
			t.Fatalf("price missing %q in %q", want, text)
		}
	}
}

func TestHandlePriceRejectsArgs(t *testing.T) {
	s := newTestState(90_000_000, nil)
	rb := testutil.NewRecordingBot(t)
	if err := s.handlePrice(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/gold_price USD")); err != nil {
		t.Fatalf("handlePrice: %v", err)
	}
	rb.AssertSentText(t, "Usage: /gold_price")
}

func TestHandlePriceFetchError(t *testing.T) {
	s := newTestState(0, errors.New("upstream down"))
	rb := testutil.NewRecordingBot(t)
	if err := s.handlePrice(context.Background(), rb.Bot, testutil.NewPrivateMessage(7, "/gold_price")); err != nil {
		t.Fatalf("handlePrice: %v", err)
	}
	rb.AssertSentText(t, "Could not fetch gold price")
}
