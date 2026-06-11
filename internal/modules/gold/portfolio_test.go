package gold

import (
	"context"
	"math"
	"testing"

	stocktrading "github.com/tiennm99/miti99bot/internal/modules/trading"
	"github.com/tiennm99/miti99bot/internal/storage"
)

func TestLoadPortfolio_FirstTimeUser(t *testing.T) {
	kv := storage.NewMemoryKVStore()
	p, err := LoadPortfolio(context.Background(), kv, 42, 123)
	if err != nil {
		t.Fatalf("LoadPortfolio: %v", err)
	}
	if p.VND != 0 || p.Luong != 0 || p.Meta.CreatedAt != 123 {
		t.Fatalf("portfolio: got %+v", p)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	kv := storage.NewMemoryKVStore()
	p := NewPortfolio(1)
	p.AddVND(5_000_000)
	p.AddLuong(1.25)
	p.Meta.Invested = 5_000_000
	if err := SavePortfolio(context.Background(), kv, 42, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadPortfolio(context.Background(), kv, 42, 999)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.VND != 5_000_000 || got.Luong != 1.25 || got.Meta.CreatedAt != 1 {
		t.Fatalf("round trip: got %+v", got)
	}
}

func TestDeductLuongDustCleanup(t *testing.T) {
	p := NewPortfolio(0)
	p.AddLuong(0.1)
	p.AddLuong(0.2)
	ok, held := p.DeductLuong(0.3)
	if !ok {
		t.Fatalf("deduct: ok=false held=%v", held)
	}
	if p.Luong != 0 {
		t.Fatalf("dust not cleaned: got %.20f", p.Luong)
	}
}

func TestDeductVNDInsufficient(t *testing.T) {
	p := NewPortfolio(0)
	p.AddVND(1000)
	ok, bal := p.DeductVND(1500)
	if ok || bal != 1000 || p.VND != 1000 {
		t.Fatalf("deduct: ok=%v bal=%v p=%+v", ok, bal, p)
	}
}

func TestNormalizeAmountSpecialValues(t *testing.T) {
	for _, n := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), goldDustEpsilon / 2} {
		if got := normalizeAmount(n); got != 0 {
			t.Fatalf("normalizeAmount(%v) = %v, want 0", n, got)
		}
	}
}

func TestTradingAndGoldPortfolioKeysDoNotCollide(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	goldPortfolio := NewPortfolio(1)
	goldPortfolio.AddLuong(2)
	if err := SavePortfolio(ctx, provider.For("gold"), 7, goldPortfolio); err != nil {
		t.Fatalf("save gold: %v", err)
	}
	tradingPortfolio := stocktrading.NewPortfolio(1)
	tradingPortfolio.AddAsset("TCB", 100)
	if err := stocktrading.SavePortfolio(ctx, provider.For("trading"), 7, tradingPortfolio); err != nil {
		t.Fatalf("save trading: %v", err)
	}
	keys, err := provider.Base().List(ctx, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := map[string]bool{"gold:user:7": false, "trading:user:7": false}
	for _, key := range keys {
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("missing raw key %q in %v", key, keys)
		}
	}
}
