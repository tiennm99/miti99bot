package wc

import (
	"testing"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

func TestNewRegistersExpectedCommandsAndCron(t *testing.T) {
	mod := New(modules.Deps{Store: storage.NewMemoryProvider().Collection("wc")})
	got := map[string]bool{}
	for _, cmd := range mod.Commands {
		got[cmd.Name] = true
	}
	for _, name := range []string{"wc", "wc_today", "wc_week", "wc_subscribe", "wc_unsubscribe"} {
		if !got[name] {
			t.Fatalf("missing command %s", name)
		}
	}
	if len(mod.Crons) != 1 || mod.Crons[0].Name != dailyPushCronName || mod.Crons[0].Schedule != "0 17 * * *" {
		t.Fatalf("crons = %+v, want %s at 00:00 UTC+7", mod.Crons, dailyPushCronName)
	}
}
