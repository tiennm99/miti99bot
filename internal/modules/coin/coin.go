package coin

import (
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// New is the coin paper-trading module factory. It is opt-in through MODULES
// and keeps its portfolio state separate from stock and gold modules.
func New(deps modules.Deps) modules.Module {
	s := newState(storage.Typed[Portfolio](deps.Store))
	return modules.Module{
		Commands: []modules.Command{
			{
				Name:        "coin_price",
				Visibility:  modules.VisibilityPublic,
				Description: "Show current crypto price in USD",
				Handler:     s.handlePrice,
			},
			{
				Name:        "coin_topup",
				Visibility:  modules.VisibilityPublic,
				Description: "Top up USD to your coin account",
				Handler:     s.handleTopup,
			},
			{
				Name:        "coin_buy",
				Visibility:  modules.VisibilityPublic,
				Description: "Buy coin with USD amount",
				Handler:     s.handleBuy,
			},
			{
				Name:        "coin_sell",
				Visibility:  modules.VisibilityPublic,
				Description: "Sell coin for a USD amount",
				Handler:     s.handleSell,
			},
			{
				Name:        "coin_portfolio",
				Visibility:  modules.VisibilityPublic,
				Description: "Show coin portfolio with P&L",
				Handler:     s.handleStats,
			},
		},
	}
}
