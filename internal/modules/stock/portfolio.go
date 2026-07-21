package stock

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/tiennm99/miti99bot/internal/storage"
)

type Store = storage.DocStore[Portfolio]

const CollectionName = "stock"

// AssetPosition keeps the complete persisted state for one stock ticker.
// Base is total remaining VND cost, not average price. DividendCheckedAt is
// the cursor for future dividend-event discovery.
type AssetPosition struct {
	Quantity          int64   `json:"quantity" bson:"quantity"`
	Base              float64 `json:"base" bson:"base"`
	DividendCheckedAt int64   `json:"dividendCheckedAt" bson:"dividendCheckedAt"`
	OpenedAt          int64   `json:"openedAt,omitempty" bson:"openedAt,omitempty"`
}

type Portfolio struct {
	VND                   float64                  `json:"vnd" bson:"vnd"`
	Assets                map[string]AssetPosition `json:"assets" bson:"assets"`
	AppliedDividendEvents map[string]int64         `json:"appliedDividendEvents,omitempty" bson:"appliedDividendEvents,omitempty"`
	Meta                  PortfolioMeta            `json:"meta" bson:"meta"`
}

type PortfolioMeta struct {
	Invested  float64 `json:"invested" bson:"invested"`
	CreatedAt int64   `json:"createdAt" bson:"createdAt"`
}

func NewPortfolio(now int64) Portfolio {
	return Portfolio{Assets: map[string]AssetPosition{}, Meta: PortfolioMeta{CreatedAt: now}}
}

func portfolioKey(userID int64) string {
	return "user:" + strconv.FormatInt(userID, 10)
}

func LoadPortfolio(ctx context.Context, store Store, userID int64, now int64) (Portfolio, error) {
	p, _, err := store.Get(ctx, portfolioKey(userID))
	switch {
	case err == nil:
		if p.Assets == nil {
			p.Assets = map[string]AssetPosition{}
		}
		if p.AppliedDividendEvents == nil {
			p.AppliedDividendEvents = map[string]int64{}
		}
		if p.Meta.CreatedAt == 0 {
			p.Meta.CreatedAt = now
		}
		if err := p.Validate(); err != nil {
			return Portfolio{}, err
		}
		return p, nil
	case errors.Is(err, storage.ErrNotFound):
		return NewPortfolio(now), nil
	default:
		return Portfolio{}, fmt.Errorf("stock: load portfolio %d: %w", userID, err)
	}
}

func SavePortfolio(ctx context.Context, store Store, userID int64, p Portfolio) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("stock: save portfolio %d: %w", userID, err)
	}
	if err := store.Put(ctx, portfolioKey(userID), p); err != nil {
		return fmt.Errorf("stock: save portfolio %d: %w", userID, err)
	}
	return nil
}

func (p Portfolio) Validate() error {
	if math.IsNaN(p.VND) || math.IsInf(p.VND, 0) || p.VND < 0 {
		return fmt.Errorf("stock: invalid VND balance")
	}
	for symbol, position := range p.Assets {
		canonical, err := normalizeStockSymbol(symbol)
		if err != nil || canonical != symbol {
			return fmt.Errorf("stock: invalid ticker %q", symbol)
		}
		if position.Quantity <= 0 || !isPositiveFiniteCost(position.Base) || position.DividendCheckedAt <= 0 || position.OpenedAt < 0 {
			return fmt.Errorf("stock: %s has invalid position", symbol)
		}
	}
	for eventID, appliedAt := range p.AppliedDividendEvents {
		if eventID == "" || len(eventID) > 128 || appliedAt <= 0 {
			return fmt.Errorf("stock: invalid applied dividend event")
		}
	}
	return nil
}

func (p *Portfolio) AddVND(amount float64) {
	p.VND += amount
}

func (p *Portfolio) DeductVND(amount float64) (ok bool, balance float64) {
	if p.VND < amount {
		return false, p.VND
	}
	p.VND -= amount
	return true, p.VND
}

// BuyTicker adds quantity and basis. DividendCheckedAt is set only when opening
// a new position so later buys cannot skip events after the original buy.
func (p *Portfolio) BuyTicker(symbol string, quantity int64, base float64, now int64) error {
	if quantity <= 0 || !isPositiveFiniteCost(base) || now <= 0 {
		return fmt.Errorf("stock: invalid purchase position")
	}
	if p.Assets == nil {
		p.Assets = map[string]AssetPosition{}
	}
	position := p.Assets[symbol]
	isOpening := position.Quantity == 0
	if position.Quantity > math.MaxInt64-quantity {
		return fmt.Errorf("stock: quantity overflows")
	}
	position.Quantity += quantity
	position.Base += base
	if !isPositiveFiniteCost(position.Base) {
		return fmt.Errorf("stock: cost basis overflows")
	}
	if position.DividendCheckedAt == 0 {
		position.DividendCheckedAt = now
	}
	if isOpening {
		position.OpenedAt = now
	}
	p.Assets[symbol] = position
	return nil
}

// SellTicker removes proportional weighted-average basis while preserving the
// dividend cursor. A full exit removes the entire ticker document.
func (p *Portfolio) SellTicker(symbol string, quantity int64) (remaining int64, soldBase float64, ok bool, err error) {
	position, exists := p.Assets[symbol]
	if !exists || position.Quantity < quantity || quantity <= 0 {
		return position.Quantity, 0, false, nil
	}
	if quantity == position.Quantity {
		delete(p.Assets, symbol)
		return 0, position.Base, true, nil
	}
	soldBase = position.Base * (float64(quantity) / float64(position.Quantity))
	position.Quantity -= quantity
	position.Base -= soldBase
	if !isPositiveFiniteCost(soldBase) || !isPositiveFiniteCost(position.Base) {
		return 0, 0, false, fmt.Errorf("stock: invalid remaining cost basis")
	}
	p.Assets[symbol] = position
	return position.Quantity, soldBase, true, nil
}

func (p *Portfolio) ApplyDividend(symbol string, quantity int64, vnd float64, now int64) error {
	position, ok := p.Assets[symbol]
	if !ok || position.Quantity <= 0 {
		return fmt.Errorf("stock: ticker position not found")
	}
	if quantity < position.Quantity || !isPositiveFiniteCost(position.Base) || now <= 0 {
		return fmt.Errorf("stock: invalid dividend position")
	}
	position.Quantity = quantity
	position.DividendCheckedAt = now
	p.Assets[symbol] = position
	p.VND = vnd
	return nil
}

func isPositiveFiniteCost(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
