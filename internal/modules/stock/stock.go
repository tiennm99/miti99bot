package stock

import (
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// New is the stock module Factory. Seven user-facing commands.
func New(deps modules.Deps) modules.Module {
	s := newState(storage.Typed[Portfolio](deps.Store))
	return modules.Module{
		Commands: []modules.Command{
			{
				Name:        "stock_price",
				Visibility:  modules.VisibilityPublic,
				Description: "Show current VN stock price",
				Handler:     s.handlePrice,
			},
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
				Name:        "stock_bonus",
				Visibility:  modules.VisibilityPublic,
				Description: "Record bonus shares",
				Handler:     s.handleBonus,
			},
			{
				Name:        "stock_dividend",
				Visibility:  modules.VisibilityPublic,
				Description: "Record cash dividend (VND per share)",
				Handler:     s.handleDividend,
			},
			{
				Name:        "stock_portfolio",
				Visibility:  modules.VisibilityPublic,
				Description: "Show stock portfolio with P&L",
				Handler:     s.handleStats,
			},
		},
	}
}
