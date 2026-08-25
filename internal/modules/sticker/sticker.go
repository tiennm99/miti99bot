package sticker

import (
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// New is the sticker-packs module factory.
//
// Commands are unprefixed so they match the names @Stickers uses: the registry
// keys commands by Command.Name independent of module name, which is how misc
// ships /ff. Three typed views share one collection, with disjoint key spaces:
// Pack records keyed by owner ID, name reservations under "slug:", and pending
// deletes under "pending-delete:".
func New(deps modules.Deps) modules.Module {
	s := &state{
		store:   storage.Typed[Pack](deps.Store),
		pending: storage.Typed[PendingDelete](deps.Store),
		slugs:   storage.Typed[SlugReservation](deps.Store),
	}
	return modules.Module{
		Commands: []modules.Command{
			{
				Name:        "newpack",
				Visibility:  modules.VisibilityPublic,
				Description: "Create your sticker pack from a replied sticker",
				Parameters:  "<pack> <title...>",
				Handler:     s.handleNewPack,
			},
			{
				Name:        "mypack",
				Visibility:  modules.VisibilityPublic,
				Description: "Show your sticker pack and its link",
				Handler:     s.handleMyPack,
			},
			{
				Name:        "addsticker",
				Visibility:  modules.VisibilityPublic,
				Description: "Add the replied sticker to your pack",
				Parameters:  "[emoji...]",
				Handler:     s.handleAddSticker,
			},
			{
				Name:        "delsticker",
				Visibility:  modules.VisibilityPublic,
				Description: "Remove the replied sticker from your pack",
				Handler:     s.handleDelSticker,
			},
			{
				Name:        "editsticker",
				Visibility:  modules.VisibilityPublic,
				Description: "Change the emoji of a sticker in your pack",
				Parameters:  "<emoji...>",
				Handler:     s.handleEditSticker,
			},
			{
				Name:        "ordersticker",
				Visibility:  modules.VisibilityPublic,
				Description: "Move a sticker in your pack to a position",
				Parameters:  "<position>",
				Handler:     s.handleOrderSticker,
			},
			{
				Name:        "setpackicon",
				Visibility:  modules.VisibilityPublic,
				Description: "Set your pack's icon from a sticker in it",
				Handler:     s.handleSetPackIcon,
			},
			{
				Name:        "renamepack",
				Visibility:  modules.VisibilityPublic,
				Description: "Change your pack's title (the link cannot change)",
				Parameters:  "<title...>",
				Handler:     s.handleRenamePack,
			},
			{
				Name:        "delpack",
				Visibility:  modules.VisibilityPublic,
				Description: "Delete your pack after confirmation",
				Handler:     s.handleDelPack,
			},
		},
		Callbacks: []modules.Callback{{
			Prefix:     callbackPrefix,
			Visibility: modules.VisibilityPublic,
			Handler:    s.handleDelPackCallback,
		}},
	}
}
