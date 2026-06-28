package coin

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
)

func TestLoadPortfolioFirstTimeUser(t *testing.T) {
	p, err := LoadPortfolio(context.Background(), storage.NewMemoryKVStore(), 42, 123)
	if err != nil {
		t.Fatalf("LoadPortfolio: %v", err)
	}
	if p.USD != 0 || len(p.Assets) != 0 || p.Meta.CreatedAt != 123 {
		t.Fatalf("portfolio = %+v", p)
	}
}

func TestPortfolioBuySellMath(t *testing.T) {
	p := NewPortfolio(1)
	p.AddUSD(1000)
	p.Meta.Invested = 1000
	if ok, bal := p.DeductUSD(250); !ok || bal != 750 {
		t.Fatalf("DeductUSD ok=%v bal=%v", ok, bal)
	}
	p.AddAsset("BTC", 0.1)
	if ok, held := p.DeductAsset("BTC", 0.04); !ok || math.Abs(held-0.06) > 1e-12 {
		t.Fatalf("DeductAsset ok=%v held=%v", ok, held)
	}
	if p.Assets["BTC"] <= 0 {
		t.Fatalf("BTC holding missing: %+v", p.Assets)
	}
}

func TestDeductInsufficientBalances(t *testing.T) {
	p := NewPortfolio(1)
	p.AddUSD(10)
	p.AddAsset("ETH", 0.5)
	if ok, bal := p.DeductUSD(11); ok || bal != 10 || p.USD != 10 {
		t.Fatalf("DeductUSD ok=%v bal=%v p=%+v", ok, bal, p)
	}
	if ok, held := p.DeductAsset("ETH", 0.6); ok || held != 0.5 || p.Assets["ETH"] != 0.5 {
		t.Fatalf("DeductAsset ok=%v held=%v p=%+v", ok, held, p)
	}
}

func TestDeductAssetDustCleanup(t *testing.T) {
	p := NewPortfolio(1)
	p.AddAsset("BTC", 0.1)
	p.AddAsset("BTC", 0.2)
	if ok, _ := p.DeductAsset("BTC", 0.3); !ok {
		t.Fatal("DeductAsset ok=false")
	}
	if _, ok := p.Assets["BTC"]; ok {
		t.Fatalf("dust key not removed: %+v", p.Assets)
	}
}

func TestNormalizeAmountSpecialValues(t *testing.T) {
	for _, n := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), coinDustEpsilon / 2} {
		if got := normalizeAmount(n); got != 0 {
			t.Fatalf("normalizeAmount(%v) = %v, want 0", n, got)
		}
	}
}

type conflictOnceStore struct {
	storage.KVStore
	conflicted bool
}

func (s *conflictOnceStore) GetVersioned(ctx context.Context, key string) ([]byte, int64, error) {
	return s.KVStore.(storage.VersionedStore).GetVersioned(ctx, key)
}

func (s *conflictOnceStore) PutVersioned(ctx context.Context, key string, expectedVersion int64, val []byte) error {
	if !s.conflicted {
		s.conflicted = true
		competing := NewPortfolio(1)
		competing.AddUSD(10)
		if err := s.PutJSON(ctx, key, competing); err != nil {
			return err
		}
		return storage.ErrConflict
	}
	return s.KVStore.(storage.VersionedStore).PutVersioned(ctx, key, expectedVersion, val)
}

func TestUpdatePortfolioRetriesAfterWriteConflict(t *testing.T) {
	ctx := context.Background()
	kv := &conflictOnceStore{KVStore: storage.NewMemoryKVStore()}
	got, err := UpdatePortfolio(ctx, kv, 7, 1, func(p *Portfolio) error {
		p.AddUSD(5)
		return nil
	})
	if err != nil {
		t.Fatalf("UpdatePortfolio: %v", err)
	}
	if got.USD != 15 {
		t.Fatalf("USD = %v, want 15", got.USD)
	}
}

func TestUpdatePortfolioMutateErrorDoesNotPersist(t *testing.T) {
	ctx := context.Background()
	kv := storage.NewMemoryKVStore()
	_, err := UpdatePortfolio(ctx, kv, 7, 1, func(p *Portfolio) error {
		return errInsufficientUSD
	})
	if !errors.Is(err, errInsufficientUSD) {
		t.Fatalf("got %v, want errInsufficientUSD", err)
	}
	if _, err := kv.Get(ctx, "user:7"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("failed mutate must not persist, Get = %v", err)
	}
}
