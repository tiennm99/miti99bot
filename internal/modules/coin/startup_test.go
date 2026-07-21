package coin

import (
	"context"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

func int64Pointer(value int64) *int64 { return &value }

func TestInitStoreRemovesStaleDividendCursorAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	coinColl := provider.Collection(CollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	legacyDocs := storage.Typed[dividendCursorCleanupPortfolio](coinColl)
	currentDocs := storage.Typed[Portfolio](coinColl)

	legacy := dividendCursorCleanupPortfolio{
		USD: 75,
		Assets: map[string]dividendCursorCleanupPosition{
			"BTC": {Quantity: 0.5, Base: 25_000, DividendCheckedAt: int64Pointer(123)},
		},
		Meta: PortfolioMeta{Invested: 100, CreatedAt: 1},
	}
	if err := legacyDocs.Put(ctx, "user:7", legacy); err != nil {
		t.Fatal(err)
	}
	clean := Portfolio{USD: 10, Assets: map[string]AssetPosition{"ETH": {Quantity: 1, Base: 2_000}}, Meta: PortfolioMeta{Invested: 10, CreatedAt: 2}}
	if err := currentDocs.Put(ctx, "user:8", clean); err != nil {
		t.Fatal(err)
	}
	_, cleanVersionBefore, _ := currentDocs.Get(ctx, "user:8")

	if err := InitStore(ctx, coinColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	got, migratedVersion, err := currentDocs.Get(ctx, "user:7")
	if err != nil {
		t.Fatal(err)
	}
	if got.USD != 75 || got.Meta.Invested != 100 || got.Meta.CreatedAt != 1 || got.Assets["BTC"].Quantity != 0.5 || got.Assets["BTC"].Base != 25_000 {
		t.Fatalf("cleanup changed business data: %+v", got)
	}
	rawShape, _, err := legacyDocs.Get(ctx, "user:7")
	if err != nil || rawShape.Assets["BTC"].DividendCheckedAt != nil {
		t.Fatalf("stale cursor remains after cleanup: %+v err=%v", rawShape, err)
	}
	_, cleanVersionAfter, _ := currentDocs.Get(ctx, "user:8")
	if cleanVersionAfter != cleanVersionBefore {
		t.Fatalf("clean portfolio version changed: before=%d after=%d", cleanVersionBefore, cleanVersionAfter)
	}
	marker, exists, err := systemstate.New(systemColl).Get(ctx, coinDividendCursorCleanupMarker)
	if err != nil || !exists || marker.Status != "completed" || marker.Count != 1 {
		t.Fatalf("marker=%+v exists=%v err=%v", marker, exists, err)
	}

	if err := InitStore(ctx, coinColl, systemColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}
	_, versionAfterSecondRun, _ := currentDocs.Get(ctx, "user:7")
	if versionAfterSecondRun != migratedVersion {
		t.Fatalf("second run rewrote portfolio: first=%d second=%d", migratedVersion, versionAfterSecondRun)
	}
}

func TestInitStoreDoesNotRewriteUnrelatedInvalidPortfolio(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	coinColl := provider.Collection(CollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	legacyDocs := storage.Typed[dividendCursorCleanupPortfolio](coinColl)
	if err := legacyDocs.Put(ctx, "user:7", dividendCursorCleanupPortfolio{
		Assets: map[string]dividendCursorCleanupPosition{
			"BTC": {Quantity: -1, Base: 1},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, versionBefore, _ := legacyDocs.Get(ctx, "user:7")
	if err := InitStore(ctx, coinColl, systemColl); err != nil {
		t.Fatalf("unrelated invalid portfolio blocked cleanup: %v", err)
	}
	_, versionAfter, _ := legacyDocs.Get(ctx, "user:7")
	if versionAfter != versionBefore {
		t.Fatalf("unrelated portfolio was rewritten: before=%d after=%d", versionBefore, versionAfter)
	}
	marker, exists, err := systemstate.New(systemColl).Get(ctx, coinDividendCursorCleanupMarker)
	if err != nil || !exists || marker.Status != "completed" || marker.Count != 0 {
		t.Fatalf("marker=%+v exists=%v err=%v", marker, exists, err)
	}
}

type cleanupConflictOnceStore struct {
	storage.DocStore[Portfolio]
	legacy     storage.DocStore[dividendCursorCleanupPortfolio]
	conflicted bool
}

func (s *cleanupConflictOnceStore) PutVersioned(ctx context.Context, key string, version int64, portfolio Portfolio) error {
	if !s.conflicted {
		s.conflicted = true
		concurrent, concurrentVersion, err := s.legacy.Get(ctx, key)
		if err != nil {
			return err
		}
		concurrent.USD += 5
		if err := s.legacy.PutVersioned(ctx, key, concurrentVersion, concurrent); err != nil {
			return err
		}
		return storage.ErrConflict
	}
	return s.DocStore.PutVersioned(ctx, key, version, portfolio)
}

func TestCleanupDividendCursorRetriesConflict(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	coll := provider.Collection(CollectionName)
	legacyDocs := storage.Typed[dividendCursorCleanupPortfolio](coll)
	if err := legacyDocs.Put(ctx, "user:7", dividendCursorCleanupPortfolio{
		Assets: map[string]dividendCursorCleanupPosition{"BTC": {Quantity: 1, Base: 10, DividendCheckedAt: int64Pointer(1)}},
		Meta:   PortfolioMeta{CreatedAt: 1},
	}); err != nil {
		t.Fatal(err)
	}
	current := &cleanupConflictOnceStore{DocStore: storage.Typed[Portfolio](coll), legacy: legacyDocs}
	changed, err := cleanupCoinDividendCursor(ctx, legacyDocs, current, "user:7")
	if err != nil || !changed || !current.conflicted {
		t.Fatalf("changed=%v conflicted=%v err=%v", changed, current.conflicted, err)
	}
	shape, _, err := legacyDocs.Get(ctx, "user:7")
	if err != nil || shape.Assets["BTC"].DividendCheckedAt != nil || shape.USD != 5 {
		t.Fatalf("cleanup lost concurrent update or left cursor: %+v err=%v", shape, err)
	}
}
