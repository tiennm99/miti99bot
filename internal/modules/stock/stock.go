package stock

import (
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// New is the stock module Factory. Eight user-facing commands.
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
				Name:        "stock_cash_dividend",
				Visibility:  modules.VisibilityPublic,
				Description: "Record cash dividend (VND/share TICKER)",
				Handler:     s.handleCashDividend,
			},
			{
				Name:        "stock_share_dividend",
				Visibility:  modules.VisibilityPublic,
				Description: "Record share dividend (owned:new TICKER)",
				Handler:     s.handleShareDividend,
			},
			{
				Name:        "stock_dividend",
				Visibility:  modules.VisibilityPublic,
				Description: "Record cash and share dividend",
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
