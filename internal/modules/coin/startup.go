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

const (
	CollectionName         = "coin"
	assetSchemaMarkerKey   = "migration:coin-asset-schema-v2"
	tickerMigrationRetries = 5
)

type legacyPortfolio struct {
	USD       float64                  `json:"usd" bson:"usd"`
	Assets    map[string]AssetPosition `json:"assets" bson:"assets"`
	CostBasis map[string]float64       `json:"costBasis,omitempty" bson:"costBasis,omitempty"`
	Meta      PortfolioMeta            `json:"meta" bson:"meta"`
}

func InitStore(ctx context.Context, portfolioColl, systemColl storage.Collection) error {
	docs := storage.Typed[legacyPortfolio](portfolioColl)
	system := systemstate.New(systemColl)
	marker, markerExists, err := system.Get(ctx, assetSchemaMarkerKey)
	if err != nil {
		return fmt.Errorf("coin asset schema migration: read marker: %w", err)
	}
	keys, err := docs.List(ctx, "user:")
	if err != nil {
		return fmt.Errorf("coin asset schema migration: list portfolios: %w", err)
	}
	var migrated int64
	for index, key := range keys {
		changed, err := migrateAssetSchema(ctx, docs, key, time.Now().UnixMilli())
		if err != nil {
			return err
		}
		if changed {
			migrated++
			log.Info("coin asset schema migrated", "portfolio", index+1, "total", len(keys))
		}
	}
	now := time.Now().UnixMilli()
	if markerExists && marker.Status == "completed" && migrated == 0 {
		return nil
	}
	if !markerExists {
		marker = systemstate.Record{Kind: "migration", Name: "coin asset schema v2", CompletedAt: now}
	}
	marker.Status = "completed"
	marker.Count += migrated
	marker.UpdatedAt = now
	if err := system.Put(ctx, assetSchemaMarkerKey, marker); err != nil {
		return fmt.Errorf("coin asset schema migration: write marker: %w", err)
	}
	return nil
}

func migrateAssetSchema(ctx context.Context, docs storage.DocStore[legacyPortfolio], key string, now int64) (bool, error) {
	for attempt := 0; attempt < tickerMigrationRetries; attempt++ {
		doc, version, err := docs.Get(ctx, key)
		if err != nil {
			return false, fmt.Errorf("coin asset schema migration: read %s: %w", key, err)
		}
		changed, err := doc.migrate(now)
		if err != nil {
			return false, fmt.Errorf("coin asset schema migration: %s: %w", key, err)
		}
		if !changed {
			return false, nil
		}
		if err := docs.PutVersioned(ctx, key, version, doc); err == nil {
			return true, nil
		} else if !errors.Is(err, storage.ErrConflict) {
			return false, fmt.Errorf("coin asset schema migration: write %s: %w", key, err)
		}
	}
	return false, fmt.Errorf("coin asset schema migration: write %s: %w", key, storage.ErrConflict)
}

func (p *legacyPortfolio) migrate(now int64) (bool, error) {
	hasLegacyQuantity := false
	for _, position := range p.Assets {
		hasLegacyQuantity = hasLegacyQuantity || position.legacyQuantity
	}
	hasLegacy := p.CostBasis != nil || hasLegacyQuantity
	if !hasLegacy {
		return false, Portfolio{USD: p.USD, Assets: p.Assets, Meta: p.Meta}.Validate()
	}
	for symbol, position := range p.Assets {
		if !position.legacyQuantity {
			return false, fmt.Errorf("document mixes legacy and nested assets")
		}
		if position.Quantity == 0 {
			delete(p.Assets, symbol)
			continue
		}
		coin, err := ResolveCoinSymbol(symbol)
		base := p.CostBasis[symbol]
		if err != nil || coin.Symbol != symbol || !isPositiveFinite(position.Quantity) || !isPositiveFinite(base) {
			return false, fmt.Errorf("invalid legacy position %q", symbol)
		}
		p.Assets[symbol] = AssetPosition{Quantity: position.Quantity, Base: base, DividendCheckedAt: now}
	}
	for symbol, base := range p.CostBasis {
		if !isPositiveFinite(base) || p.Assets[symbol].Quantity <= 0 {
			return false, fmt.Errorf("orphan or invalid legacy basis %q", symbol)
		}
	}
	p.CostBasis = nil
	return true, Portfolio{USD: p.USD, Assets: p.Assets, Meta: p.Meta}.Validate()
}
