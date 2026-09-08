package alias

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

// attrs renders replyShape's key/value pairs into one string for assertions.
func attrs(t *testing.T, replied *models.Message) string {
	t.Helper()
	entry, ok := capture(replied)
	pairs := replyShape(replied, entry, ok)
	if len(pairs)%2 != 0 {
		t.Fatalf("replyShape returned %d values, want key/value pairs", len(pairs))
	}
	var sb strings.Builder
	for i := 0; i < len(pairs); i += 2 {
		fmt.Fprintf(&sb, "%v=%v ", pairs[i], pairs[i+1])
	}
	return sb.String()
}

func TestReplyShape(t *testing.T) {
	cases := []struct {
		name    string
		replied *models.Message
		want    []string
	}{
		{
			// The case this exists for: /alias arrives with no reply attached,
			// which looks the same to the caller as forgetting to reply.
			name:    "absent reply",
			replied: nil,
			want:    []string{"reply=absent"},
		},
		{
			// What another bot's message looks like once Telegram strips it:
			// delivered, from a bot, carrying nothing.
			name:    "stripped bot reply",
			replied: &models.Message{ID: 9, From: &models.User{ID: 555, IsBot: true}},
			want:    []string{"reply=present", "fields=none", "captured=false", "from_bot=true", "from_id=555"},
		},
		{
			name: "text reply",
			replied: &models.Message{
				ID:       4,
				From:     &models.User{ID: 7},
				Text:     "hello",
				Entities: []models.MessageEntity{{Type: models.MessageEntityTypeBold, Length: 5}},
			},
			want: []string{"fields=text", "text_len=5", "entities=1", "captured=true", "kind=text", "from_bot=false"},
		},
		{
			// A kind capture refuses still reports what arrived, so
			// "unsupported" can be told apart from "empty".
			name:    "unsupported kind",
			replied: &models.Message{ID: 5, From: &models.User{ID: 7}, Location: &models.Location{Latitude: 1}},
			want:    []string{"fields=location", "captured=false"},
		},
		{
			name: "photo with caption",
			replied: &models.Message{
				ID:      6,
				From:    &models.User{ID: 7},
				Photo:   []models.PhotoSize{{FileID: "p"}},
				Caption: "cap",
			},
			want: []string{"caption", "photo", "caption_len=3", "kind=photo"},
		},
		{
			// No From at all — an anonymous channel post, or a stripped reply.
			name:    "no sender",
			replied: &models.Message{ID: 7, SenderChat: &models.Chat{ID: -100}},
			want:    []string{"from=absent", "sender_chat=-100"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := attrs(t, tc.replied)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("replyShape = %q, missing %q", got, want)
				}
			}
		})
	}
}

// The debug line reports shape, never content: it lands in stdout and whatever
// ships it, so aliased message text must not travel with it.
func TestReplyShape_LogsNoMessageContent(t *testing.T) {
	const secret = "SENSITIVE-MESSAGE-BODY"
	replied := &models.Message{
		ID:      1,
		From:    &models.User{ID: 7, FirstName: secret, Username: secret},
		Text:    secret,
		Caption: secret,
	}

	if got := attrs(t, replied); strings.Contains(got, secret) {
		t.Errorf("replyShape leaked message content: %q", got)
	}
}
