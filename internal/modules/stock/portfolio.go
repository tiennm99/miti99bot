package stock

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/tiennm99/miti99bot/internal/storage"
)

// Store is the stock module's typed portfolio store.
type Store = storage.DocStore[Portfolio]

// Portfolio is the per-user stock state. Currency is a map for forward-
// compat with USD/EUR (currently VND-only). Assets is a flat ticker→qty map.
type Portfolio struct {
	Currency  map[string]float64 `json:"currency" bson:"currency"`
	Assets    map[string]int64   `json:"assets" bson:"assets"`
	CostBasis map[string]float64 `json:"costBasis" bson:"costBasis"`
	Meta      PortfolioMeta      `json:"meta" bson:"meta"`
}

// PortfolioMeta tracks invested cost basis for P&L. CreatedAt is purely
// informational (ms-epoch when the portfolio first existed).
type PortfolioMeta struct {
	Invested  float64 `json:"invested" bson:"invested"`
	CreatedAt int64   `json:"createdAt" bson:"createdAt"`
}

// NewPortfolio returns an empty starting state. Currency map seeded with VND=0
// so deductCurrency on a fresh user reports "0 balance" cleanly instead of
// nil-map panics.
func NewPortfolio(now int64) Portfolio {
	return Portfolio{
		Currency:  map[string]float64{"VND": 0},
		Assets:    map[string]int64{},
		CostBasis: map[string]float64{},
		Meta:      PortfolioMeta{Invested: 0, CreatedAt: now},
	}
}

func portfolioKey(userID int64) string {
	return "user:" + strconv.FormatInt(userID, 10)
}

// LoadPortfolio reads from store; returns an empty portfolio on first-time use.
// Defensively initialises nil maps so callers never need a nil check.
func LoadPortfolio(ctx context.Context, store Store, userID int64, now int64) (Portfolio, error) {
	p, _, err := store.Get(ctx, portfolioKey(userID))
	switch {
	case err == nil:
		// Repair any nils from older / partial saves — defence in depth.
		if p.Currency == nil {
			p.Currency = map[string]float64{"VND": 0}
		} else if _, ok := p.Currency["VND"]; !ok {
			p.Currency["VND"] = 0
		}
		if p.Assets == nil {
			p.Assets = map[string]int64{}
		}
		if p.CostBasis == nil {
			p.CostBasis = map[string]float64{}
		}
		if err := p.ValidateCostBasis(); err != nil {
			return Portfolio{}, err
		}
		return p, nil
	case errors.Is(err, storage.ErrNotFound):
		return NewPortfolio(now), nil
	default:
		return Portfolio{}, fmt.Errorf("stock: load portfolio %d: %w", userID, err)
	}
}

func (p *Portfolio) AddCostBasis(symbol string, amount float64) error {
	if !isPositiveFiniteCost(amount) {
		return fmt.Errorf("stock: invalid purchase cost basis")
	}
	if p.CostBasis == nil {
		p.CostBasis = map[string]float64{}
	}
	next := p.CostBasis[symbol] + amount
	if !isPositiveFiniteCost(next) {
		return fmt.Errorf("stock: cost basis overflows")
	}
	p.CostBasis[symbol] = next
	return nil
}

func (p *Portfolio) RemoveCostBasis(symbol string, sold, held int64) (float64, error) {
	if sold <= 0 || held <= 0 || sold > held {
		return 0, fmt.Errorf("stock: invalid cost basis quantities")
	}
	basis := p.CostBasis[symbol]
	if !isPositiveFiniteCost(basis) {
		return 0, fmt.Errorf("stock: missing cost basis for %s", symbol)
	}
	if sold == held {
		delete(p.CostBasis, symbol)
		return basis, nil
	}
	removed := basis * (float64(sold) / float64(held))
	remaining := basis - removed
	if !isPositiveFiniteCost(removed) || !isPositiveFiniteCost(remaining) {
		return 0, fmt.Errorf("stock: invalid remaining cost basis")
	}
	p.CostBasis[symbol] = remaining
	return removed, nil
}

func (p Portfolio) ValidateCostBasis() error {
	for symbol, basis := range p.CostBasis {
		if !isPositiveFiniteCost(basis) {
			return fmt.Errorf("stock: %s has invalid cost basis", symbol)
		}
		if p.Assets[symbol] <= 0 {
			return fmt.Errorf("stock: %s has cost basis without a holding", symbol)
		}
	}
	for symbol, qty := range p.Assets {
		if qty <= 0 {
			continue
		}
		if !isPositiveFiniteCost(p.CostBasis[symbol]) {
			return fmt.Errorf("stock: holding %s has missing or invalid cost basis", symbol)
		}
	}
	return nil
}

func isPositiveFiniteCost(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// SavePortfolio persists the portfolio.
func SavePortfolio(ctx context.Context, store Store, userID int64, p Portfolio) error {
	if err := p.ValidateCostBasis(); err != nil {
		return fmt.Errorf("stock: save portfolio %d: %w", userID, err)
	}
	if err := store.Put(ctx, portfolioKey(userID), p); err != nil {
		return fmt.Errorf("stock: save portfolio %d: %w", userID, err)
	}
	return nil
}

// AddCurrency credits the currency balance.
func (p *Portfolio) AddCurrency(currency string, amount float64) {
	if p.Currency == nil {
		p.Currency = map[string]float64{}
	}
	p.Currency[currency] += amount
}

// DeductCurrency debits the currency balance. Returns false + the current
// balance when insufficient — caller renders the user-facing error.
func (p *Portfolio) DeductCurrency(currency string, amount float64) (ok bool, balance float64) {
	if p.Currency == nil {
		p.Currency = map[string]float64{}
	}
	balance = p.Currency[currency]
	if balance < amount {
		return false, balance
	}
	p.Currency[currency] = balance - amount
	return true, p.Currency[currency]
}

// AddAsset credits the share holding.
func (p *Portfolio) AddAsset(symbol string, qty int64) {
	if p.Assets == nil {
		p.Assets = map[string]int64{}
	}
	p.Assets[symbol] += qty
}

// DeductAsset debits the share holding. Returns false + held when caller asks
// for more than they own. Removes the key when balance hits zero so the
// portfolio doesn't accumulate empty entries.
func (p *Portfolio) DeductAsset(symbol string, qty int64) (ok bool, held int64) {
	if p.Assets == nil {
		p.Assets = map[string]int64{}
	}
	held = p.Assets[symbol]
	if held < qty {
		return false, held
	}
	remaining := held - qty
	if remaining == 0 {
		delete(p.Assets, symbol)
	} else {
		p.Assets[symbol] = remaining
	}
	return true, remaining
}
