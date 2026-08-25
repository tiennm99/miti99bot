package sticker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/testutil"
)

const promptMessageID = 555

// seedPendingDelete stores a confirm action as /delpack would have.
func seedPendingDelete(t *testing.T, s *state, mutate func(*PendingDelete)) PendingDelete {
	t.Helper()
	action := PendingDelete{
		ID:        "abc123",
		OwnerID:   testUser,
		Slug:      "mypack",
		SetName:   testSet,
		ChatID:    testChat,
		MessageID: promptMessageID,
		CreatedAt: fixedNow.UnixMilli(),
		ExpiresAt: fixedNow.Add(pendingDeleteTTL).UnixMilli(),
	}
	if mutate != nil {
		mutate(&action)
	}
	if err := s.pending.Put(context.Background(), pendingDeleteKey(action.OwnerID), action); err != nil {
		t.Fatalf("seed pending delete: %v", err)
	}
	return action
}

// confirmPress builds the callback update for pressing the confirm button.
func confirmPress(action PendingDelete, presser int64) *models.Update {
	return &models.Update{CallbackQuery: &models.CallbackQuery{
		ID:   "cbq-1",
		From: models.User{ID: presser},
		Data: deleteCallbackData(action.ID),
		Message: models.MaybeInaccessibleMessage{
			Message: &models.Message{
				ID:   action.MessageID,
				Chat: models.Chat{ID: action.ChatID},
			},
		},
	}}
}

// The prompt is the last point at which a user changing their pack's URL learns
// the stickers do not survive it, so it must state all four consequences.
func TestDelPack_PromptStatesConsequences(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 47)

	if err := s.handleDelPack(context.Background(), rb.Bot, testutil.NewPrivateMessage(testUser, "/delpack")); err != nil {
		t.Fatalf("handleDelPack: %v", err)
	}

	text := rb.LastSent().Text()
	for _, want := range []string{"My Pack", "47", shareLink(testSet), "permanent"} {
		if !strings.Contains(text, want) {
			t.Errorf("confirm prompt %q missing %q", text, want)
		}
	}
	if countMethod(rb, "deleteStickerSet") != 0 {
		t.Error("/delpack deleted without confirmation")
	}
}

func TestDelPack_CallbackDataFitsTelegramLimit(t *testing.T) {
	id, err := newActionID()
	if err != nil {
		t.Fatalf("newActionID: %v", err)
	}
	data := deleteCallbackData(id)
	if len(data) > maxCallbackBytes {
		t.Errorf("callback data is %d bytes, over the %d-byte limit", len(data), maxCallbackBytes)
	}
	got, ok := parseDeleteCallback(data)
	if !ok || got != id {
		t.Errorf("parseDeleteCallback(%q) = (%q, %v), want (%q, true)", data, got, ok, id)
	}
}

func TestDelPackCallback_HappyPath(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 3)
	action := seedPendingDelete(t, s, nil)

	if err := s.handleDelPackCallback(context.Background(), rb.Bot, confirmPress(action, testUser)); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if countMethod(rb, "deleteStickerSet") != 1 {
		t.Fatalf("methods = %v, want one deleteStickerSet", methodsSent(rb))
	}
	if _, found := loadPack(t, s); found {
		t.Error("pack record survived a confirmed delete")
	}
}

// Identity comes from From.ID, never from the payload — the payload is
// client-controlled.
// A foreign presser gets nothing. The mechanism is the key, not the owner
// comparison: the action is loaded by the presser's own id, so someone else's
// press finds no action at all and returns before the owner check is reached.
//
// Named for that, because the previous name claimed to exercise the owner
// comparison at delpack_callback.go and did not — that branch is unreachable
// by construction, and is kept only as defence in depth.
func TestDelPackCallback_ForeignPresserResolvesToNothing(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 3)
	action := seedPendingDelete(t, s, nil)

	if err := s.handleDelPackCallback(context.Background(), rb.Bot, confirmPress(action, testUser+1)); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if countMethod(rb, "deleteStickerSet") != 0 {
		t.Errorf("methods = %v, want no delete", methodsSent(rb))
	}
	if _, found := loadPack(t, s); !found {
		t.Error("another user's press deleted the pack record")
	}
}

func TestDelPackCallback_RejectsExpired(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 3)
	action := seedPendingDelete(t, s, func(a *PendingDelete) {
		a.ExpiresAt = fixedNow.Add(-time.Second).UnixMilli()
	})

	if err := s.handleDelPackCallback(context.Background(), rb.Bot, confirmPress(action, testUser)); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if countMethod(rb, "deleteStickerSet") != 0 {
		t.Errorf("methods = %v, want no delete after expiry", methodsSent(rb))
	}
	if _, found := loadPack(t, s); !found {
		t.Error("an expired press deleted the pack record")
	}
}

// The action is bound to the message carrying the button, so a press arriving
// from anywhere else resolves to nothing.
func TestDelPackCallback_RejectsWrongBinding(t *testing.T) {
	cases := map[string]func(*models.Update){
		"different chat":    func(u *models.Update) { u.CallbackQuery.Message.Message.Chat.ID = testChat + 1 },
		"different message": func(u *models.Update) { u.CallbackQuery.Message.Message.ID = promptMessageID + 1 },
		// MaybeInaccessibleMessage is nil for messages Telegram marks
		// inaccessible; the panic barrier is a backstop, not a substitute.
		"inaccessible message": func(u *models.Update) { u.CallbackQuery.Message.Message = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			rb := testutil.NewRecordingBot(t)
			s := newTestState()
			seedPack(t, s, 3)
			action := seedPendingDelete(t, s, nil)

			upd := confirmPress(action, testUser)
			mutate(upd)

			if err := s.handleDelPackCallback(context.Background(), rb.Bot, upd); err != nil {
				t.Fatalf("callback: %v", err)
			}
			if countMethod(rb, "deleteStickerSet") != 0 {
				t.Errorf("methods = %v, want no delete", methodsSent(rb))
			}
			if _, found := loadPack(t, s); !found {
				t.Error("pack record deleted despite a broken binding")
			}
		})
	}
}

// Single use: the action is consumed before the destructive call, so a second
// press finds nothing.
func TestDelPackCallback_SecondPressIsInert(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 3)
	action := seedPendingDelete(t, s, nil)

	press := func() {
		if err := s.handleDelPackCallback(context.Background(), rb.Bot, confirmPress(action, testUser)); err != nil {
			t.Fatalf("callback: %v", err)
		}
	}
	press()
	press()

	if got := countMethod(rb, "deleteStickerSet"); got != 1 {
		t.Errorf("deleteStickerSet calls = %d, want exactly 1", got)
	}
}

// A set Telegram already lost still clears the record — that is what unblocks
// /newpack after the phantom-record failure mode.
func TestDelPackCallback_MissingSetStillClearsRecord(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	rb.FailMethodCode("deleteStickerSet", 400, "Bad Request: STICKERSET_INVALID")
	s := newTestState()
	seedPack(t, s, 3)
	action := seedPendingDelete(t, s, nil)

	if err := s.handleDelPackCallback(context.Background(), rb.Bot, confirmPress(action, testUser)); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if _, found := loadPack(t, s); found {
		t.Error("record survived; /newpack stays blocked")
	}
}

// The mirror image: an unclassifiable failure leaves the pack alone.
func TestDelPackCallback_TransientErrorKeepsRecord(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	rb.FailMethod("deleteStickerSet", 500, `{"ok":false,"description":"upstream is unhappy"}`)
	s := newTestState()
	seedPack(t, s, 3)
	action := seedPendingDelete(t, s, nil)

	if err := s.handleDelPackCallback(context.Background(), rb.Bot, confirmPress(action, testUser)); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if _, found := loadPack(t, s); !found {
		t.Error("a transient delete failure destroyed the pack record")
	}
}

func TestDelPackCallback_IgnoresNonCallbackUpdate(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	if err := s.handleDelPackCallback(context.Background(), rb.Bot, &models.Update{}); err != nil {
		t.Fatalf("callback with no query: %v", err)
	}
	if len(rb.Sent()) != 0 {
		t.Errorf("methods = %v, want none", methodsSent(rb))
	}
}

// The stale-confirmation regression. /delpack is the documented route to a new
// pack URL (delete, then /newpack under a new name), so a user genuinely can
// have an old prompt in scrollback while holding a *different*, live pack.
//
// Pressing the stale button deleted the old set, got STICKERSET_INVALID, and
// then cleared the record by owner id — erasing the record of the new, live
// pack and orphaning it permanently.
func TestDelPackCallback_StalePressLeavesTheCurrentPackAlone(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	// The old set no longer exists: this is the deleted-then-recreated flow.
	rb.FailMethodCode("deleteStickerSet", 400, "Bad Request: STICKERSET_INVALID")
	s := newTestState()
	ctx := context.Background()

	// A confirmation written for the *old* pack.
	stale := seedPendingDelete(t, s, func(a *PendingDelete) {
		a.Slug = "oldslug"
		a.SetName = "oldslug_by_testbot"
	})

	// The user has since created a new pack.
	current := Pack{Slug: "newslug", Name: "newslug_by_testbot", Title: "New", OwnerID: testUser, Count: 7}
	if err := s.store.Put(ctx, packKey(testUser), current); err != nil {
		t.Fatalf("seed current pack: %v", err)
	}

	if err := s.handleDelPackCallback(ctx, rb.Bot, confirmPress(stale, testUser)); err != nil {
		t.Fatalf("callback: %v", err)
	}

	pack, found := loadPack(t, s)
	if !found {
		t.Fatal("the live pack's record was deleted by a stale confirmation")
	}
	if pack.Name != current.Name || pack.Count != 7 {
		t.Errorf("pack = %+v, want the live newslug record untouched", pack)
	}
}

// Two /delpack runs must not leave two live capabilities. The second prompt
// supersedes the first, and pressing the first afterwards does nothing.
func TestDelPack_SecondPromptSupersedesTheFirst(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 3)
	ctx := context.Background()

	run := func() PendingDelete {
		if err := s.handleDelPack(ctx, rb.Bot, testutil.NewPrivateMessage(testUser, "/delpack")); err != nil {
			t.Fatalf("handleDelPack: %v", err)
		}
		action, _, err := s.pending.Get(ctx, pendingDeleteKey(testUser))
		if err != nil {
			t.Fatalf("load action: %v", err)
		}
		return action
	}
	first := run()
	second := run()

	if first.ID == second.ID {
		t.Fatal("both prompts share an id; the test cannot distinguish them")
	}
	// Exactly one action is stored, not two.
	keys, err := s.pending.List(ctx, pendingDeletePrefix)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("stored pending actions = %d, want 1 — a public command must not accumulate documents", len(keys))
	}

	// The superseded button is inert.
	if err := s.handleDelPackCallback(ctx, rb.Bot, confirmPress(first, testUser)); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if countMethod(rb, "deleteStickerSet") != 0 {
		t.Errorf("methods = %v, want the stale button to delete nothing", methodsSent(rb))
	}
	if _, found := loadPack(t, s); !found {
		t.Error("the superseded button deleted the pack")
	}
}

// Anyone in a group can tap anyone's inline button. Checking supersession
// before the chat/message binding meant a bystander's press stripped the button
// off a live confirmation they had no part in — no data leak, but the victim's
// prompt was destroyed and they had to start over.
func TestDelPackCallback_BystanderCannotTouchAnotherUsersPrompt(t *testing.T) {
	const bystander = int64(2)

	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 3)
	ctx := context.Background()

	victimAction := seedPendingDelete(t, s, nil)

	// The bystander has their own pending confirmation, bound to their own
	// message — this is what made action.ID differ and triggered the clear.
	bystanderAction := PendingDelete{
		ID: "bbbb2222", OwnerID: bystander, Slug: "theirs", SetName: "theirs_by_testbot",
		ChatID: testChat, MessageID: 777,
		CreatedAt: fixedNow.UnixMilli(), ExpiresAt: fixedNow.Add(pendingDeleteTTL).UnixMilli(),
	}
	if err := s.pending.Put(ctx, pendingDeleteKey(bystander), bystanderAction); err != nil {
		t.Fatalf("seed bystander action: %v", err)
	}

	// The bystander presses the victim's button.
	if err := s.handleDelPackCallback(ctx, rb.Bot, confirmPress(victimAction, bystander)); err != nil {
		t.Fatalf("callback: %v", err)
	}

	if countMethod(rb, "editMessageReplyMarkup") != 0 {
		t.Errorf("methods = %v; a bystander cleared the button on someone else's prompt", methodsSent(rb))
	}
	if countMethod(rb, "deleteStickerSet") != 0 {
		t.Errorf("methods = %v, want no delete", methodsSent(rb))
	}
	// The victim's confirmation is untouched and still usable.
	if _, _, err := s.pending.Get(ctx, pendingDeleteKey(testUser)); err != nil {
		t.Errorf("victim's pending action was consumed by a bystander's press: %v", err)
	}
}

// A deleted pack must give its name back. Holding it forever would shrink the
// global namespace permanently and let a /newpack + /delpack loop burn one name
// per cycle.
func TestDelPackCallback_ReleasesTheName(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 3)
	ctx := context.Background()
	action := seedPendingDelete(t, s, nil)

	if _, held, _ := getSlugReservation(ctx, s.slugs, "mypack"); !held {
		t.Fatal("fixture is wrong: the pack should start with its name reserved")
	}

	if err := s.handleDelPackCallback(ctx, rb.Bot, confirmPress(action, testUser)); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if _, held, _ := getSlugReservation(ctx, s.slugs, "mypack"); held {
		t.Error("the name is still reserved after the pack was deleted")
	}
}
