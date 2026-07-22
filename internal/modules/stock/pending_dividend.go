package stock

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tiennm99/miti99bot/internal/storage"
)

const (
	dividendCallbackPrefix = "stock_div:"
	pendingDividendPrefix  = "pending-dividend:"
	pendingDividendTTL     = 24 * time.Hour
	dividendTokenBytes     = 16
	dividendTokenLength    = 22
)

var dividendTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

type PendingDividendStore = storage.DocStore[PendingDividendAction]

// PendingDividendAction is the server-side half of an inline button. Financial
// values live in the user's dividend history; Telegram receives only an opaque
// random token.
type PendingDividendAction struct {
	OwnerUserID      int64  `json:"ownerUserId" bson:"ownerUserId"`
	ChatID           int64  `json:"chatId" bson:"chatId"`
	MessageID        int    `json:"messageId" bson:"messageId"`
	ProviderEventID  string `json:"providerEventId" bson:"providerEventId"`
	Symbol           string `json:"symbol" bson:"symbol"`
	PositionOpenedAt int64  `json:"positionOpenedAt,omitempty" bson:"positionOpenedAt,omitempty"`
	CreatedAt        int64  `json:"createdAt" bson:"createdAt"`
	ExpiresAt        int64  `json:"expiresAt" bson:"expiresAt"`
}

func pendingDividendKey(token string) string { return pendingDividendPrefix + token }

func generateDividendToken() (string, error) {
	raw := make([]byte, dividendTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate dividend action token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func validDividendToken(token string) bool {
	return len(token) == dividendTokenLength && dividendTokenPattern.MatchString(token)
}

func callbackToken(data string) (string, bool) {
	token, ok := strings.CutPrefix(data, dividendCallbackPrefix)
	if !ok || !validDividendToken(token) {
		return "", false
	}
	return token, true
}

func (s *state) createPendingDividend(ctx context.Context, action PendingDividendAction) (string, error) {
	if s.pending == nil {
		return "", errors.New("stock: pending dividend store unavailable")
	}
	generator := s.newDividendToken
	if generator == nil {
		generator = generateDividendToken
	}
	for attempt := 0; attempt < 3; attempt++ {
		token, err := generator()
		if err != nil {
			return "", err
		}
		if !validDividendToken(token) {
			return "", errors.New("stock: invalid generated dividend token")
		}
		err = s.pending.PutVersioned(ctx, pendingDividendKey(token), 0, action)
		if errors.Is(err, storage.ErrConflict) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("save pending dividend action: %w", err)
		}
		return token, nil
	}
	return "", errors.New("stock: could not allocate unique dividend token")
}

func (s *state) cleanupExpiredDividends(ctx context.Context, now int64) {
	if s.pending == nil {
		return
	}
	keys, err := s.pending.List(ctx, pendingDividendPrefix)
	if err != nil {
		return
	}
	for _, key := range keys {
		action, _, getErr := s.pending.Get(ctx, key)
		if getErr == nil && action.ExpiresAt > 0 && action.ExpiresAt <= now {
			_ = s.pending.Delete(ctx, key)
		}
	}
}
