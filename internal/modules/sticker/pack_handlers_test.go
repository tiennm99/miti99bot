package sticker

import (
	"context"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/testutil"
)

const getMeResult = `{"id":7,"is_bot":true,"first_name":"Test","username":"testbot"}`

// stubBotIdentity makes the username resolver work. The bot starts with
// WithSkipGetMe, so nothing populates a username until the module asks.
func stubBotIdentity(rb *testutil.RecordingBot) { rb.StubMethod("getMe", getMeResult) }

// setMissing makes getStickerSet report the set does not exist — the only
// classification that lets the module treat a slug as free.
func setMissing(rb *testutil.RecordingBot) {
	rb.FailMethodCode("getStickerSet", 400, "Bad Request: STICKERSET_INVALID")
}

// setExists makes getStickerSet return a real set, which needs a struct result
// the bare harness cannot produce.
func setExists(rb *testutil.RecordingBot) {
	rb.StubMethod("getStickerSet", `{"name":"`+testSet+`","title":"My Pack","sticker_type":"regular","stickers":[]}`)
}

func TestNewPack_HappyPath(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setMissing(rb)
	s := newTestState()

	if err := s.handleNewPack(context.Background(), rb.Bot, stickerReply("/newpack mypack My Pack", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}

	if countMethod(rb, "createNewStickerSet") != 1 {
		t.Fatalf("methods = %v, want one createNewStickerSet", methodsSent(rb))
	}
	pack, found := loadPack(t, s)
	if !found {
		t.Fatal("no pack record after a successful /newpack")
	}
	if pack.Pending {
		t.Error("record is still Pending after success")
	}
	if pack.Name != testSet || pack.Count != 1 {
		t.Errorf("pack = %+v, want name %q and count 1", pack, testSet)
	}
	if !strings.Contains(rb.LastSent().Text(), shareLink(testSet)) {
		t.Errorf("reply = %q, want the share link", rb.LastSent().Text())
	}
}

// The quota is the create-only write itself: there is no separate counter, so a
// second /newpack must lose on PutVersioned and never reach the API.
func TestNewPack_SecondPackRefusedWithoutAPICall(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	s := newTestState()
	seedPack(t, s, 5)

	if err := s.handleNewPack(context.Background(), rb.Bot, stickerReply("/newpack another Another", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}
	if countMethod(rb, "createNewStickerSet") != 0 || countMethod(rb, "getStickerSet") != 0 {
		t.Errorf("methods = %v, want no sticker-set API calls", methodsSent(rb))
	}
	text := rb.LastSent().Text()
	if !strings.Contains(text, "mypack") || !strings.Contains(text, "/delpack") {
		t.Errorf("reply = %q, want it to name the existing slug and /delpack", text)
	}
}

// Re-running the same command after an interruption must complete the pack,
// not report the slug taken. This is what keeps a crash from stranding a
// permanent URL.
func TestNewPack_ResumesInterruptedAttempt(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setExists(rb)
	s := newTestState()

	seedInterrupted(t, s, "mypack", testSet)

	if err := s.handleNewPack(context.Background(), rb.Bot, stickerReply("/newpack mypack My Pack", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}

	// The set already exists, so it is adopted rather than recreated.
	if countMethod(rb, "createNewStickerSet") != 0 {
		t.Errorf("methods = %v, want no create — the set already existed", methodsSent(rb))
	}
	pack, _ := loadPack(t, s)
	if pack.Pending {
		t.Error("record still Pending after the resumed attempt")
	}
	if strings.Contains(rb.LastSent().Text(), "taken") {
		t.Errorf("reply = %q, want a success message", rb.LastSent().Text())
	}
}

// A pending record under a *different* slug whose set exists must be adopted,
// not overwritten: overwriting orphans that set permanently, because adoption
// keys on the slug matching.
func TestNewPack_DifferentSlugAdoptsExistingSet(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setExists(rb)
	s := newTestState()

	seedInterrupted(t, s, "oldslug", testSet)

	if err := s.handleNewPack(context.Background(), rb.Bot, stickerReply("/newpack newslug New", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}

	if countMethod(rb, "createNewStickerSet") != 0 {
		t.Errorf("methods = %v, want no create", methodsSent(rb))
	}
	pack, _ := loadPack(t, s)
	if pack.Slug != "oldslug" {
		t.Errorf("slug = %q, want the adopted oldslug — the old set must not be orphaned", pack.Slug)
	}
	if pack.Pending {
		t.Error("adopted record still Pending")
	}
	if !strings.Contains(rb.LastSent().Text(), "oldslug") {
		t.Errorf("reply = %q, want it to name the adopted pack", rb.LastSent().Text())
	}
}

// Same shape, but nothing was created under the old name: the record is free to
// take over.
func TestNewPack_DifferentSlugReplacesDeadIntent(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setMissing(rb)
	s := newTestState()

	seedInterrupted(t, s, "oldslug", "oldslug_by_testbot")

	if err := s.handleNewPack(context.Background(), rb.Bot, stickerReply("/newpack mypack My Pack", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}
	if countMethod(rb, "createNewStickerSet") != 1 {
		t.Fatalf("methods = %v, want one create", methodsSent(rb))
	}
	pack, _ := loadPack(t, s)
	if pack.Slug != "mypack" || pack.Pending {
		t.Errorf("pack = %+v, want a confirmed mypack record", pack)
	}
}

// An unclassifiable getStickerSet failure means "unknown". Guessing either way
// is what strands slugs or orphans sets, so the handler must change nothing.
func TestNewPack_UnknownLookupErrorAborts(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	rb.FailMethod("getStickerSet", 500, `{"ok":false,"description":"upstream is unhappy"}`)
	s := newTestState()

	if err := s.handleNewPack(context.Background(), rb.Bot, stickerReply("/newpack mypack My Pack", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}
	if countMethod(rb, "createNewStickerSet") != 0 {
		t.Errorf("methods = %v, want no create after an unknown error", methodsSent(rb))
	}
	// The intent and reservation must SURVIVE. "Unknown" means the set may
	// exist; destroying either here is what permanently strands a slug. Keeping
	// them is what makes re-running the same command recover.
	pack, found := loadPack(t, s)
	if !found || !pack.Pending {
		t.Errorf("intent = (%+v, found=%v), want it kept and still pending for re-run recovery", pack, found)
	}
	if _, held, _ := getSlugReservation(context.Background(), s.slugs, "mypack"); !held {
		t.Error("reservation dropped on an unknown error; another user could take the name while the set may exist")
	}
}

// Telegram itself refusing the name — NOT the "another user of this bot holds
// it" case, which the reservation now settles before any API call (see
// TestNewPack_ForeignReservationRefusedBeforeAnyAPICall).
//
// The reachable path here is a short name Telegram still reserves after a
// delete (plan R11): our reservation is free, GetStickerSet says missing, and
// createNewStickerSet refuses. A classified refusal proves nothing was created,
// so both the intent and the reservation are released for a retry.
func TestNewPack_OccupiedSlugDropsIntent(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setMissing(rb)
	rb.FailMethodCode("createNewStickerSet", 400, "Bad Request: PACK_SHORT_NAME_OCCUPIED")
	s := newTestState()

	if err := s.handleNewPack(context.Background(), rb.Bot, stickerReply("/newpack mypack My Pack", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}
	if _, found := loadPack(t, s); found {
		t.Error("intent survived a rejected create")
	}
	if _, held, _ := getSlugReservation(context.Background(), s.slugs, "mypack"); held {
		t.Error("reservation survived a classified refusal; the name would be held with no set behind it")
	}
	if !strings.Contains(rb.LastSent().Text(), "taken") {
		t.Errorf("reply = %q, want the slug-taken message", rb.LastSent().Text())
	}
}

func TestNewPack_ValidatesInput(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"no arguments", "/newpack"},
		{"slug only", "/newpack mypack"},
		{"leading digit", "/newpack 1pack Title"},
		{"double underscore", "/newpack my__pack Title"},
		{"trailing underscore", "/newpack mypack_ Title"},
		{"too short", "/newpack ab Title"},
		{"title too long", "/newpack mypack " + strings.Repeat("x", maxTitleLen+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rb := testutil.NewRecordingBot(t)
			stubBotIdentity(rb)
			setMissing(rb)
			s := newTestState()

			if err := s.handleNewPack(context.Background(), rb.Bot, stickerReply(tc.text, otherSet)); err != nil {
				t.Fatalf("handleNewPack: %v", err)
			}
			if countMethod(rb, "createNewStickerSet") != 0 {
				t.Errorf("methods = %v, want rejection before any create", methodsSent(rb))
			}
			if _, found := loadPack(t, s); found {
				t.Error("a rejected /newpack wrote a record")
			}
		})
	}
}

func TestMyPack_MakesNoAPICalls(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 7)

	upd := testutil.NewPrivateMessage(testUser, "/mypack")
	if err := s.handleMyPack(context.Background(), rb.Bot, upd); err != nil {
		t.Fatalf("handleMyPack: %v", err)
	}

	for _, call := range rb.Sent() {
		if call.Method != "sendMessage" {
			t.Errorf("unexpected API call %q; /mypack must read only the store", call.Method)
		}
	}
	text := rb.LastSent().Text()
	for _, want := range []string{"My Pack", "mypack", "7", shareLink(testSet)} {
		if !strings.Contains(text, want) {
			t.Errorf("reply %q missing %q", text, want)
		}
	}
}

// A stranded attempt is shown, not hidden: it blocks /newpack, and re-running
// the same command is what clears it.
func TestMyPack_ShowsPendingMarker(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	pack := seedPack(t, s, 0)
	pack.Pending = true
	if err := s.store.Put(context.Background(), packKey(testUser), pack); err != nil {
		t.Fatalf("seed pending: %v", err)
	}

	if err := s.handleMyPack(context.Background(), rb.Bot, testutil.NewPrivateMessage(testUser, "/mypack")); err != nil {
		t.Fatalf("handleMyPack: %v", err)
	}
	if !strings.Contains(rb.LastSent().Text(), "incomplete") {
		t.Errorf("reply = %q, want the incomplete marker", rb.LastSent().Text())
	}
}

func TestMyPack_NoPack(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()

	if err := s.handleMyPack(context.Background(), rb.Bot, testutil.NewPrivateMessage(testUser, "/mypack")); err != nil {
		t.Fatalf("handleMyPack: %v", err)
	}
	if !strings.Contains(rb.LastSent().Text(), "/newpack") {
		t.Errorf("reply = %q, want it to point at /newpack", rb.LastSent().Text())
	}
}

// The link cannot follow a rename, so the reply has to name the only route to a
// different one — otherwise the user is left at a dead end.
func TestRenamePack_NamesTheURLChangeRoute(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	seedPack(t, s, 4)

	if err := s.handleRenamePack(context.Background(), rb.Bot, testutil.NewPrivateMessage(testUser, "/renamepack Better Name")); err != nil {
		t.Fatalf("handleRenamePack: %v", err)
	}
	if countMethod(rb, "setStickerSetTitle") != 1 {
		t.Fatalf("methods = %v, want one setStickerSetTitle", methodsSent(rb))
	}
	text := rb.LastSent().Text()
	for _, want := range []string{"Better Name", shareLink(testSet), "/delpack", "/newpack"} {
		if !strings.Contains(text, want) {
			t.Errorf("reply %q missing %q", text, want)
		}
	}
	pack, _ := loadPack(t, s)
	if pack.Title != "Better Name" {
		t.Errorf("Title = %q, want the new title committed", pack.Title)
	}
}

func TestRenamePack_NoPack(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()

	if err := s.handleRenamePack(context.Background(), rb.Bot, testutil.NewPrivateMessage(testUser, "/renamepack Whatever")); err != nil {
		t.Fatalf("handleRenamePack: %v", err)
	}
	if countMethod(rb, "setStickerSetTitle") != 0 {
		t.Errorf("methods = %v, want no API call", methodsSent(rb))
	}
}

// Anonymous senders are refused before any store or API access, on every
// command. Telegram gives every anonymous admin the same From.ID, so without
// this they would all share one pack — and under one-pack-per-user, the first
// one to run /newpack would own it and block the rest.
func TestHandlers_RefuseAnonymousSenders(t *testing.T) {
	handlers := map[string]struct {
		text string
		run  func(*state, context.Context, *bot.Bot, *models.Update) error
	}{
		"newpack":      {"/newpack mypack My Pack", (*state).handleNewPack},
		"mypack":       {"/mypack", (*state).handleMyPack},
		"addsticker":   {"/addsticker 😂", (*state).handleAddSticker},
		"delsticker":   {"/delsticker", (*state).handleDelSticker},
		"editsticker":  {"/editsticker 😂", (*state).handleEditSticker},
		"ordersticker": {"/ordersticker 0", (*state).handleOrderSticker},
		"renamepack":   {"/renamepack Title", (*state).handleRenamePack},
		"delpack":      {"/delpack", (*state).handleDelPack},
		"setpackicon":  {"/setpackicon", (*state).handleSetPackIcon},
	}
	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			rb := testutil.NewRecordingBot(t)
			stubBotIdentity(rb)
			setMissing(rb)
			s := newTestState()
			seedPack(t, s, 3)

			upd := stickerReply(h.text, testSet)
			upd.Message.SenderChat = &models.Chat{ID: -100}

			if err := h.run(s, context.Background(), rb.Bot, upd); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			for _, call := range rb.Sent() {
				if call.Method != "sendMessage" {
					t.Errorf("%s called %q for an anonymous sender", name, call.Method)
				}
			}
			if !strings.Contains(rb.LastSent().Text(), "personal account") {
				t.Errorf("%s reply = %q, want the anonymous-sender refusal", name, rb.LastSent().Text())
			}
		})
	}
}

// seedInterrupted recreates the state a crashed /newpack leaves behind: a
// pending pack record AND the global name reservation that always precedes it.
// Seeding the record alone would build a state production cannot reach.
func seedInterrupted(t *testing.T, s *state, slug, setName string) {
	t.Helper()
	ctx := context.Background()
	pending := Pack{Slug: slug, Name: setName, Title: "Old", OwnerID: testUser, Pending: true}
	if err := s.store.Put(ctx, packKey(testUser), pending); err != nil {
		t.Fatalf("seed pending: %v", err)
	}
	if err := s.slugs.Put(ctx, slugKey(slug),
		SlugReservation{Slug: slug, OwnerID: testUser, CreatedAt: fixedNow.UnixMilli()}); err != nil {
		t.Fatalf("seed reservation: %v", err)
	}
}

// The pack-takeover regression. Share links are public, so any user can read a
// pack's slug off t.me and try to claim it. Before the global reservation, an
// attacker with no pack of their own reached createOrAdopt, found the victim's
// set existing, and adopted it — after which /delpack destroyed the victim's
// pack.
//
// The attacker must be refused, and must leave no trace: no adoption, no record
// of their own, and the victim's reservation untouched.
func TestNewPack_CannotSeizeAnotherUsersPack(t *testing.T) {
	const victim, attacker = int64(1), int64(2)

	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setExists(rb) // the victim's set resolves
	s := newTestState()

	ctx := context.Background()
	if err := s.store.Put(ctx, packKey(victim),
		Pack{Slug: "victimpack", Name: "victimpack_by_testbot", Title: "Victim", OwnerID: victim, Count: 9}); err != nil {
		t.Fatalf("seed victim pack: %v", err)
	}
	if err := s.slugs.Put(ctx, slugKey("victimpack"),
		SlugReservation{Slug: "victimpack", OwnerID: victim, CreatedAt: fixedNow.UnixMilli()}); err != nil {
		t.Fatalf("seed victim reservation: %v", err)
	}

	upd := stickerReply("/newpack victimpack Mine Now", otherSet)
	upd.Message.From.ID = attacker
	if err := s.handleNewPack(ctx, rb.Bot, upd); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}

	if !strings.Contains(rb.LastSent().Text(), "taken") {
		t.Errorf("reply = %q, want the name refused as taken", rb.LastSent().Text())
	}
	if got, found, _ := getPack(ctx, s.store, attacker); found {
		t.Errorf("attacker now holds a pack record %+v — takeover succeeded", got)
	}
	held, _, _ := getSlugReservation(ctx, s.slugs, "victimpack")
	if held.OwnerID != victim {
		t.Errorf("reservation owner = %d, want the victim (%d)", held.OwnerID, victim)
	}
	// The victim's own record must be exactly as it was.
	pack, found, _ := getPack(ctx, s.store, victim)
	if !found || pack.Count != 9 || pack.Title != "Victim" {
		t.Errorf("victim pack = (%+v, found=%v), want it untouched", pack, found)
	}
}

// The same protection has to hold for the resume path: a pending record whose
// slug is reserved by somebody else must not adopt either.
func TestNewPack_StaleIntentCannotAdoptForeignName(t *testing.T) {
	const victim, attacker = int64(1), int64(2)

	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setExists(rb)
	s := newTestState()
	ctx := context.Background()

	// The attacker holds a pending record naming the victim's set, but the
	// reservation belongs to the victim.
	if err := s.store.Put(ctx, packKey(attacker),
		Pack{Slug: "victimpack", Name: "victimpack_by_testbot", Title: "Old", OwnerID: attacker, Pending: true}); err != nil {
		t.Fatalf("seed attacker intent: %v", err)
	}
	if err := s.slugs.Put(ctx, slugKey("victimpack"),
		SlugReservation{Slug: "victimpack", OwnerID: victim, CreatedAt: fixedNow.UnixMilli()}); err != nil {
		t.Fatalf("seed victim reservation: %v", err)
	}

	upd := stickerReply("/newpack otherslug Other", otherSet)
	upd.Message.From.ID = attacker
	if err := s.handleNewPack(ctx, rb.Bot, upd); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}

	pack, _, _ := getPack(ctx, s.store, attacker)
	if pack.Name == "victimpack_by_testbot" && !pack.Pending {
		t.Errorf("attacker adopted the victim's set via the stale-intent path: %+v", pack)
	}
	held, _, _ := getSlugReservation(ctx, s.slugs, "victimpack")
	if held.OwnerID != victim {
		t.Errorf("reservation owner = %d, want the victim (%d)", held.OwnerID, victim)
	}
}

// F5's replacement: a name held by another user is now refused by the
// reservation, before any API call. The previous test of this name stubbed a
// combination (set missing + PACK_SHORT_NAME_OCCUPIED) that cannot occur for a
// set another user holds, so it never covered this case.
func TestNewPack_ForeignReservationRefusedBeforeAnyAPICall(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	s := newTestState()
	ctx := context.Background()

	if err := s.slugs.Put(ctx, slugKey("mypack"),
		SlugReservation{Slug: "mypack", OwnerID: 999, CreatedAt: fixedNow.UnixMilli()}); err != nil {
		t.Fatalf("seed reservation: %v", err)
	}

	if err := s.handleNewPack(ctx, rb.Bot, stickerReply("/newpack mypack Mine", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}
	for _, call := range rb.Sent() {
		if call.Method != "sendMessage" && call.Method != "getMe" {
			t.Errorf("called %q for a name held by another user; want refusal before any sticker API call", call.Method)
		}
	}
	if !strings.Contains(rb.LastSent().Text(), "taken") {
		t.Errorf("reply = %q, want the taken refusal", rb.LastSent().Text())
	}
}

// The name-burning regression. Reservations are permanent and global, so
// writing one before establishing the caller is even entitled to a pack turned
// every refused /newpack into a free, unlimited denial primitive: no API call,
// no cost, and the name is gone for everyone else forever.
func TestNewPack_RefusedRunsClaimNoNames(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setMissing(rb)
	s := newTestState()
	ctx := context.Background()

	seedPack(t, s, 3) // the caller already has a finished pack

	for _, name := range []string{"memes", "funny", "cats"} {
		if err := s.handleNewPack(ctx, rb.Bot, stickerReply("/newpack "+name+" x", otherSet)); err != nil {
			t.Fatalf("handleNewPack(%s): %v", name, err)
		}
		if !strings.Contains(rb.LastSent().Text(), "already have a pack") {
			t.Fatalf("reply = %q, want the already-have-a-pack refusal", rb.LastSent().Text())
		}
		if _, held, _ := getSlugReservation(ctx, s.slugs, name); held {
			t.Errorf("refused /newpack claimed %q — every other user is now permanently denied that name", name)
		}
	}

	// Only the real pack's own name is reserved.
	keys, err := s.slugs.List(ctx, slugPrefix)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("reservations = %d (%v), want exactly the one backing the real pack", len(keys), keys)
	}
}

// A name is also not burned when the *set name* is unusable, or when anything
// else makes the command bail after reserving.
func TestNewPack_ReservationReleasedWhenClaimFails(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setMissing(rb)
	rb.FailMethodCode("createNewStickerSet", 400, "Bad Request: PACK_SHORT_NAME_INVALID")
	s := newTestState()
	ctx := context.Background()

	if err := s.handleNewPack(ctx, rb.Bot, stickerReply("/newpack mypack My Pack", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}
	if _, held, _ := getSlugReservation(ctx, s.slugs, "mypack"); held {
		t.Error("reservation survived a refusal that proves nothing was created")
	}
	if _, found := loadPack(t, s); found {
		t.Error("intent survived a refusal that proves nothing was created")
	}
}

// A reservation the caller merely resumed must not be released when a later
// step bails — it predates this command.
func TestNewPack_ResumedReservationSurvivesABail(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	rb.FailMethod("getStickerSet", 500, `{"ok":false,"description":"upstream is unhappy"}`)
	s := newTestState()
	ctx := context.Background()

	seedInterrupted(t, s, "mypack", testSet)

	if err := s.handleNewPack(ctx, rb.Bot, stickerReply("/newpack mypack My Pack", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}
	if _, held, _ := getSlugReservation(ctx, s.slugs, "mypack"); !held {
		t.Error("a resumed reservation was released on an unknown error; the name is now claimable by others while the set may exist")
	}
}

// The two tests below are the only coverage of the `created` flag itself.
//
// Both drive a bail *inside* claimSlug, which is the single place handleNewPack
// consults `created`. The other reservation tests bail later — in createOrAdopt
// — where a different mechanism (createRefused) does the releasing, so they
// pass with the `created` guard removed entirely and cannot pin it.
//
// Reaching claimSlug's bail takes a pending record under a *different* slug:
// that sends claimSlug into resolveStaleIntent, which gives up when it cannot
// establish what happened to the old set.
func seedStaleIntentBail(t *testing.T, rb *testutil.RecordingBot, s *state) {
	t.Helper()
	stubBotIdentity(rb)
	// Unknown failure probing the *old* set: resolveStaleIntent refuses to
	// guess, so handleNewPack bails holding whatever reserveSlug just did.
	rb.FailMethod("getStickerSet", 500, `{"ok":false,"description":"upstream is unhappy"}`)
	seedInterrupted(t, s, "oldname", otherSet)
}

// Direction 1: a name this invocation reserved must not survive the bail.
// Without the release, every refused attempt burns a global name for everyone.
func TestNewPack_FreshReservationReleasedWhenClaimBails(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	ctx := context.Background()
	seedStaleIntentBail(t, rb, s)

	if err := s.handleNewPack(ctx, rb.Bot, stickerReply("/newpack newname New Pack", testSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}

	if _, held, _ := getSlugReservation(ctx, s.slugs, "newname"); held {
		t.Error("a name reserved by this invocation survived its bail — the name is now permanently denied to every other user, with no pack behind it")
	}
	if _, held, _ := getSlugReservation(ctx, s.slugs, "oldname"); !held {
		t.Error("the pre-existing reservation was collateral damage")
	}
}

// Direction 2: a name the caller already held must survive the bail. Releasing
// it would hand a live claim to the next user to ask while the set may exist.
func TestNewPack_ResumedReservationNotReleasedWhenClaimBails(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	s := newTestState()
	ctx := context.Background()
	seedStaleIntentBail(t, rb, s)

	// The caller already holds "newname" from an earlier run, so reserveSlug
	// resumes it rather than creating it.
	if err := s.slugs.Put(ctx, slugKey("newname"),
		SlugReservation{Slug: "newname", OwnerID: testUser, CreatedAt: fixedNow.UnixMilli()}); err != nil {
		t.Fatalf("seed prior reservation: %v", err)
	}

	if err := s.handleNewPack(ctx, rb.Bot, stickerReply("/newpack newname New Pack", testSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}

	if _, held, _ := getSlugReservation(ctx, s.slugs, "newname"); !held {
		t.Error("a reservation that predates this command was released on its bail — another user can now claim a name whose set may already exist")
	}
}

// A reservation is only proof of ownership while it outlives the sets it
// guards, and it does not: reservations live in our store, packs live at
// Telegram, and a restart on the in-memory backend wipes the former while every
// pack survives. This is the takeover of TestNewPack_CannotSeizeAnotherUsersPack
// replayed against an empty store, which is exactly what the attacker gets for
// free after any wipe.
//
// The set existing under a name this invocation has only just claimed proves
// the set is somebody else's: a real interrupted attempt reserved the name
// before creating the set, so it always finds its own reservation waiting.
func TestNewPack_WipedStoreCannotAdoptSurvivingPack(t *testing.T) {
	rb := testutil.NewRecordingBot(t)
	stubBotIdentity(rb)
	setExists(rb) // the victim's pack outlived our store
	s := newTestState()
	ctx := context.Background()

	// Deliberately empty: no reservations, no pack records, nothing.
	if err := s.handleNewPack(ctx, rb.Bot, stickerReply("/newpack mypack My Pack", otherSet)); err != nil {
		t.Fatalf("handleNewPack: %v", err)
	}

	if got := rb.LastSent().Text(); !strings.Contains(got, slugTaken) {
		t.Errorf("reply = %q, want the name-taken refusal", got)
	}
	if pack, found := loadPack(t, s); found && !pack.Pending {
		t.Errorf("adopted a pack that survived the wipe: %+v — /delpack would now destroy its real owner's set", pack)
	}
	for _, call := range rb.Sent() {
		if call.Method == "createNewStickerSet" || call.Method == "setStickerSetTitle" {
			t.Errorf("refused adoption still called %s", call.Method)
		}
	}
	// The refusal must not burn the name either: the set's real owner has to be
	// able to re-register it after the same wipe.
	if _, held, _ := getSlugReservation(ctx, s.slugs, "mypack"); held {
		t.Error("the refused attempt kept the reservation, denying the name to the set's actual owner")
	}
}
