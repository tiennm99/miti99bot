package gold

import "github.com/tiennm99/miti99bot/internal/modules"

// New is the gold paper-trading module factory. It is opt-in through MODULES
// and keeps its portfolio state separate from the VN-stocks trading module.
func New(deps modules.Deps) modules.Module {
	s := newState(deps.KV)
	return modules.Module{
		Commands: []modules.Command{
			{
				Name:        "gold_topup",
				Visibility:  modules.VisibilityPublic,
				Description: "Top up VND to your gold account",
				Handler:     s.handleTopup,
			},
			{
				Name:        "gold_buy",
				Visibility:  modules.VisibilityPublic,
				Description: "Buy gold at spot price (luong)",
				Handler:     s.handleBuy,
			},
			{
				Name:        "gold_sell",
				Visibility:  modules.VisibilityPublic,
				Description: "Sell gold back to VND (luong)",
				Handler:     s.handleSell,
			},
			{
				Name:        "gold_stats",
				Visibility:  modules.VisibilityPublic,
				Description: "Show gold account summary with P&L",
				Handler:     s.handleStats,
			},
		},
	}
}
