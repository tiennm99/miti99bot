package misc

import (
	"bytes"
	"context"
	"errors"
	"html"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

const (
	wheelFilename              = "wheelofnames.gif"
	wheelResultCaptionMaxRunes = 900
)

func wheelOfNamesCommand() modules.Command {
	return modules.Command{
		Name:        "wheelofnames",
		Visibility:  modules.VisibilityPublic,
		Description: "Pick one comma-separated option with wheel GIF when configured",
		Parameters:  "<options(comma-separated)>",
		Handler: func(ctx context.Context, b *bot.Bot, update *models.Update) error {
			if update.Message == nil {
				return nil
			}
			options := splitWheelOptions(chathelper.ArgAfterCommand(update.Message.Text))
			if len(options) == 0 {
				return chathelper.Reply(ctx, b, update.Message, wheelUsage)
			}
			winner := pickWheelOption(options)
			animation, err := renderWheelOfNamesAnimation(ctx, options, winner)
			if err != nil {
				if !errors.Is(err, errWheelAPINotConfigured) {
					log.Warn("wheelofnames remote render failed", "err", err)
				}
				return chathelper.Reply(ctx, b, update.Message, options[winner])
			}
			_, err = b.SendAnimation(ctx, &bot.SendAnimationParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Animation: &models.InputFileUpload{
					Filename: wheelFilename,
					Data:     bytes.NewReader(animation.Data),
				},
				Duration:  animation.Duration,
				Width:     animation.Width,
				Height:    animation.Height,
				Caption:   wheelResultCaption(options[winner]),
				ParseMode: models.ParseModeHTML,
			})
			if err != nil {
				log.Warn("wheelofnames send animation failed", "chat", update.Message.Chat.ID, "err", err)
				return chathelper.Reply(ctx, b, update.Message, options[winner])
			}
			return nil
		},
	}
}

const wheelUsage = "Usage: /wheelofnames <options(comma-separated)>"

func wheelResultCaption(result string) string {
	result = truncateWheelResultCaption(result)
	return `Result: <span class="tg-spoiler">` + html.EscapeString(result) + `</span>`
}

func truncateWheelResultCaption(result string) string {
	runes := []rune(result)
	if len(runes) <= wheelResultCaptionMaxRunes {
		return result
	}
	return string(runes[:wheelResultCaptionMaxRunes]) + "..."
}
