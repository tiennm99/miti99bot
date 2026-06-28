package coin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/tiennm99/miti99bot/internal/storage"
)

const coinDustEpsilon = 1e-9
const portfolioUpdateAttempts = 5

type Portfolio struct {
	USD    float64            `json:"usd"`
	Assets map[string]float64 `json:"assets"`
	Meta   PortfolioMeta      `json:"meta"`
}

type PortfolioMeta struct {
	Invested  float64 `json:"invested"`
	CreatedAt int64   `json:"createdAt"`
}

func NewPortfolio(now int64) Portfolio {
	return Portfolio{Assets: map[string]float64{}, Meta: PortfolioMeta{CreatedAt: now}}
}

func portfolioKey(userID int64) string {
	return "user:" + strconv.FormatInt(userID, 10)
}

func LoadPortfolio(ctx context.Context, kv storage.KVStore, userID int64, now int64) (Portfolio, error) {
	var p Portfolio
	err := kv.GetJSON(ctx, portfolioKey(userID), &p)
	switch {
	case err == nil:
		p.normalize()
		if p.Assets == nil {
			p.Assets = map[string]float64{}
		}
		if p.Meta.CreatedAt == 0 {
			p.Meta.CreatedAt = now
		}
		return p, nil
	case errors.Is(err, storage.ErrNotFound):
		return NewPortfolio(now), nil
	default:
		return Portfolio{}, fmt.Errorf("coin: load portfolio %d: %w", userID, err)
	}
}

func SavePortfolio(ctx context.Context, kv storage.KVStore, userID int64, p Portfolio) error {
	p.normalize()
	if err := kv.PutJSON(ctx, portfolioKey(userID), p); err != nil {
		return fmt.Errorf("coin: save portfolio %d: %w", userID, err)
	}
	return nil
}

func UpdatePortfolio(ctx context.Context, kv storage.KVStore, userID int64, now int64, mutate func(*Portfolio) error) (Portfolio, error) {
	vs, ok := kv.(storage.VersionedStore)
	if !ok {
		return Portfolio{}, fmt.Errorf("coin: storage does not support versioned portfolio updates")
	}
	key := portfolioKey(userID)
	for attempt := 0; attempt < portfolioUpdateAttempts; attempt++ {
		p, version, err := loadPortfolioForUpdate(ctx, vs, key, now)
		if err != nil {
			return Portfolio{}, fmt.Errorf("coin: load portfolio %d: %w", userID, err)
		}
		if err := mutate(&p); err != nil {
			return p, err
		}
		p.normalize()
		next, err := json.Marshal(p)
		if err != nil {
			return Portfolio{}, fmt.Errorf("coin: save portfolio %d: json encode: %w", userID, err)
		}
		if err := vs.PutVersioned(ctx, key, version, next); err == nil {
			return p, nil
		} else if !errors.Is(err, storage.ErrConflict) {
			return Portfolio{}, fmt.Errorf("coin: save portfolio %d: %w", userID, err)
		}
	}
	return Portfolio{}, fmt.Errorf("coin: save portfolio %d: %w", userID, storage.ErrConflict)
}

func loadPortfolioForUpdate(ctx context.Context, vs storage.VersionedStore, key string, now int64) (Portfolio, int64, error) {
	raw, version, err := vs.GetVersioned(ctx, key)
	switch {
	case err == nil:
		var p Portfolio
		if err := json.Unmarshal(raw, &p); err != nil {
			return Portfolio{}, 0, fmt.Errorf("json decode: %w", err)
		}
		p.normalize()
		if p.Assets == nil {
			p.Assets = map[string]float64{}
		}
		if p.Meta.CreatedAt == 0 {
			p.Meta.CreatedAt = now
		}
		return p, version, nil
	case errors.Is(err, storage.ErrNotFound):
		return NewPortfolio(now), 0, nil
	default:
		return Portfolio{}, 0, err
	}
}

func (p *Portfolio) AddUSD(amount float64) {
	p.USD += amount
	p.normalize()
}

func (p *Portfolio) DeductUSD(amount float64) (ok bool, balance float64) {
	p.normalize()
	balance = p.USD
	if balance+coinDustEpsilon < amount {
		return false, balance
	}
	p.USD = balance - amount
	p.normalize()
	return true, p.USD
}

func (p *Portfolio) AddAsset(symbol string, amount float64) {
	if p.Assets == nil {
		p.Assets = map[string]float64{}
	}
	p.Assets[symbol] += amount
	p.normalize()
}

func (p *Portfolio) DeductAsset(symbol string, amount float64) (ok bool, held float64) {
	if p.Assets == nil {
		p.Assets = map[string]float64{}
	}
	p.normalize()
	held = p.Assets[symbol]
	if held+coinDustEpsilon < amount {
		return false, held
	}
	p.Assets[symbol] = held - amount
	p.normalize()
	return true, p.Assets[symbol]
}

func (p *Portfolio) normalize() {
	p.USD = normalizeAmount(p.USD)
	p.Meta.Invested = normalizeAmount(p.Meta.Invested)
	if p.Assets == nil {
		return
	}
	for symbol, amount := range p.Assets {
		amount = normalizeAmount(amount)
		if amount == 0 {
			delete(p.Assets, symbol)
		} else {
			p.Assets[symbol] = amount
		}
	}
}

func normalizeAmount(n float64) float64 {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0
	}
	if math.Abs(n) < coinDustEpsilon {
		return 0
	}
	return n
}
