package main

import (
	"testing"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

func TestFactoriesIncludesGold(t *testing.T) {
	catalog := factories()
	if catalog["gold"] == nil {
		t.Fatal("factories missing gold")
	}
	reg, err := modules.Build([]string{"gold"}, catalog, storage.NewMemoryProvider(), modules.BuildOptions{})
	if err != nil {
		t.Fatalf("Build gold: %v", err)
	}
	for _, name := range []string{"gold_price", "gold_topup", "gold_buy", "gold_sell", "gold_stats"} {
		if _, ok := reg.AllCommands[name]; !ok {
			t.Fatalf("missing command %s", name)
		}
	}
}
