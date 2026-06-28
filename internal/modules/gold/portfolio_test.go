package gold

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"

	stockmod "github.com/tiennm99/miti99bot/internal/modules/stock"
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
		competing.AddVND(10)
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
		p.AddVND(5)
		return nil
	})
	if err != nil {
		t.Fatalf("UpdatePortfolio: %v", err)
	}
	if got.VND != 15 {
		t.Fatalf("updated portfolio VND = %v, want 15", got.VND)
	}
	loaded, err := LoadPortfolio(ctx, kv, 7, 1)
	if err != nil {
		t.Fatalf("LoadPortfolio: %v", err)
	}
	if loaded.VND != 15 {
		t.Fatalf("stored portfolio VND = %v, want 15", loaded.VND)
	}
}

type alwaysConflictStore struct {
	storage.KVStore
	attempts int
}

func (s *alwaysConflictStore) GetVersioned(ctx context.Context, key string) ([]byte, int64, error) {
	return s.KVStore.(storage.VersionedStore).GetVersioned(ctx, key)
}

func (s *alwaysConflictStore) PutVersioned(context.Context, string, int64, []byte) error {
	s.attempts++
	return storage.ErrConflict
}

func TestUpdatePortfolioReturnsConflictAfterExhaustingRetries(t *testing.T) {
	ctx := context.Background()
	kv := &alwaysConflictStore{KVStore: storage.NewMemoryKVStore()}
	_, err := UpdatePortfolio(ctx, kv, 7, 1, func(p *Portfolio) error {
		p.AddVND(5)
		return nil
	})
	if !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("UpdatePortfolio: got %v, want wrapped ErrConflict", err)
	}
	if kv.attempts != portfolioUpdateAttempts {
		t.Errorf("CAS attempts = %d, want %d", kv.attempts, portfolioUpdateAttempts)
	}
	if _, err := kv.Get(ctx, "user:7"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("exhausted update must not persist anything, Get = %v", err)
	}
}

type countingCASStore struct {
	*storage.MemoryKVStore
	casCalls int
}

func (s *countingCASStore) PutVersioned(ctx context.Context, key string, expectedVersion int64, val []byte) error {
	s.casCalls++
	return s.MemoryKVStore.PutVersioned(ctx, key, expectedVersion, val)
}

func TestUpdatePortfolioMutateErrorDoesNotRetryOrPersist(t *testing.T) {
	ctx := context.Background()
	kv := &countingCASStore{MemoryKVStore: storage.NewMemoryKVStore()}
	mutateCalls := 0
	_, err := UpdatePortfolio(ctx, kv, 7, 1, func(p *Portfolio) error {
		mutateCalls++
		return errInsufficientVND
	})
	if !errors.Is(err, errInsufficientVND) {
		t.Fatalf("UpdatePortfolio: got %v, want errInsufficientVND", err)
	}
	if mutateCalls != 1 {
		t.Errorf("mutate calls = %d, want 1 (business errors must not retry)", mutateCalls)
	}
	if kv.casCalls != 0 {
		t.Errorf("CAS calls = %d, want 0 (failed mutate must not write)", kv.casCalls)
	}
	if _, err := kv.Get(ctx, "user:7"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("failed mutate must not persist anything, Get = %v", err)
	}
}

func TestUpdatePortfolioConcurrentIncrementsLoseNoUpdates(t *testing.T) {
	ctx := context.Background()
	kv := storage.NewMemoryKVStore()
	const goroutines = 16
	var successes atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := UpdatePortfolio(ctx, kv, 7, 1, func(p *Portfolio) error {
				p.AddVND(1000)
				return nil
			})
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, storage.ErrConflict) {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("UpdatePortfolio: %v", err)
	}
	// Retry exhaustion under heavy contention is acceptable; a lost update is not:
	// the stored balance must equal exactly 1000 per successful update.
	if successes.Load() == 0 {
		t.Fatal("no update succeeded")
	}
	loaded, err := LoadPortfolio(ctx, kv, 7, 1)
	if err != nil {
		t.Fatalf("LoadPortfolio: %v", err)
	}
	if want := float64(successes.Load()) * 1000; loaded.VND != want {
		t.Errorf("VND = %v, want %v (%d successful updates)", loaded.VND, want, successes.Load())
	}
}

// plainKVStore hides the embedded store's versioned methods to model a backend
// without optimistic-locking support.
type plainKVStore struct {
	storage.KVStore
}

func TestUpdatePortfolioFailsFastWhenCASUnsupported(t *testing.T) {
	ctx := context.Background()
	mutate := func(p *Portfolio) error {
		p.AddVND(5)
		return nil
	}

	// A bare non-CAS store is rejected by the capability check.
	if _, err := UpdatePortfolio(ctx, &plainKVStore{KVStore: storage.NewMemoryKVStore()}, 7, 1, mutate); err == nil {
		t.Error("UpdatePortfolio on non-CAS store: want error, got nil")
	}

	// A prefixed wrapper around a non-CAS store must fail fast as unsupported,
	// not burn retries on a fake conflict.
	_, err := UpdatePortfolio(ctx, storage.Prefixed(&plainKVStore{KVStore: storage.NewMemoryKVStore()}, "gold"), 7, 1, mutate)
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("got %v, want errors.ErrUnsupported", err)
	}
	if errors.Is(err, storage.ErrConflict) {
		t.Error("missing CAS capability must not be reported as a write conflict")
	}
}

func TestStockAndGoldPortfolioKeysDoNotCollide(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	goldPortfolio := NewPortfolio(1)
	goldPortfolio.AddLuong(2)
	if err := SavePortfolio(ctx, provider.For("gold"), 7, goldPortfolio); err != nil {
		t.Fatalf("save gold: %v", err)
	}
	stockPortfolio := stockmod.NewPortfolio(1)
	stockPortfolio.AddAsset("TCB", 100)
	if err := stockmod.SavePortfolio(ctx, provider.For("stock"), 7, stockPortfolio); err != nil {
		t.Fatalf("save stock: %v", err)
	}
	keys, err := provider.Base().List(ctx, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := map[string]bool{"gold:user:7": false, "stock:user:7": false}
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
