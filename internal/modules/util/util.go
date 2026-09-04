// Package util implements /info, /help, /stickerid, /addsticker — the
// framework-validating "always on" module. /help is a pure renderer over the
// registry, /info and /stickerid are debug helpers, and /addsticker appends a
// replied sticker or photo to one shared, env-configured sticker pack.
package util

import (
	"github.com/tiennm99/miti99bot/internal/modules"
)

// New is the module Factory. Closes over Deps so each handler has access to
// the registry (for /help) and to the bot framework (for sending replies).
func New(deps modules.Deps) modules.Module {
	return modules.Module{
		Commands: []modules.Command{
			infoCommand(),
			helpCommand(deps.Registry),
			stickerIDCommand(),
			addStickerCommand(),
		},
	}
}
