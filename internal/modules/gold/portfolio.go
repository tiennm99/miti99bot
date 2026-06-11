package gold

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/tiennm99/miti99bot/internal/storage"
)

const goldDustEpsilon = 1e-9

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
