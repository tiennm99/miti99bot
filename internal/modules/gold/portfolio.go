package gold

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/tiennm99/miti99bot/internal/storage"
)

const goldDustEpsilon = 1e-9
const portfolioUpdateAttempts = 5

type Portfolio struct {
	VND   float64       `json:"vnd"`
	Luong float64       `json:"luong"`
	Meta  PortfolioMeta `json:"meta"`
}

type PortfolioMeta struct {
	Invested  float64 `json:"invested"`
	CreatedAt int64   `json:"createdAt"`
}

func NewPortfolio(now int64) Portfolio {
	return Portfolio{Meta: PortfolioMeta{CreatedAt: now}}
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
		if p.Meta.CreatedAt == 0 {
			p.Meta.CreatedAt = now
		}
		return p, nil
	case errors.Is(err, storage.ErrNotFound):
		return NewPortfolio(now), nil
	default:
		return Portfolio{}, fmt.Errorf("gold: load portfolio %d: %w", userID, err)
	}
}

func SavePortfolio(ctx context.Context, kv storage.KVStore, userID int64, p Portfolio) error {
	p.normalize()
	if err := kv.PutJSON(ctx, portfolioKey(userID), p); err != nil {
		return fmt.Errorf("gold: save portfolio %d: %w", userID, err)
	}
	return nil
}

func UpdatePortfolio(ctx context.Context, kv storage.KVStore, userID int64, now int64, mutate func(*Portfolio) error) (Portfolio, error) {
	vs, ok := kv.(storage.VersionedStore)
	if !ok {
		return Portfolio{}, fmt.Errorf("gold: storage does not support versioned portfolio updates")
	}
	key := portfolioKey(userID)
	for attempt := 0; attempt < portfolioUpdateAttempts; attempt++ {
		p, version, err := loadPortfolioForUpdate(ctx, vs, key, now)
		if err != nil {
			return Portfolio{}, fmt.Errorf("gold: load portfolio %d: %w", userID, err)
		}
		if err := mutate(&p); err != nil {
			return p, err
		}
		p.normalize()
		next, err := json.Marshal(p)
		if err != nil {
			return Portfolio{}, fmt.Errorf("gold: save portfolio %d: json encode: %w", userID, err)
		}
		if err := vs.PutVersioned(ctx, key, version, next); err == nil {
			return p, nil
		} else if !errors.Is(err, storage.ErrConflict) {
			return Portfolio{}, fmt.Errorf("gold: save portfolio %d: %w", userID, err)
		}
	}
	return Portfolio{}, fmt.Errorf("gold: save portfolio %d: %w", userID, storage.ErrConflict)
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

func (p *Portfolio) AddVND(amount float64) {
	p.VND += amount
	p.normalize()
}

func (p *Portfolio) DeductVND(amount float64) (ok bool, balance float64) {
	p.normalize()
	balance = p.VND
	if balance+goldDustEpsilon < amount {
		return false, balance
	}
	p.VND = balance - amount
	p.normalize()
	return true, p.VND
}

func (p *Portfolio) AddLuong(amount float64) {
	p.Luong += amount
	p.normalize()
}

func (p *Portfolio) DeductLuong(amount float64) (ok bool, held float64) {
	p.normalize()
	held = p.Luong
	if held+goldDustEpsilon < amount {
		return false, held
	}
	p.Luong = held - amount
	p.normalize()
	return true, p.Luong
}

func (p *Portfolio) normalize() {
	p.VND = normalizeAmount(p.VND)
	p.Luong = normalizeAmount(p.Luong)
	p.Meta.Invested = normalizeAmount(p.Meta.Invested)
}

func normalizeAmount(n float64) float64 {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0
	}
	if math.Abs(n) < goldDustEpsilon {
		return 0
	}
	return n
}
