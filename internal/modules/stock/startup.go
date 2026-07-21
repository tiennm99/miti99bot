package stock

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

const (
	CollectionName         = "stock"
	assetSchemaMarkerKey   = "migration:stock-asset-schema-v2"
	tickerMigrationRetries = 5
)

type legacyPortfolio struct {
	VND       float64                  `json:"vnd,omitempty" bson:"vnd,omitempty"`
	Currency  map[string]float64       `json:"currency,omitempty" bson:"currency,omitempty"`
	Assets    map[string]AssetPosition `json:"assets" bson:"assets"`
	CostBasis map[string]float64       `json:"costBasis,omitempty" bson:"costBasis,omitempty"`
	Meta      PortfolioMeta            `json:"meta" bson:"meta"`
}

// InitStore migrates the old currency/assets/costBasis shape into the nested
// asset schema before handlers can load portfolios. It remains an every-boot
// invariant scan; the system marker is audit history, not a bypass.
func InitStore(ctx context.Context, portfolioColl, systemColl storage.Collection) error {
	docs := storage.Typed[legacyPortfolio](portfolioColl)
	system := systemstate.New(systemColl)
	marker, markerExists, err := system.Get(ctx, assetSchemaMarkerKey)
	if err != nil {
		return fmt.Errorf("stock asset schema migration: read marker: %w", err)
	}
	keys, err := docs.List(ctx, "user:")
	if err != nil {
		return fmt.Errorf("stock asset schema migration: list portfolios: %w", err)
	}
	var migrated int64
	for index, key := range keys {
		changed, err := migrateAssetSchema(ctx, docs, key, time.Now().UnixMilli())
		if err != nil {
			return err
		}
		if changed {
			migrated++
			log.Info("stock asset schema migrated", "portfolio", index+1, "total", len(keys))
		}
	}
	now := time.Now().UnixMilli()
	if markerExists && marker.Status == "completed" && migrated == 0 {
		return nil
	}
	if !markerExists {
		marker = systemstate.Record{Kind: "migration", Name: "stock asset schema v2", CompletedAt: now}
	}
	marker.Status = "completed"
	marker.Count += migrated
	marker.UpdatedAt = now
	if err := system.Put(ctx, assetSchemaMarkerKey, marker); err != nil {
		return fmt.Errorf("stock asset schema migration: write marker: %w", err)
	}
	return nil
}

func migrateAssetSchema(ctx context.Context, docs storage.DocStore[legacyPortfolio], key string, now int64) (bool, error) {
	for attempt := 0; attempt < tickerMigrationRetries; attempt++ {
		doc, version, err := docs.Get(ctx, key)
		if err != nil {
			return false, fmt.Errorf("stock asset schema migration: read %s: %w", key, err)
		}
		changed, err := doc.migrate(now)
		if err != nil {
			return false, fmt.Errorf("stock asset schema migration: %s: %w", key, err)
		}
		if !changed {
			return false, nil
		}
		if err := docs.PutVersioned(ctx, key, version, doc); err == nil {
			return true, nil
		} else if !errors.Is(err, storage.ErrConflict) {
			return false, fmt.Errorf("stock asset schema migration: write %s: %w", key, err)
		}
	}
	return false, fmt.Errorf("stock asset schema migration: write %s: %w", key, storage.ErrConflict)
}

func (p *legacyPortfolio) migrate(now int64) (bool, error) {
	if !p.hasLegacyFields() {
		return false, p.currentPortfolio().Validate()
	}
	if err := p.migrateVND(); err != nil {
		return false, err
	}
	if err := p.migrateAssets(now); err != nil {
		return false, err
	}
	p.Currency = nil
	p.CostBasis = nil
	return true, p.currentPortfolio().Validate()
}

func (p *legacyPortfolio) hasLegacyFields() bool {
	if p.Currency != nil || p.CostBasis != nil {
		return true
	}
	for _, position := range p.Assets {
		if position.legacyQuantity {
			return true
		}
	}
	return false
}

func (p *legacyPortfolio) migrateVND() error {
	if math.IsNaN(p.VND) || math.IsInf(p.VND, 0) || p.VND < 0 {
		return fmt.Errorf("invalid VND balance")
	}
	for currency, balance := range p.Currency {
		if currency != "VND" && balance != 0 {
			return fmt.Errorf("unsupported legacy currency %q", currency)
		}
	}
	if legacyVND := p.Currency["VND"]; legacyVND != 0 {
		if p.VND != 0 && p.VND != legacyVND {
			return fmt.Errorf("conflicting VND balances")
		}
		p.VND = legacyVND
	}
	return nil
}

func (p *legacyPortfolio) migrateAssets(now int64) error {
	for symbol, position := range p.Assets {
		if !position.legacyQuantity {
			return fmt.Errorf("document mixes legacy and nested assets")
		}
		if position.Quantity == 0 {
			delete(p.Assets, symbol)
			continue
		}
		canonical, err := normalizeStockSymbol(symbol)
		base := p.CostBasis[symbol]
		if err != nil || canonical != symbol || position.Quantity < 0 || !isPositiveFiniteCost(base) {
			return fmt.Errorf("invalid legacy position %q", symbol)
		}
		p.Assets[symbol] = AssetPosition{Quantity: position.Quantity, Base: base, DividendCheckedAt: now}
	}
	for symbol, base := range p.CostBasis {
		if !isPositiveFiniteCost(base) || p.Assets[symbol].Quantity <= 0 {
			return fmt.Errorf("orphan or invalid legacy basis %q", symbol)
		}
	}
	return nil
}

func (p *legacyPortfolio) currentPortfolio() Portfolio {
	return Portfolio{VND: p.VND, Assets: p.Assets, Meta: p.Meta}
}
