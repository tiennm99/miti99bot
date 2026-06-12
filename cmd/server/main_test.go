package main

import (
	"testing"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

func TestFactoriesIncludesGoldAndCoin(t *testing.T) {
	catalog := factories()
	if catalog["gold"] == nil {
		t.Fatal("factories missing gold")
	}
	if catalog["coin"] == nil {
		t.Fatal("factories missing coin")
	}
	reg, err := modules.Build([]string{"gold", "coin"}, catalog, storage.NewMemoryProvider(), modules.BuildOptions{})
	if err != nil {
		t.Fatalf("Build gold: %v", err)
	}
	for _, name := range []string{
		"gold_price", "gold_topup", "gold_buy", "gold_sell", "gold_stats",
		"coin_price", "coin_topup", "coin_buy", "coin_sell", "coin_stats",
	} {
		if _, ok := reg.AllCommands[name]; !ok {
			t.Fatalf("missing command %s", name)
		}
	}
}
