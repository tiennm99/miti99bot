package sticker

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
)

const (
	// maxSetNameLen is Telegram's cap on a sticker set's short name.
	maxSetNameLen = 64
	// maxTitleLen is Telegram's cap on a set title.
	maxTitleLen = 64
	// minSlugLen / maxSlugLen keep the share link readable and leave room for
	// the "_by_<botusername>" suffix inside maxSetNameLen.
	minSlugLen = 3
	maxSlugLen = 40
)

// slugRe is the user-chosen half of a set name: 3-40 chars, starting with a
// letter. Telegram additionally forbids consecutive underscores, which a
// character class cannot express, so validateSlug checks that separately.
var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,39}$`)

// validateSlug reports why a slug is unusable, or nil when it is fine.
//
// The slug is the one irreversible choice in this module: it fixes
// t.me/addstickers/<slug>_by_<bot> forever, because Telegram has no
// rename-short-name method. Rejecting loudly here is much cheaper than a user
// discovering the typo is permanent.
func validateSlug(slug string) error {
	if !slugRe.MatchString(slug) {
		return refuse(fmt.Sprintf("Pack name must be %d-%d characters: lowercase letters, digits and underscores, starting with a letter.", minSlugLen, maxSlugLen))
	}
	if strings.Contains(slug, "__") {
		return refuse("Pack name cannot contain two underscores in a row.")
	}
	if strings.HasSuffix(slug, "_") {
		return refuse("Pack name cannot end with an underscore.")
	}
	return nil
}

// makeSetName builds the Telegram set name for a new pack. It is used only at
// creation — never to resolve ownership, which compares the *stored* name (see
// ownsSet).
//
// The error reports the remaining budget rather than only refusing, because the
// only fix available to the user is a shorter slug and the limit depends on the
// bot's username length, which they cannot see.
func makeSetName(slug, botUsername string) (string, error) {
	if botUsername == "" {
		return "", errNoUsername
	}
	suffix := "_by_" + botUsername
	if len(slug)+len(suffix) > maxSetNameLen {
		budget := maxSetNameLen - len(suffix)
		if budget > maxSlugLen {
			budget = maxSlugLen
		}
		return "", refuse(fmt.Sprintf("Pack name is too long for this bot — use at most %d characters.", budget))
	}
	return slug + suffix, nil
}

// ownsSet reports whether setName is the caller's pack, comparing
// case-insensitively against the *stored* Pack.Name.
//
// It deliberately does not re-derive the name from the live bot username.
// Renaming the bot in BotFather is supported and leaves existing set names
// untouched, so a derived comparison would make every user's own pack refuse as
// "not yours" while /mypack still displayed it. Comparing the stored name also
// sidesteps casing: Telegram returns SetName with whatever casing the set was
// created with.
func ownsSet(pack Pack, setName string) bool {
	if pack.Name == "" || setName == "" {
		return false
	}
	return strings.EqualFold(pack.Name, setName)
}

// usernameResolver caches the bot's username for building new set names.
//
// The bot starts with bot.WithSkipGetMe(), so nothing populates a username
// until this asks. Failures are never cached: a transient GetMe error must not
// disable /newpack for the process's lifetime.
type usernameResolver struct {
	mu       sync.Mutex
	username string
}

// resolve returns the bot's username, calling GetMe at most once per success.
// It takes the handler's *bot.Bot rather than Deps.Bot, which is documented
// nil-safe and is nil under BuildOptions{}.
func (r *usernameResolver) resolve(ctx context.Context, b *bot.Bot) (string, error) {
	r.mu.Lock()
	cached := r.username
	r.mu.Unlock()
	if cached != "" {
		return cached, nil
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		return "", err
	}
	if me == nil || me.Username == "" {
		return "", errNoUsername
	}

	r.mu.Lock()
	r.username = me.Username
	r.mu.Unlock()
	return me.Username, nil
}
