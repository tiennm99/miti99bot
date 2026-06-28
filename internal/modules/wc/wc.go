package wc

import (
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// New is the World Cup schedule module factory.
func New(deps modules.Deps) modules.Module {
	s := &state{
		subscribers: storage.Typed[subscribersDoc](deps.Store),
		pushDate:    storage.Typed[lastPushDoc](deps.Store),
		cache:       storage.Typed[cacheRecord](deps.Store),
		client:      NewClientFromEnv(),
	}
	return modules.Module{
		Commands: []modules.Command{
			{
				Name:        "wc",
				Visibility:  modules.VisibilityPublic,
				Description: "World Cup matches for a date (dd-mm-yyyy, dd/mm/yyyy, ddmmyyyy; default today)",
				Handler:     s.handleSchedule,
			},
			{
				Name:        "wc_today",
				Visibility:  modules.VisibilityPublic,
				Description: "Today's World Cup matches (scores if available)",
				Handler:     s.handleToday,
			},
			{
				Name:        "wc_week",
				Visibility:  modules.VisibilityPublic,
				Description: "World Cup matches for this week (Mon-Sun, ICT)",
				Handler:     s.handleWeek,
			},
			{
				Name:        "wc_subscribe",
				Visibility:  modules.VisibilityPublic,
				Description: "Get the daily World Cup schedule digest at 08:00 ICT",
				Handler:     s.handleSubscribe,
			},
			{
				Name:        "wc_unsubscribe",
				Visibility:  modules.VisibilityPublic,
				Description: "Stop receiving the daily World Cup schedule digest",
				Handler:     s.handleUnsubscribe,
			},
		},
		Crons: []modules.Cron{s.dailyPushCron()},
	}
}
