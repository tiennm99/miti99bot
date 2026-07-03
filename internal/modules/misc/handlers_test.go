package misc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

// installMisc wires the misc module to a recording bot with a fresh
// in-memory store. Returns the bot and the typed store (so tests can pre-seed
// or read), plus an Auth that permits Owner + Admin so /ping_stats /the_answer dispatch.
func installMisc(t *testing.T, ownerID int64) (*testutil.RecordingBot, storage.DocStore[lastPing]) {
	t.Helper()
	rb := testutil.NewRecordingBot(t)
	provider := storage.NewMemoryProvider()
	coll := provider.Collection("misc")
	mod := New(modules.Deps{Store: coll})
	store := storage.Typed[lastPing](coll)

	reg := &modules.Registry{
		Modules:     []modules.Module{{Name: "misc", Commands: mod.Commands}},
		AllCommands: map[string]modules.Command{},
	}
	for _, c := range mod.Commands {
		reg.AllCommands[c.Name] = c
	}
	auth := modules.Auth{BotOwnerID: ownerID}
	modules.Install(rb.Bot, reg, auth)
	return rb, store
}

func TestPing_RepliesPongAndWritesStore(t *testing.T) {
	rb, store := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(999, "/ping"))

	if got := rb.LastSent().Text(); got != "pong" {
		t.Errorf("ping reply = %q, want %q", got, "pong")
	}
	stored, _, err := store.Get(context.Background(), lastPingKey)
	if err != nil {
		t.Fatalf("expected lastPing in store: %v", err)
	}
	if stored.At <= 0 {
		t.Errorf("lastPing.At = %d, want positive", stored.At)
	}
	// Sanity: timestamp is within a minute of now (rules out stale fixture).
	if delta := time.Now().UTC().UnixMilli() - stored.At; delta > 60_000 || delta < 0 {
		t.Errorf("lastPing.At delta from now = %dms, want within 60s", delta)
	}
}

func TestPingStats_NeverWhenStoreEmpty(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(999, "/ping_stats"))

	if got := rb.LastSent().Text(); got != "last ping: never" {
		t.Errorf("ping_stats reply = %q, want 'last ping: never'", got)
	}
}

func TestPingStats_AfterPing(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(999, "/ping"))
	rb.Reset()
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(999, "/ping_stats"))

	got := rb.LastSent().Text()
	if !strings.HasPrefix(got, "last ping: ") {
		t.Errorf("ping_stats reply = %q, want 'last ping: ...'", got)
	}
	if strings.Contains(got, "never") {
		t.Errorf("ping_stats still says 'never' after /ping: %q", got)
	}
}

func TestPingStats_DeniedToNonAdmin(t *testing.T) {
	rb, _ := installMisc(t, 999) // owner = 999
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/ping_stats"))

	if calls := rb.Sent(); len(calls) != 0 {
		t.Errorf("non-admin /ping_stats produced replies: %+v", calls)
	}
}

func TestWheelOfNames_UsageWhenMissingOptions(t *testing.T) {
	for _, text := range []string{"/wheelofnames", "/wheelofnames , ,"} {
		t.Run(text, func(t *testing.T) {
			rb, _ := installMisc(t, 999)
			rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, text))

			if got := rb.LastSent().Text(); got != wheelOfNamesUsage {
				t.Errorf("wheelofnames reply = %q, want usage %q", got, wheelOfNamesUsage)
			}
		})
	}
}

func TestWheelOfNames_SingleOption(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice"))

	if got := rb.LastSent().Text(); got != "Alice" {
		t.Errorf("wheelofnames reply = %q, want Alice", got)
	}
}

func TestWheelOfNames_PicksFromTrimmedOptions(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice, Bob, Carol"))

	got := rb.LastSent().Text()
	if got != "Alice" && got != "Bob" && got != "Carol" {
		t.Errorf("wheelofnames reply = %q, want one of Alice/Bob/Carol", got)
	}
}

func TestWheelOfNames_IgnoresEmptySegments(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames , Alice , , Bob ,"))

	got := rb.LastSent().Text()
	if got != "Alice" && got != "Bob" {
		t.Errorf("wheelofnames reply = %q, want Alice or Bob", got)
	}
}

// trongTruongHopUpdate is the inline counterpart of testutil.NewPrivateMessage
// for cases that need control over From (username, names). The dispatcher
// requires a bot_command entity, so we lift that from the helper API by reusing
// NewPrivateMessage and overwriting From.
func trongTruongHopUpdate(t *testing.T, text string, from *models.User) *models.Update {
	t.Helper()
	u := testutil.NewPrivateMessage(from.ID, text)
	u.Message.From = from
	return u
}

func TestTrongTruongHop_DefaultArgUsesVNG(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/trongtruonghop",
		&models.User{ID: 7, Username: "boss", FirstName: "Boss"}))

	got := rb.LastSent().Text()
	if !strings.Contains(got, "VNG") {
		t.Errorf("reply missing default target VNG: %q", got)
	}
	if n := strings.Count(got, "@boss"); n != 2 {
		t.Errorf("reply mentions @boss %d times, want 2: %q", n, got)
	}
}

func TestTrongTruongHop_CustomArg(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/trongtruonghop Acme Corp",
		&models.User{ID: 7, Username: "boss", FirstName: "Boss"}))

	got := rb.LastSent().Text()
	if !strings.Contains(got, "Acme Corp") {
		t.Errorf("reply missing custom arg Acme Corp: %q", got)
	}
	if strings.Contains(got, "VNG") {
		t.Errorf("reply unexpectedly contains default VNG: %q", got)
	}
}

func TestTTHAlias_CustomArg(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/tth Acme Corp",
		&models.User{ID: 7, Username: "boss", FirstName: "Boss"}))

	got := rb.LastSent().Text()
	if !strings.Contains(got, "Acme Corp") {
		t.Errorf("reply missing custom arg Acme Corp: %q", got)
	}
	if n := strings.Count(got, "@boss"); n != 2 {
		t.Errorf("reply mentions @boss %d times, want 2: %q", n, got)
	}
}

func TestTrongTruongHop_HTMLEscapesArg(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/trongtruonghop <script>",
		&models.User{ID: 7, Username: "boss", FirstName: "Boss"}))

	got := rb.LastSent().Text()
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("reply did not HTML-escape arg: %q", got)
	}
	if strings.Contains(got, "<script>") {
		t.Errorf("reply leaked raw <script>: %q", got)
	}
}

func TestTrongTruongHop_NoUsernameFallsBackToLink(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/trongtruonghop",
		&models.User{ID: 42, FirstName: "Anh"})) // no Username

	got := rb.LastSent().Text()
	wantLink := `<a href="tg://user?id=42">Anh</a>`
	if n := strings.Count(got, wantLink); n != 2 {
		t.Errorf("reply contains link %q %d times, want 2: %q", wantLink, n, got)
	}
}

func TestTrongTruongHop_EmptyDisplayNameFallsBackToThanhVien(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/trongtruonghop",
		&models.User{ID: 42})) // no Username, no FirstName/LastName

	got := rb.LastSent().Text()
	wantLink := `<a href="tg://user?id=42">thành viên</a>`
	if n := strings.Count(got, wantLink); n != 2 {
		t.Errorf("reply contains fallback link %d times, want 2: %q", n, got)
	}
}

func TestTheAnswer_OwnerOnly(t *testing.T) {
	rb, _ := installMisc(t, 999)

	// Non-owner: silent denial
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/the_answer"))
	if calls := rb.Sent(); len(calls) != 0 {
		t.Errorf("non-owner /the_answer replied: %+v", calls)
	}

	// Owner: reply
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(999, "/the_answer"))
	if got := rb.LastSent().Text(); got != "The answer." {
		t.Errorf("owner /the_answer reply = %q, want 'The answer.'", got)
	}
}
