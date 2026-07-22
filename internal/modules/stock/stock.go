package stock

import (
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// New is the stock module Factory. Seven user-facing commands.
func New(deps modules.Deps) modules.Module {
	s := newState(
		storage.Typed[Portfolio](deps.Store),
		storage.Typed[PendingDividendAction](deps.Store),
	)
	return modules.Module{
		Callbacks: []modules.Callback{{Prefix: dividendCallbackPrefix, Visibility: modules.VisibilityPublic, Handler: s.handleDividendCallback}},
		Commands: []modules.Command{
			{
				Name:        "stock_price",
				Visibility:  modules.VisibilityPublic,
				Description: "Show current VN stock price",
				Parameters:  "<ticker>",
				Handler:     s.handlePrice,
			},
			{
				Name:        "stock_topup",
				Visibility:  modules.VisibilityPublic,
				Description: "Top up VND to your stock account",
				Parameters:  "<vnd_amount>",
				Handler:     s.handleTopup,
			},
			{
				Name:        "stock_buy",
				Visibility:  modules.VisibilityPublic,
				Description: "Buy VN stock at market price",
				Parameters:  "<quantity> <ticker>",
				Handler:     s.handleBuy,
			},
			{
				Name:        "stock_sell",
				Visibility:  modules.VisibilityPublic,
				Description: "Sell VN stock back to VND",
				Parameters:  "<quantity> <ticker>",
				Handler:     s.handleSell,
			},
			{
				Name:        "stock_cash_dividend",
				Visibility:  modules.VisibilityPublic,
				Description: "Record cash dividend",
				Parameters:  "<vnd_per_share> <ticker>",
				Handler:     s.handleCashDividend,
			},
			{
				Name:        "stock_share_dividend",
				Visibility:  modules.VisibilityPublic,
				Description: "Record share dividend",
				Parameters:  "<ratio(owned:new)> <ticker>",
				Handler:     s.handleShareDividend,
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
