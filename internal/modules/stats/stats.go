// Package stats tracks command usage and exposes /stats subcommands sorted by
// popularity.
package stats

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

const topK = 20

// counter owns the stats repository used by the command hook and render views.
type counter struct {
	store usageStore
}

// Inc records one authorized command invocation. A sender only contributes to
// user-level stats when Telegram provides a public username; otherwise the
// invocation still contributes to command totals.
func (c *counter) Inc(ctx context.Context, name string, update *models.Update) {
	var (
		user    usageUser
		hasUser bool
	)
	if update != nil && update.Message != nil && update.Message.From != nil && update.Message.From.Username != "" {
		user = usageUser{
			ID:       update.Message.From.ID,
			Username: update.Message.From.Username,
		}
		hasUser = true
	}

	if err := c.store.Increment(ctx, name, user, hasUser); err != nil {
		if hasUser {
			log.Error("stats: increment failed", "command", name, "user_id", user.ID, "err", err)
			return
		}
		log.Error("stats: increment failed", "command", name, "err", err)
	}
}

func newCounter(coll storage.Collection) *counter {
	return &counter{store: newUsageStore(coll)}
}

// New is the module Factory. Registers a CommandHook that persists counts and
// a /stats command that displays them.
func New(deps modules.Deps) modules.Module {
	c := newCounter(deps.Store)
	return modules.Module{
		CommandHook: c.Inc,
		Commands: []modules.Command{
			statsCommand(c),
		},
	}
}
