package coin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

const coinDividendCursorCleanupMarker = "migration:coin-remove-dividend-checked-at-v1"

// dividendCursorCleanupPosition exists only to detect the stale field that an
// earlier completed migration persisted on coin assets. A pointer distinguishes
// an absent field from a present zero value so clean rows are never rewritten.
type dividendCursorCleanupPosition struct {
	Quantity          float64 `json:"quantity" bson:"quantity"`
	Base              float64 `json:"base" bson:"base"`
	DividendCheckedAt *int64  `json:"dividendCheckedAt,omitempty" bson:"dividendCheckedAt,omitempty"`
}

type dividendCursorCleanupPortfolio struct {
	USD    float64                                  `json:"usd" bson:"usd"`
	Assets map[string]dividendCursorCleanupPosition `json:"assets" bson:"assets"`
	Meta   PortfolioMeta                            `json:"meta" bson:"meta"`
}

// InitStore removes the stock-only dividend cursor accidentally persisted in
// legacy coin positions. The completion marker makes the scan a one-time task;
// each affected row uses optimistic concurrency and whole-document replacement.
func InitStore(ctx context.Context, coinColl, systemColl storage.Collection) error {
	system := systemstate.New(systemColl)
	marker, exists, err := system.Get(ctx, coinDividendCursorCleanupMarker)
	if err != nil {
		return fmt.Errorf("coin dividend cursor cleanup: read marker: %w", err)
	}
	if exists && marker.Status == "completed" {
		return nil
	}

	legacyDocs := storage.Typed[dividendCursorCleanupPortfolio](coinColl)
	currentDocs := storage.Typed[Portfolio](coinColl)
	keys, err := legacyDocs.List(ctx, "user:")
	if err != nil {
		return fmt.Errorf("coin dividend cursor cleanup: list portfolios: %w", err)
	}
	var cleaned int64
	for _, key := range keys {
		changed, cleanupErr := cleanupCoinDividendCursor(ctx, legacyDocs, currentDocs, key)
		if cleanupErr != nil {
			return cleanupErr
		}
		if changed {
			cleaned++
		}
	}

	now := time.Now().UnixMilli()
	if !exists {
		marker = systemstate.Record{Kind: "migration", Name: "coin remove dividendCheckedAt v1"}
	}
	marker.Status = "completed"
	marker.Count += cleaned
	marker.CompletedAt = now
	marker.UpdatedAt = now
	if err := system.Put(ctx, coinDividendCursorCleanupMarker, marker); err != nil {
		return fmt.Errorf("coin dividend cursor cleanup: write marker: %w", err)
	}
	log.Info("coin_dividend_cursor_cleanup_completed", "portfolios", cleaned)
	return nil
}

func cleanupCoinDividendCursor(
	ctx context.Context,
	legacyDocs storage.DocStore[dividendCursorCleanupPortfolio],
	currentDocs storage.DocStore[Portfolio],
	key string,
) (bool, error) {
	for attempt := 0; attempt < portfolioUpdateAttempts; attempt++ {
		legacy, version, err := legacyDocs.Get(ctx, key)
		if err != nil {
			return false, fmt.Errorf("coin dividend cursor cleanup: read %s: %w", key, err)
		}
		current, stale := legacy.currentPortfolio()
		if !stale {
			return false, nil
		}
		err = currentDocs.PutVersioned(ctx, key, version, current)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, storage.ErrConflict) {
			return false, fmt.Errorf("coin dividend cursor cleanup: write %s: %w", key, err)
		}
	}
	return false, fmt.Errorf("coin dividend cursor cleanup: write %s: %w after %d attempts", key, storage.ErrConflict, portfolioUpdateAttempts)
}

func (p dividendCursorCleanupPortfolio) currentPortfolio() (Portfolio, bool) {
	assets := make(map[string]AssetPosition, len(p.Assets))
	stale := false
	for symbol, position := range p.Assets {
		assets[symbol] = AssetPosition{Quantity: position.Quantity, Base: position.Base}
		stale = stale || position.DividendCheckedAt != nil
	}
	return Portfolio{USD: p.USD, Assets: assets, Meta: p.Meta}, stale
}
