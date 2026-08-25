package sticker

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/keylock"
	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

const (
	// handlerTimeout bounds every handler in this module.
	//
	// Nothing else does. The bot registers handlers with
	// bot.WithNotAsyncHandlers() and one worker, so updates run inline on the
	// polling goroutine, and the handler context is rootCtx, which carries no
	// deadline — the only remaining ceiling is the library's shared 60s HTTP
	// client, per call. A handler making several sequential API calls could
	// therefore freeze the bot for every user for minutes.
	handlerTimeout = 10 * time.Second

	// commitTimeout bounds a post-success store write. These run on a context
	// detached from the request (see commitContext), so they need their own.
	commitTimeout = 5 * time.Second
)

// state holds everything the handlers share. Mirrors the shape used by coin
// and stock: a typed store, a second typed view for pending actions, the
// per-user lock map, and an injectable clock.
type state struct {
	store    PackStore
	pending  PendingDeleteStore
	slugs    SlugStore
	resolver usernameResolver
	locks    keylock.Map
	nowFn    func() time.Time
}

func (s *state) now() time.Time {
	if s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now().UTC()
}

// commitContext detaches a store write from the request context.
//
// rootCtx is cancelled by SIGTERM, and a handler is most likely to be mid-flight
// exactly when a deploy lands. A commit that records a completed Telegram-side
// action must not be lost because the process is shutting down: at that point
// the set already exists and only the bot's memory of it is at stake.
func commitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), commitTimeout)
}

// handlerContext applies the module-wide deadline.
func handlerContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, handlerTimeout)
}

// mediaContext bounds the download-and-upload leg of a handler, reserving the
// tail of the parent's budget for what comes after it.
//
// This module is the only one that spends most of its deadline before it has
// anything to say: a photo /newpack downloads, resamples and re-uploads before
// it calls CreateNewStickerSet. Run on the bare handler context, a slow link
// exhausted the whole 10s inside the media leg, and the reply — including the
// error reply explaining what went wrong — was then sent on a dead context, so
// the user saw nothing at all. chathelper.FetchContext is the existing fix for
// exactly this, already used by coin, gold, stock and monkeyd.
func mediaContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return chathelper.FetchContext(ctx)
}

// lockUser serialises a user's mutations.
//
// Nothing contends for it today: the map is state-local to this module, the bot
// dispatches inline with a single worker, and this module registers neither a
// cron nor a command hook. An earlier version of this comment claimed the cron
// scheduler and stats hook contended here — they do not, and a wrong reason for
// a right guard is worse than none, because the next reader trusts it.
//
// It stays because every mutation here is a read-modify-write, which is wrong
// the moment dispatch stops being serial, and an uncontended mutex costs
// nothing. Note that releaseSlug's read-then-delete is not atomic under this
// lock either; that is safe only while dispatch is serial.
func (s *state) lockUser(ownerID int64) func() {
	return s.locks.Acquire(strconv.FormatInt(ownerID, 10))
}

// commandArgs returns the whitespace-separated arguments after the command.
func commandArgs(msg *models.Message) []string {
	return strings.Fields(chathelper.ArgAfterCommand(msg.Text))
}

// commandArgText returns the raw text after the command, trimmed.
func commandArgText(msg *models.Message) string {
	return chathelper.ArgAfterCommand(msg.Text)
}

// reply sends text as a reply to msg.
func reply(ctx context.Context, b *bot.Bot, msg *models.Message, text string) error {
	return chathelper.Reply(ctx, b, msg, text)
}

// replyErr turns a handler error into a reply.
//
// A userError is shown verbatim — it was written for the user. Anything else is
// logged and replaced with a generic line: internal errors can carry a download
// URL with the bot token in it, and this module must never echo one.
func replyErr(ctx context.Context, b *bot.Bot, msg *models.Message, op string, err error) error {
	var ue userError
	if errors.As(err, &ue) {
		return reply(ctx, b, msg, ue.msg)
	}
	log.Error(op, "err", err)
	return reply(ctx, b, msg, genericFailure)
}

const genericFailure = "Something went wrong. Try again in a moment."
