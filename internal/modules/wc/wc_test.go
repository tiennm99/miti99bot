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
	if len(mod.Crons) != 1 || mod.Crons[0].Name != dailyPushCronName {
		t.Fatalf("crons = %+v, want %s", mod.Crons, dailyPushCronName)
	}
}
