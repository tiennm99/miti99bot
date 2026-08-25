package sticker

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/testutil"
)

// resolveOwnedText runs the gate and returns the refusal the user would see.
func resolveOwnedText(t *testing.T, s *state, upd *models.Update) string {
	t.Helper()
	_, err := s.resolveOwned(context.Background(), upd.Message, testUser)
	if err == nil {
		t.Fatal("resolveOwned succeeded; want a refusal")
	}
	ue, ok := err.(userError)
	if !ok {
		t.Fatalf("err is %T (%v), want userError", err, err)
	}
	return ue.msg
}

// The point of the uniform refusal: "you have no pack" and "that set is not
// yours" must be indistinguishable, or a user can probe which sets exist under
// this bot by elimination.
//
// This asserts the two replies against *each other* rather than against fixed
// strings, so rewording the copy cannot silently reintroduce the disclosure.
func TestResolveOwned_RefusalsAreIdentical(t *testing.T) {
	noPack := newTestState()
	noPackText := resolveOwnedText(t, noPack, stickerReply("/delsticker", otherSet))

	withPack := newTestState()
	seedPack(t, withPack, 2)
	foreignText := resolveOwnedText(t, withPack, stickerReply("/delsticker", otherSet))

	if noPackText != foreignText {
		t.Errorf("refusals differ and leak whether a set exists:\n no pack: %q\n foreign: %q", noPackText, foreignText)
	}
}

// A pending record is not a usable pack, and must refuse identically too.
func TestResolveOwned_PendingRefusesIdentically(t *testing.T) {
	noPack := newTestState()
	noPackText := resolveOwnedText(t, noPack, stickerReply("/delsticker", otherSet))

	pendingState := newTestState()
	pack := seedPack(t, pendingState, 0)
	pack.Pending = true
	if err := pendingState.store.Put(context.Background(), packKey(testUser), pack); err != nil {
		t.Fatalf("seed pending: %v", err)
	}
	pendingText := resolveOwnedText(t, pendingState, stickerReply("/delsticker", testSet))

	if noPackText != pendingText {
		t.Errorf("pending refusal differs:\n no pack: %q\n pending: %q", noPackText, pendingText)
	}
}

// Telegram echoes SetName with the casing the set was created with, so
// ownership has to fold case or a user's own pack stops resolving.
func TestResolveOwned_CaseInsensitiveSetName(t *testing.T) {
	s := newTestState()
	seedPack(t, s, 1)

	owned, err := s.resolveOwned(context.Background(), stickerReply("/delsticker", "MyPack_By_TestBot").Message, testUser)
	if err != nil {
		t.Fatalf("resolveOwned with differing case: %v", err)
	}
	if owned.pack.Name != testSet {
		t.Errorf("pack.Name = %q, want %q", owned.pack.Name, testSet)
	}
}

func TestResolveOwned_UsageErrors(t *testing.T) {
	s := newTestState()
	seedPack(t, s, 1)

	t.Run("no reply", func(t *testing.T) {
		upd := testutil.NewPrivateMessage(testUser, "/delsticker")
		if _, err := s.resolveOwned(context.Background(), upd.Message, testUser); err == nil {
			t.Error("resolveOwned with no reply succeeded")
		}
	})

	t.Run("reply is not a sticker", func(t *testing.T) {
		upd := testutil.NewPrivateMessage(testUser, "/delsticker")
		upd.Message.ReplyToMessage = &models.Message{Text: "hello"}
		if _, err := s.resolveOwned(context.Background(), upd.Message, testUser); err == nil {
			t.Error("resolveOwned with a text reply succeeded")
		}
	})

	t.Run("sticker has no set", func(t *testing.T) {
		upd := stickerReply("/delsticker", testSet)
		upd.Message.ReplyToMessage.Sticker.SetName = ""
		if _, err := s.resolveOwned(context.Background(), upd.Message, testUser); err == nil {
			t.Error("resolveOwned with an empty set_name succeeded")
		}
	})
}

// The static-only gate. IsAnimated/IsVideo are the obvious half; Type is the
// half a boolean-only check misses — a mask sticker is static yet invalid in a
// regular set.
func TestRequireStaticSticker(t *testing.T) {
	cases := []struct {
		name    string
		sticker models.Sticker
		ok      bool
	}{
		{"regular", models.Sticker{Type: "regular"}, true},
		{"type absent", models.Sticker{}, true},
		{"animated", models.Sticker{Type: "regular", IsAnimated: true}, false},
		{"video", models.Sticker{Type: "regular", IsVideo: true}, false},
		{"mask", models.Sticker{Type: "mask"}, false},
		{"custom emoji", models.Sticker{Type: "custom_emoji"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireStaticSticker(&tc.sticker)
			if tc.ok && err != nil {
				t.Errorf("requireStaticSticker = %v, want nil", err)
			}
			if !tc.ok && err == nil {
				t.Error("requireStaticSticker = nil, want a refusal")
			}
		})
	}
}

// Both entry points must enforce it: resolveSource is the one that can actually
// receive a non-static sticker, and neither may reach an API call.
func TestStaticGate_BlocksBothPathsBeforeAnyAPICall(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*models.Sticker)
		setName string
	}{
		{"animated source", func(st *models.Sticker) { st.IsAnimated = true }, otherSet},
		{"video source", func(st *models.Sticker) { st.IsVideo = true }, otherSet},
		{"mask source", func(st *models.Sticker) { st.Type = "mask" }, otherSet},
		{"animated owned", func(st *models.Sticker) { st.IsAnimated = true }, testSet},
		{"mask owned", func(st *models.Sticker) { st.Type = "mask" }, testSet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rb := testutil.NewRecordingBot(t)
			s := newTestState()
			seedPack(t, s, 1)

			upd := stickerReply("/addsticker", tc.setName)
			tc.mutate(upd.Message.ReplyToMessage.Sticker)
			if err := s.handleAddSticker(context.Background(), rb.Bot, upd); err != nil {
				t.Fatalf("handleAddSticker: %v", err)
			}
			if countMethod(rb, "addStickerToSet") != 0 {
				t.Errorf("methods = %v, want no API call", methodsSent(rb))
			}

			rb2 := testutil.NewRecordingBot(t)
			if err := s.handleDelSticker(context.Background(), rb2.Bot, upd); err != nil {
				t.Fatalf("handleDelSticker: %v", err)
			}
			if countMethod(rb2, "deleteStickerFromSet") != 0 {
				t.Errorf("methods = %v, want no API call", methodsSent(rb2))
			}
		})
	}
}

// A replied sticker contributes at most one emoji, because models.Sticker.Emoji
// is a single string.
func TestResolveSource_InheritsSingleEmoji(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()

	src, err := s.resolveSource(context.Background(), rb.Bot, testUser, stickerReply("/addsticker", otherSet).Message)
	if err != nil {
		t.Fatalf("resolveSource: %v", err)
	}
	if len(src.emoji) != 1 || src.emoji[0] != "🎉" {
		t.Errorf("emoji = %q, want exactly one 🎉", src.emoji)
	}
	if src.fileID == "" {
		t.Error("fileID is empty")
	}
}
