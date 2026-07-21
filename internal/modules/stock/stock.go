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
				Parameters:  "<ticker>",
				Example:     "/stock_price TCB",
				Handler:     s.handlePrice,
			},
			{
				Name:        "stock_topup",
				Visibility:  modules.VisibilityPublic,
				Description: "Top up VND to your stock account",
				Parameters:  "<vnd_amount>",
				Example:     "/stock_topup 5000000",
				Handler:     s.handleTopup,
			},
			{
				Name:        "stock_buy",
				Visibility:  modules.VisibilityPublic,
				Description: "Buy VN stock at market price",
				Parameters:  "<quantity> <ticker>",
				Example:     "/stock_buy 100 TCB",
				Handler:     s.handleBuy,
			},
			{
				Name:        "stock_sell",
				Visibility:  modules.VisibilityPublic,
				Description: "Sell VN stock back to VND",
				Parameters:  "<quantity> <ticker>",
				Example:     "/stock_sell 100 TCB",
				Handler:     s.handleSell,
			},
			{
				Name:        "stock_cash_dividend",
				Visibility:  modules.VisibilityPublic,
				Description: "Record cash dividend",
				Parameters:  "<vnd_per_share> <ticker>",
				Example:     "/stock_cash_dividend 1500 TCB",
				Handler:     s.handleCashDividend,
			},
			{
				Name:        "stock_share_dividend",
				Visibility:  modules.VisibilityPublic,
				Description: "Record share dividend",
				Parameters:  "<ratio(owned:new)> <ticker>",
				Example:     "/stock_share_dividend 100:10 TCB",
				Handler:     s.handleShareDividend,
			},
			{
				Name:        "stock_dividend",
				Visibility:  modules.VisibilityPublic,
				Description: "Record cash and share dividend",
				Parameters:  "<vnd_per_share> <ratio(owned:new)> <ticker>",
				Example:     "/stock_dividend 1500 100:10 TCB",
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
