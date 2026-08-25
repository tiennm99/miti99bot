package sticker

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/tiennm99/miti99bot/internal/storage"
)

const (
	// callbackPrefix owns this module's inline-button namespace. Checked
	// bidirectionally against every other module's prefix at registry build.
	callbackPrefix = "sticker_pack:"
	// deleteCallbackPrefix is the confirm button for /delpack.
	deleteCallbackPrefix = callbackPrefix + "d:"
	// pendingDeletePrefix namespaces pending actions inside the collection the
	// Pack records also live in.
	pendingDeletePrefix = "pending-delete:"

	// pendingDeleteTTL is short on purpose. The stock module uses 24h for a
	// non-destructive suggestion; deleting a pack is irreversible on Telegram's
	// side, so the window to confirm is minutes, not a day.
	pendingDeleteTTL = 10 * time.Minute

	// maxCallbackBytes is Telegram's cap on inline-button callback data.
	maxCallbackBytes = 64
)

// PendingDeleteStore is the second typed view over the module's collection.
type PendingDeleteStore = storage.DocStore[PendingDelete]

// PendingDelete is the server-side half of a /delpack confirm button.
//
// The payload in the button is an opaque id and nothing else. Everything that
// decides whether a press is legitimate — who, where, which message, until
// when — lives here, where the user cannot edit it.
type PendingDelete struct {
	ID        string `bson:"id"`
	OwnerID   int64  `bson:"ownerId"`
	Slug      string `bson:"slug"`
	SetName   string `bson:"setName"`
	ChatID    int64  `bson:"chatId"`
	MessageID int    `bson:"messageId"`
	CreatedAt int64  `bson:"createdAt"`
	ExpiresAt int64  `bson:"expiresAt"`
}

// pendingDeleteKey is deterministic per user, so running /delpack twice
// supersedes the first prompt instead of leaving two independently valid
// delete capabilities in scrollback — the shape stock/pending_dividend.go
// already uses and documents.
//
// A random per-invocation key produced two live confirmations at once, which is
// worse than untidy: the stale one could be pressed after the pack it named was
// already deleted and a *different* pack created, and its "set is gone" result
// then cleared the new pack's record.
//
// It also bounds storage. Pending actions are only deleted when consumed, so a
// random key let anyone accumulate documents by running a public command and
// never tapping.
func pendingDeleteKey(ownerID int64) string {
	return pendingDeletePrefix + strconv.FormatInt(ownerID, 10)
}

// newActionID returns an unguessable id for a pending action. Guessability
// matters: the id is the entire contents of the callback payload.
func newActionID() (string, error) {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// deleteCallbackData builds the button payload — a prefix plus the opaque id,
// well inside the 64-byte cap.
func deleteCallbackData(id string) string { return deleteCallbackPrefix + id }

// parseDeleteCallback recovers the action id from client-controlled callback
// data. The id is only a lookup key; every authorisation check happens against
// the stored action.
func parseDeleteCallback(data string) (string, bool) {
	if len(data) > maxCallbackBytes {
		return "", false
	}
	id, ok := strings.CutPrefix(data, deleteCallbackPrefix)
	if !ok || id == "" {
		return "", false
	}
	for _, r := range id {
		if !isHexDigit(r) {
			return "", false
		}
	}
	return id, true
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
}
