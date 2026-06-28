package stock

import (
	"github.com/tiennm99/miti99bot/internal/modules"
)

// New is the stock module Factory. Five user-facing commands; no crons.
// (Original miti99bot only has a SQL retention cron, which our KV-only port
// does not implement — keeping commits paper-ledger-only is acceptable.)
func New(deps modules.Deps) modules.Module {
	s := newState(deps.KV)
	return modules.Module{
		Commands: []modules.Command{
			{
				Name:        "stock_topup",
				Visibility:  modules.VisibilityPublic,
				Description: "Top up VND to your stock account",
				Handler:     s.handleTopup,
			},
			{
				Name:        "stock_buy",
				Visibility:  modules.VisibilityPublic,
				Description: "Buy VN stock at market price (qty TICKER)",
				Handler:     s.handleBuy,
			},
			{
				Name:        "stock_sell",
				Visibility:  modules.VisibilityPublic,
				Description: "Sell VN stock back to VND (qty TICKER)",
				Handler:     s.handleSell,
			},
			{
				Name:        "stock_income_stock",
				Visibility:  modules.VisibilityPublic,
				Description: "Record stock dividend (bonus shares)",
				Handler:     s.handleIncomeStock,
			},
			{
				Name:        "stock_income_vnd",
				Visibility:  modules.VisibilityPublic,
				Description: "Record cash dividend (VND per share)",
				Handler:     s.handleIncomeVND,
			},
			{
				Name:        "stock_convert",
				Visibility:  modules.VisibilityPublic,
				Description: "Currency exchange (coming soon)",
				Handler:     s.handleConvert,
			},
			{
				Name:        "stock_stats",
				Visibility:  modules.VisibilityPublic,
				Description: "Show portfolio summary with P&L",
				Handler:     s.handleStats,
			},
		},
	}
}
