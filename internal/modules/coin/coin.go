package coin

import "github.com/tiennm99/miti99bot/internal/modules"

// New is the coin paper-trading module factory. It is opt-in through MODULES
// and keeps its portfolio state separate from stock and gold modules.
func New(deps modules.Deps) modules.Module {
	s := newState(deps.KV)
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
				Description: "Sell coin back to USD amount",
				Handler:     s.handleSell,
			},
			{
				Name:        "coin_stats",
				Visibility:  modules.VisibilityPublic,
				Description: "Show coin account summary with P&L",
				Handler:     s.handleStats,
			},
		},
	}
}
