// Package random groups the commands that pick one option at random: /random
// (plain text pick), /wheelofnames (wheel GIF when a renderer is configured),
// /gacha (Genshin-style wish MP4), and the unlisted /gachabeta (beta version
// of the gacha wish). The animated commands share the optional wheelofnames renderer
// and fall back to a text reply without it.
package random

import "github.com/tiennm99/miti99bot/internal/modules"

// New is the module Factory. The commands keep no state, so deps is unused.
func New(_ modules.Deps) modules.Module {
	return modules.Module{
		Commands: []modules.Command{
			randomCommand(),
			wheelOfNamesCommand(),
			gachaCommand(),
			gachaBetaCommand(),
		},
	}
}
