package sticker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

const (
	testUser = int64(42)
	testChat = int64(1000)
	testSet  = "mypack_by_testbot"
	otherSet = "someoneelse_by_testbot"
)

var fixedNow = time.UnixMilli(1_700_000_000_000)

// newTestState builds a state over one in-memory collection, matching
// production: both typed views share the collection, with disjoint key spaces.
func newTestState() *state {
	coll := storage.NewMemoryProvider().Collection("sticker")
	return &state{
		store:   storage.Typed[Pack](coll),
		pending: storage.Typed[PendingDelete](coll),
		slugs:   storage.Typed[SlugReservation](coll),
		nowFn:   func() time.Time { return fixedNow },
	}
}

func seedPack(t *testing.T, s *state, count int) Pack {
	t.Helper()
	pack := Pack{
		Slug: "mypack", Name: testSet, Title: "My Pack",
		OwnerID: testUser, Count: count, CreatedAt: fixedNow.UnixMilli(),
	}
	if err := s.store.Put(context.Background(), packKey(testUser), pack); err != nil {
		t.Fatalf("seed pack: %v", err)
	}
	// A real pack always carries its name reservation; seeding without one
	// would let tests pass against a state production cannot reach.
	if err := s.slugs.Put(context.Background(), slugKey(pack.Slug),
		SlugReservation{Slug: pack.Slug, OwnerID: testUser, CreatedAt: fixedNow.UnixMilli()}); err != nil {
		t.Fatalf("seed reservation: %v", err)
	}
	return pack
}

// stickerReply builds a message replying to a sticker in setName.
func stickerReply(text, setName string) *models.Update {
	upd := testutil.NewPrivateMessage(testUser, text)
	upd.Message.Chat.ID = testChat
	upd.Message.ReplyToMessage = &models.Message{
		Sticker: &models.Sticker{
			FileID:       "file-in-" + setName,
			FileUniqueID: "uniq",
			Type:         "regular",
			SetName:      setName,
			Emoji:        "🎉",
		},
	}
	return upd
}

func loadPack(t *testing.T, s *state) (Pack, bool) {
	t.Helper()
	pack, found, err := getPack(context.Background(), s.store, testUser)
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	return pack, found
}

// methodsSent lists the API methods a run produced, so a test can assert both
// what was called and that nothing was.
func methodsSent(rb *testutil.RecordingBot) []string {
	var out []string
	for _, call := range rb.Sent() {
		out = append(out, call.Method)
	}
	return out
}

func countMethod(rb *testutil.RecordingBot, method string) int {
	n := 0
	for _, call := range rb.Sent() {
		if call.Method == method {
			n++
		}
	}
	return n
}

func TestAddSticker_HappyPath(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 3)

	if err := s.handleAddSticker(context.Background(), rb.Bot, stickerReply("/addsticker 😂", otherSet)); err != nil {
		t.Fatalf("handleAddSticker: %v", err)
	}

	if countMethod(rb, "addStickerToSet") != 1 {
		t.Fatalf("methods = %v, want one addStickerToSet", methodsSent(rb))
	}
	for _, call := range rb.Sent() {
		if call.Method != "addStickerToSet" {
			continue
		}
		if call.Form["name"] != testSet {
			t.Errorf("name = %q, want %q", call.Form["name"], testSet)
		}
		// UserID is always the caller: a non-owner never reaches this call.
		if call.Form["user_id"] != "42" {
			t.Errorf("user_id = %q, want 42", call.Form["user_id"])
		}
		if !strings.Contains(call.Form["sticker"], "😂") {
			t.Errorf("sticker payload %q missing the explicit emoji", call.Form["sticker"])
		}
	}

	pack, _ := loadPack(t, s)
	if pack.Count != 4 {
		t.Errorf("Count = %d, want 4", pack.Count)
	}
}

// Explicit args beat the replied sticker's emoji, which beats the default.
// addedStickerPayload returns the payload of the one addStickerToSet call the
// handler is expected to have made.
//
// Ranging over rb.Sent() and asserting only inside an `if call.Method == ...`
// makes the assertion vacuous: a handler that returns early and never calls
// Telegram at all satisfies it, because the loop body never runs. Requiring
// exactly one call is what makes these tests fail when the call disappears.
func addedStickerPayload(t *testing.T, rb *testutil.RecordingBot) string {
	t.Helper()
	var payloads []string
	for _, call := range rb.Sent() {
		if call.Method == "addStickerToSet" {
			payloads = append(payloads, call.Form["sticker"])
		}
	}
	if len(payloads) != 1 {
		t.Fatalf("addStickerToSet calls = %d, want exactly 1", len(payloads))
	}
	return payloads[0]
}

func TestAddSticker_EmojiPrecedence(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"explicit wins", "/addsticker 🔥", "🔥"},
		{"inherits from replied sticker", "/addsticker", "🎉"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rb := testutil.NewRecordingBot(t)
			s := newTestState()
			seedPack(t, s, 0)

			if err := s.handleAddSticker(context.Background(), rb.Bot, stickerReply(tc.text, otherSet)); err != nil {
				t.Fatalf("handleAddSticker: %v", err)
			}
			if payload := addedStickerPayload(t, rb); !strings.Contains(payload, tc.want) {
				t.Errorf("sticker payload %q, want emoji %q", payload, tc.want)
			}
		})
	}
}

// With no emoji anywhere, the default keeps the call valid — Telegram rejects
// an empty emoji_list.
func TestAddSticker_FallsBackToDefaultEmoji(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 0)

	upd := stickerReply("/addsticker", otherSet)
	upd.Message.ReplyToMessage.Sticker.Emoji = ""
	if err := s.handleAddSticker(context.Background(), rb.Bot, upd); err != nil {
		t.Fatalf("handleAddSticker: %v", err)
	}
	if payload := addedStickerPayload(t, rb); !strings.Contains(payload, defaultEmoji) {
		t.Errorf("sticker payload %q, want the default emoji", payload)
	}
}

// A stray word is caught by parseEmoji. With no pack argument left, there is
// nothing else it could have been mistaken for.
func TestAddSticker_RejectsNonEmojiArgument(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 0)

	if err := s.handleAddSticker(context.Background(), rb.Bot, stickerReply("/addsticker mypack", otherSet)); err != nil {
		t.Fatalf("handleAddSticker: %v", err)
	}
	if countMethod(rb, "addStickerToSet") != 0 {
		t.Errorf("methods = %v, want no API call", methodsSent(rb))
	}
	if !strings.Contains(rb.LastSent().Text(), "not an emoji") {
		t.Errorf("reply = %q, want an emoji usage error", rb.LastSent().Text())
	}
}

func TestAddSticker_NoPackYet(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()

	if err := s.handleAddSticker(context.Background(), rb.Bot, stickerReply("/addsticker 😂", otherSet)); err != nil {
		t.Fatalf("handleAddSticker: %v", err)
	}
	if countMethod(rb, "addStickerToSet") != 0 {
		t.Errorf("methods = %v, want no API call", methodsSent(rb))
	}
	if !strings.Contains(rb.LastSent().Text(), "/newpack") {
		t.Errorf("reply = %q, want it to point at /newpack", rb.LastSent().Text())
	}
}

// A pending record means an unfinished /newpack: there is no usable pack yet,
// and the reply has to say how to finish it.
func TestAddSticker_PendingPackIsNotUsable(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	pack := seedPack(t, s, 0)
	pack.Pending = true
	if err := s.store.Put(context.Background(), packKey(testUser), pack); err != nil {
		t.Fatalf("seed pending: %v", err)
	}

	if err := s.handleAddSticker(context.Background(), rb.Bot, stickerReply("/addsticker 😂", otherSet)); err != nil {
		t.Fatalf("handleAddSticker: %v", err)
	}
	if countMethod(rb, "addStickerToSet") != 0 {
		t.Errorf("methods = %v, want no API call", methodsSent(rb))
	}
	if !strings.Contains(rb.LastSent().Text(), "incomplete") {
		t.Errorf("reply = %q, want the incomplete-pack hint", rb.LastSent().Text())
	}
}

func TestAddSticker_FullPack(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	rb.FailMethodCode("addStickerToSet", 400, "Bad Request: STICKERS_TOO_MUCH")
	s := newTestState()
	seedPack(t, s, maxStickersPerPack)

	if err := s.handleAddSticker(context.Background(), rb.Bot, stickerReply("/addsticker 😂", otherSet)); err != nil {
		t.Fatalf("handleAddSticker: %v", err)
	}
	if !strings.Contains(rb.LastSent().Text(), "full") {
		t.Errorf("reply = %q, want the pack-is-full message", rb.LastSent().Text())
	}
	// A failed add must not move the count.
	pack, _ := loadPack(t, s)
	if pack.Count != maxStickersPerPack {
		t.Errorf("Count = %d, want it unchanged at %d", pack.Count, maxStickersPerPack)
	}
}

func TestDelSticker_HappyPath(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 3)

	if err := s.handleDelSticker(context.Background(), rb.Bot, stickerReply("/delsticker", testSet)); err != nil {
		t.Fatalf("handleDelSticker: %v", err)
	}

	if countMethod(rb, "deleteStickerFromSet") != 1 {
		t.Fatalf("methods = %v, want exactly one deleteStickerFromSet", methodsSent(rb))
	}
	pack, found := loadPack(t, s)
	if !found {
		t.Fatal("pack record deleted by a successful /delsticker")
	}
	if pack.Count != 2 {
		t.Errorf("Count = %d, want 2", pack.Count)
	}
}

// R7: a transient failure must never destroy the record. The probe that would
// have done so was removed for exactly this reason.
func TestDelSticker_TransientErrorKeepsRecord(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	rb.FailMethod("deleteStickerFromSet", 500, `{"ok":false,"description":"server exploded"}`)
	s := newTestState()
	seedPack(t, s, 3)

	if err := s.handleDelSticker(context.Background(), rb.Bot, stickerReply("/delsticker", testSet)); err != nil {
		t.Fatalf("handleDelSticker: %v", err)
	}
	pack, found := loadPack(t, s)
	if !found {
		t.Fatal("a transient error deleted the pack record")
	}
	if pack.Count != 3 {
		t.Errorf("Count = %d, want it unchanged at 3", pack.Count)
	}
}

// The other half of the same rule: a *positive* STICKERSET_INVALID is the one
// signal that authorises dropping the record, and dropping it is what unblocks
// /newpack.
func TestDelSticker_SetGoneDropsRecord(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	rb.FailMethodCode("deleteStickerFromSet", 400, "Bad Request: STICKERSET_INVALID")
	s := newTestState()
	seedPack(t, s, 1)

	if err := s.handleDelSticker(context.Background(), rb.Bot, stickerReply("/delsticker", testSet)); err != nil {
		t.Fatalf("handleDelSticker: %v", err)
	}
	if _, found := loadPack(t, s); found {
		t.Error("record survived a positive STICKERSET_INVALID; /newpack stays blocked")
	}
}

// Deleting the last sticker may destroy the set Telegram-side, and /mypack
// makes no API calls so it cannot notice. The reply has to name the way out.
func TestDelSticker_EmptyPackNamesRecovery(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 1)

	if err := s.handleDelSticker(context.Background(), rb.Bot, stickerReply("/delsticker", testSet)); err != nil {
		t.Fatalf("handleDelSticker: %v", err)
	}
	text := rb.LastSent().Text()
	if !strings.Contains(text, "/delpack") {
		t.Errorf("reply = %q, want it to name /delpack", text)
	}
	pack, _ := loadPack(t, s)
	if pack.Count != 0 {
		t.Errorf("Count = %d, want 0", pack.Count)
	}
}

// Count is floored: a drifted record must not go negative.
func TestDelSticker_CountFlooredAtZero(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 0)

	if err := s.handleDelSticker(context.Background(), rb.Bot, stickerReply("/delsticker", testSet)); err != nil {
		t.Fatalf("handleDelSticker: %v", err)
	}
	pack, _ := loadPack(t, s)
	if pack.Count != 0 {
		t.Errorf("Count = %d, want 0", pack.Count)
	}
}

func TestEditSticker_HappyPath(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 1)

	if err := s.handleEditSticker(context.Background(), rb.Bot, stickerReply("/editsticker 😂🔥", testSet)); err != nil {
		t.Fatalf("handleEditSticker: %v", err)
	}
	if countMethod(rb, "setStickerEmojiList") != 1 {
		t.Fatalf("methods = %v, want one setStickerEmojiList", methodsSent(rb))
	}
}

// An empty emoji_list is invalid, so unlike /addsticker this cannot fall back
// to a default — the user has to say what they want.
func TestEditSticker_RequiresEmoji(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 1)

	if err := s.handleEditSticker(context.Background(), rb.Bot, stickerReply("/editsticker", testSet)); err != nil {
		t.Fatalf("handleEditSticker: %v", err)
	}
	if countMethod(rb, "setStickerEmojiList") != 0 {
		t.Errorf("methods = %v, want no API call", methodsSent(rb))
	}
}

func TestOrderSticker(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		wantAPI int
	}{
		{"zero is valid", "/ordersticker 0", 1},
		// Not bounded locally: Telegram validates against the current set size
		// and a local copy would go stale.
		{"large position reaches the API", "/ordersticker 999", 1},
		{"negative rejected locally", "/ordersticker -1", 0},
		{"non-numeric rejected locally", "/ordersticker first", 0},
		{"missing argument rejected locally", "/ordersticker", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rb := testutil.NewRecordingBot(t)
			s := newTestState()
			seedPack(t, s, 5)

			if err := s.handleOrderSticker(context.Background(), rb.Bot, stickerReply(tc.text, testSet)); err != nil {
				t.Fatalf("handleOrderSticker: %v", err)
			}
			if got := countMethod(rb, "setStickerPositionInSet"); got != tc.wantAPI {
				t.Errorf("setStickerPositionInSet calls = %d, want %d (methods %v)", got, tc.wantAPI, methodsSent(rb))
			}
		})
	}
}

// The self-heal path frees the name too: STICKERSET_INVALID is a positive
// "this set is gone", so there is nothing left for the reservation to protect.
func TestSelfHeal_ReleasesTheName(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	rb.FailMethodCode("addStickerToSet", 400, "Bad Request: STICKERSET_INVALID")
	s := newTestState()
	seedPack(t, s, 2)
	ctx := context.Background()

	if err := s.handleAddSticker(ctx, rb.Bot, stickerReply("/addsticker 😂", otherSet)); err != nil {
		t.Fatalf("handleAddSticker: %v", err)
	}
	if _, found := loadPack(t, s); found {
		t.Error("pack record survived a positive STICKERSET_INVALID")
	}
	if _, held, _ := getSlugReservation(ctx, s.slugs, "mypack"); held {
		t.Error("the name is still reserved after the set was confirmed gone")
	}
}
