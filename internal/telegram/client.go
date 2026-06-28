package telegram

import (
	"github.com/go-telegram/bot"
)

// pollingAllowedUpdates restricts getUpdates to the update kinds the modules
// actually handle (text commands + inline-keyboard callbacks), matching the
// allowed_updates the old webhook registration set. Anything else (channel
// posts, edited messages, etc.) is dropped server-side by Telegram.
var pollingAllowedUpdates = bot.AllowedUpdates{"message", "callback_query"}

// NewBot constructs a Telegram bot for long-polling mode (the sole transport
// on self-host — b.Start runs the getUpdates loop in cmd/server):
//
//   - WithSkipGetMe: avoid a blocking GetMe call at startup. Token validity
//     surfaces on the first outgoing API call instead.
//   - WithNotAsyncHandlers: handlers run synchronously inside the dispatch
//     goroutine. Module handlers take their own ctx (not r.Context()), so this
//     is safe; it also bounds in-flight work to one update at a time, which
//     suits the single-replica polling deployment.
//   - WithAllowedUpdates: only request the update kinds the bot handles.
//
// Callers may pass extra options that override these defaults.
func NewBot(token string, opts ...bot.Option) (*bot.Bot, error) {
	defaults := []bot.Option{
		bot.WithSkipGetMe(),
		bot.WithNotAsyncHandlers(),
		bot.WithAllowedUpdates(pollingAllowedUpdates),
	}
	return bot.New(token, append(defaults, opts...)...)
}
