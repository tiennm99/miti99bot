package lol

import (
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// New is the lol module Factory. The 4 user-facing commands plus the
// daily-push cron (lol_daily_push at 08:00 ICT, fan-out to
// subscribers) are wired here. The cron handler reads deps.Bot at invoke
// time — main.go must wire BuildOptions.Bot for the cron to function;
// without it the handler fails fast with a clear error.
func New(deps modules.Deps) modules.Module {
	s := &state{
		subscribers: storage.Typed[subscribersDoc](deps.Store),
		pushDate:    storage.Typed[lastPushDoc](deps.Store),
		cache:       storage.Typed[cacheRecord](deps.Store),
		client:      &Client{},
	}
	return modules.Module{
		Commands: []modules.Command{
			{
				Name:        "lol",
				Visibility:  modules.VisibilityPublic,
				Description: "LoL matches for a date (dd-mm-yyyy, dd/mm/yyyy, ddmmyyyy; default today)",
				Handler:     s.handleSchedule,
			},
			{
				Name:        "lol_this_week",
				Visibility:  modules.VisibilityPublic,
				Description: "LoL esports matches for this week (Mon–Sun, ICT)",
				Handler:     s.handleWeek,
			},
			{
				Name:        "lol_subscribe",
				Visibility:  modules.VisibilityPublic,
				Description: "Get the daily LoL schedule digest at 08:00 ICT",
				Handler:     s.handleSubscribe,
			},
			{
				Name:        "lol_unsubscribe",
				Visibility:  modules.VisibilityPublic,
				Description: "Stop receiving the daily LoL schedule digest",
				Handler:     s.handleUnsubscribe,
			},
		},
		Crons: []modules.Cron{s.dailyPushCron()},
	}
}
