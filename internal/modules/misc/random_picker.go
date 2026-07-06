package misc

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

const (
	randomUsage       = "Usage: /random <option1>, <option2>, ..."
	wheelOfNamesUsage = "Usage: /wheelofnames <option1>, <option2>, ..."
)

var wheelDraftFrameDelay = 500 * time.Millisecond

func splitWheelOptions(arg string) []string {
	parts := strings.Split(arg, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if option := strings.TrimSpace(part); option != "" {
			out = append(out, option)
		}
	}
	return out
}

func randomCommand() modules.Command {
	return modules.Command{
		Name:        "random",
		Visibility:  modules.VisibilityPublic,
		Description: "Pick one random comma-separated option",
		Handler: func(ctx context.Context, b *bot.Bot, update *models.Update) error {
			if update.Message == nil {
				return nil
			}
			options := splitWheelOptions(chathelper.ArgAfterCommand(update.Message.Text))
			if len(options) == 0 {
				return chathelper.Reply(ctx, b, update.Message, randomUsage)
			}
			return chathelper.Reply(ctx, b, update.Message, options[pickWheelOption(options)])
		},
	}
}

func wheelOfNamesCommand() modules.Command {
	return modules.Command{
		Name:        "wheelofnames",
		Visibility:  modules.VisibilityPublic,
		Description: "Spin a suspenseful wheel for comma-separated options",
		Handler: func(ctx context.Context, b *bot.Bot, update *models.Update) error {
			if update.Message == nil {
				return nil
			}
			options := splitWheelOptions(chathelper.ArgAfterCommand(update.Message.Text))
			if len(options) == 0 {
				return chathelper.Reply(ctx, b, update.Message, wheelOfNamesUsage)
			}
			winner := pickWheelOption(options)
			streamWheelDrafts(ctx, b, update.Message, options, winner)
			return chathelper.Reply(ctx, b, update.Message, options[winner])
		},
	}
}

func pickWheelOption(options []string) int {
	return rand.N(len(options))
}

func streamWheelDrafts(ctx context.Context, b *bot.Bot, msg *models.Message, options []string, winner int) {
	if msg.Chat.Type != models.ChatTypePrivate {
		return
	}
	draftID := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	frames := wheelDraftFrames(options, winner)
	for i, text := range frames {
		ok, err := b.SendMessageDraft(ctx, &bot.SendMessageDraftParams{
			ChatID:          msg.Chat.ID,
			MessageThreadID: msg.MessageThreadID,
			DraftID:         draftID,
			Text:            text,
		})
		if err != nil {
			log.Warn("wheelofnames draft stream failed", "chat", msg.Chat.ID, "err", err)
			return
		}
		if !ok {
			log.Warn("wheelofnames draft stream returned false", "chat", msg.Chat.ID)
			return
		}
		if i < len(frames)-1 && !waitWheelDraftFrame(ctx) {
			return
		}
	}
}

func wheelDraftFrames(options []string, winner int) []string {
	candidates := []string{
		options[(winner+1)%len(options)],
		options[(winner+2)%len(options)],
		options[(winner+3)%len(options)],
		options[(winner+4)%len(options)],
		options[(winner+5)%len(options)],
		options[winner],
	}
	labels := []string{
		"Spinning the wheel...",
		"Still spinning...",
		"Picking up speed...",
		"Last few names...",
		"Slowing down...",
		"Almost there...",
	}
	frames := make([]string, 0, len(labels))
	for i, label := range labels {
		frames = append(frames, fmt.Sprintf("%s\n> %s", label, candidates[i]))
	}
	return frames
}

func waitWheelDraftFrame(ctx context.Context) bool {
	if wheelDraftFrameDelay <= 0 {
		return true
	}
	timer := time.NewTimer(wheelDraftFrameDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
