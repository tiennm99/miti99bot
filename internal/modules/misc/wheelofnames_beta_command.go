package misc

import (
	"bytes"
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

const (
	wheelOfNamesBetaUsage = "Usage: /wheelofnamesbeta <option1>, <option2>, ..."
	wheelBetaFilename     = "wheelofnamesbeta.gif"
)

func wheelOfNamesBetaCommand() modules.Command {
	return modules.Command{
		Name:        "wheelofnamesbeta",
		Visibility:  modules.VisibilityPublic,
		Description: "Spin an animated wheel GIF for comma-separated options",
		Handler: func(ctx context.Context, b *bot.Bot, update *models.Update) error {
			if update.Message == nil {
				return nil
			}
			options := splitWheelOptions(chathelper.ArgAfterCommand(update.Message.Text))
			if len(options) == 0 {
				return chathelper.Reply(ctx, b, update.Message, wheelOfNamesBetaUsage)
			}
			winner := pickWheelOption(options)
			animation, err := renderWheelOfNamesBetaAnimation(ctx, options, winner)
			if err != nil {
				log.Error("wheelofnamesbeta render failed", "err", err)
				return chathelper.Reply(ctx, b, update.Message, options[winner])
			}
			_, err = b.SendAnimation(ctx, &bot.SendAnimationParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Animation: &models.InputFileUpload{
					Filename: wheelBetaFilename,
					Data:     bytes.NewReader(animation.Data),
				},
				Duration: animation.Duration,
				Width:    animation.Width,
				Height:   animation.Height,
				Caption:  "Spinning...",
			})
			if err != nil {
				log.Warn("wheelofnamesbeta send animation failed", "chat", update.Message.Chat.ID, "err", err)
				return chathelper.Reply(ctx, b, update.Message, options[winner])
			}
			return nil
		},
	}
}
