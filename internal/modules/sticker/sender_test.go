package sticker

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

// Every pack is keyed by this value, so anything that is not one human must be
// refused before the key is built.
func TestSenderID(t *testing.T) {
	cases := []struct {
		name string
		msg  *models.Message
		want int64
		ok   bool
	}{
		{"personal user", &models.Message{From: &models.User{ID: 42}}, 42, true},
		{"nil message", nil, 0, false},
		{"nil from", &models.Message{}, 0, false},
		{"zero id", &models.Message{From: &models.User{ID: 0}}, 0, false},
		{"bot sender", &models.Message{From: &models.User{ID: 7, IsBot: true}}, 0, false},
		{
			// Telegram substitutes one global GroupAnonymousBot user for every
			// anonymous admin message; without this check they would all share
			// a single pack.
			"anonymous admin",
			&models.Message{From: &models.User{ID: 1087968824, IsBot: true}, SenderChat: &models.Chat{ID: -100}},
			0, false,
		},
		{
			// A channel post carries SenderChat with a non-bot From.
			"sender chat present",
			&models.Message{From: &models.User{ID: 42}, SenderChat: &models.Chat{ID: -100}},
			0, false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := senderID(tc.msg)
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("senderID = (%d, %v), want (%d, nil)", got, err, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("senderID = (%d, nil), want an error", got)
			}
		})
	}
}

// Denying without explaining leaves the user with no move; anonymous posting is
// a per-message toggle they can flip immediately.
func TestSenderRefusal_ExplainsTheFix(t *testing.T) {
	for _, want := range []string{"personal account", "anonymous"} {
		if !strings.Contains(senderRefusal, want) {
			t.Errorf("senderRefusal %q does not mention %q", senderRefusal, want)
		}
	}
}
