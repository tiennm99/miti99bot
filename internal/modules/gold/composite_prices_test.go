package gold

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
)

// stubVNAppMobClient implements just enough of the VNAppMob client contract
// for composite fetcher tests.
type stubVNAppMobClient struct {
	buy float64
	sell float64
	err  error
}

func (s *stubVNAppMobClient) FetchSJCPrice(context.Context) (float64, float64, error) {
	return s.buy, s.sell, s.err
}

type stubFallbackFetcher struct {
	price float64
	err   error
}

func (s *stubFallbackFetcher) FetchLuongPrice(context.Context) (float64, error) {
	return s.price, s.err
}

func (s *stubFallbackFetcher) FetchLuongPrices(context.Context) (float64, float64, error) {
	return s.price, s.price, s.err
}

func (s *stubFallbackFetcher) FetchPrice(context.Context) (GoldPrice, error) {
	if s.err != nil {
		return GoldPrice{}, s.err
	}
	return GoldPrice{VNDPerLuong: s.price, Source: "xau-fallback"}, nil
}

func TestCompositeFetcher_PrefersVNAppMob(t *testing.T) {
	f := &compositePriceFetcher{
		vnappmob: &stubVNAppMobClient{buy: 90_000_000, sell: 91_000_000},
		fallback: &stubFallbackFetcher{err: errors.New("fallback unavailable")},
	}

	p, err := f.FetchPrice(context.Background())
	if err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	if p.Source != "vnappmob-sjc" {
		t.Fatalf("source: got %q, want vnappmob-sjc", p.Source)
	}
	if p.SJC == nil || p.SJC.Buy != 90_000_000 || p.SJC.Sell != 91_000_000 {
		t.Fatalf("SJC: got %+v", p.SJC)
	}
	wantMid := 90_500_000.0
	if p.VNDPerLuong != wantMid {
		t.Fatalf("VNDPerLuong: got %v, want %v", p.VNDPerLuong, wantMid)
	}

	mid, err := f.FetchLuongPrice(context.Background())
	if err != nil {
		t.Fatalf("FetchLuongPrice: %v", err)
	}
	if mid != wantMid {
		t.Fatalf("FetchLuongPrice: got %v, want %v", mid, wantMid)
	}

	buy, sell, err := f.FetchLuongPrices(context.Background())
	if err != nil {
		t.Fatalf("FetchLuongPrices: %v", err)
	}
	if buy != 90_000_000 || sell != 91_000_000 {
		t.Fatalf("FetchLuongPrices: got buy=%v sell=%v", buy, sell)
	}
}

func TestCompositeFetcher_FallsBack(t *testing.T) {
	f := &compositePriceFetcher{
		vnappmob: &stubVNAppMobClient{err: errors.New("vnappmob down")},
		fallback: &stubFallbackFetcher{price: 88_000_000},
	}
	p, err := f.FetchPrice(context.Background())
	if err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	if p.Source != "xau-fallback" {
		t.Fatalf("source: got %q, want xau-fallback", p.Source)
	}
	if p.VNDPerLuong != 88_000_000 {
		t.Fatalf("VNDPerLuong: got %v, want 88000000", p.VNDPerLuong)
	}

	mid, err := f.FetchLuongPrice(context.Background())
	if err != nil {
		t.Fatalf("FetchLuongPrice: %v", err)
	}
	if mid != 88_000_000 {
		t.Fatalf("FetchLuongPrice: got %v, want 88000000", mid)
	}
}

func TestGoldPriceLines(t *testing.T) {
	t.Run("sjc", func(t *testing.T) {
		lines := goldPriceLines(GoldPrice{
			VNDPerLuong: 90_500_000,
			Source:      "vnappmob-sjc",
			SJC:         &SJCPrice{Buy: 90_000_000, Sell: 91_000_000},
		})
		want := []string{"Gold Spot Price (SJC)", "Buy:", "Sell:"}
		if len(lines) != len(want) {
			t.Fatalf("got %d lines, want %d: %v", len(lines), len(want), lines)
		}
		for i, w := range want {
			if !strings.Contains(lines[i], w) {
				t.Fatalf("line %d missing %q in %v", i, w, lines)
			}
		}
	})

	t.Run("fallback", func(t *testing.T) {
		lines := goldPriceLines(GoldPrice{
			XAUUSD:      3000,
			USDVND:      25000,
			VNDPerLuong: 90_000_000,
			Source:      "xau-fallback",
		})
		want := []string{"Gold Spot Price", "XAU:", "Rate:", "VND:"}
		for i, w := range want {
			if i >= len(lines) || !strings.Contains(lines[i], w) {
				t.Fatalf("line %d missing %q in %v", i, w, lines)
			}
		}
	})
}

func TestNewCompositePriceFetcherFromEnv(t *testing.T) {
	f, ok := NewCompositePriceFetcherFromEnv(storage.NewMemoryKVStore()).(*compositePriceFetcher)
	if !ok {
		t.Fatalf("expected *compositePriceFetcher, got %T", f)
	}
	if f.vnappmob == nil {
		t.Fatal("vnappmob client is nil")
	}
	if f.fallback == nil {
		t.Fatal("fallback client is nil")
	}
}
