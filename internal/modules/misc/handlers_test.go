package misc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
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

func TestRandom_UsageWhenMissingOptions(t *testing.T) {
	for _, text := range []string{"/random", "/random , ,"} {
		t.Run(text, func(t *testing.T) {
			rb, _ := installMisc(t, 999)
			rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, text))

			if got := rb.LastSent().Text(); got != randomUsage {
				t.Errorf("random reply = %q, want usage %q", got, randomUsage)
			}
		})
	}
}

func TestRandom_SingleOption(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/random Alice"))

	if got := rb.LastSent().Text(); got != "Alice" {
		t.Errorf("random reply = %q, want Alice", got)
	}
}

func TestRandom_PicksFromTrimmedOptions(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/random Alice, Bob, Carol"))

	got := rb.LastSent().Text()
	if got != "Alice" && got != "Bob" && got != "Carol" {
		t.Errorf("random reply = %q, want one of Alice/Bob/Carol", got)
	}
}

func TestRandom_IgnoresEmptySegments(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/random , Alice , , Bob ,"))

	got := rb.LastSent().Text()
	if got != "Alice" && got != "Bob" {
		t.Errorf("random reply = %q, want Alice or Bob", got)
	}
}

func TestWheelOfNames_UsageWhenMissingOptions(t *testing.T) {
	for _, text := range []string{"/wheelofnames", "/wheelofnames , ,"} {
		t.Run(text, func(t *testing.T) {
			rb, _ := installMisc(t, 999)
			rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, text))

			if got := rb.LastSent().Text(); got != wheelUsage {
				t.Errorf("wheelofnames reply = %q, want usage %q", got, wheelUsage)
			}
		})
	}
}

func TestWheelOfNames_ResultCaptionEscapesHTML(t *testing.T) {
	got := wheelResultCaption([]string{`<Alice & Bob>`}, 0)
	want := `Result: <span class="tg-spoiler">&lt;Alice &amp; Bob&gt;</span>`
	if got != want {
		t.Fatalf("wheelResultCaption() = %q, want %q", got, want)
	}
}

func TestWheelOfNames_ResultCaptionTruncatesLongResult(t *testing.T) {
	got := wheelResultCaption([]string{strings.Repeat("a", wheelResultCaptionMaxRunes+1)}, 0)
	want := `Result: <span class="tg-spoiler">` + strings.Repeat("a", wheelResultCaptionMaxRunes) + `...</span>`
	if got != want {
		t.Fatalf("wheelResultCaption() length = %d, want truncated caption length %d", len(got), len(want))
	}
}

func TestWheelOfNames_ResultCaptionPadsShortWinnerToLongestOption(t *testing.T) {
	options := []string{"Bob", "Alexandria"}
	got := wheelResultCaption(options, 0)
	want := `Result: <span class="tg-spoiler">` + strings.Repeat(wheelCaptionPad, 3) + "Bob" + strings.Repeat(wheelCaptionPad, 4) + `</span>`
	if got != want {
		t.Fatalf("wheelResultCaption() = %q, want %q", got, want)
	}
	if longest := wheelResultCaption(options, 1); len([]rune(longest)) != len([]rune(got)) {
		t.Fatalf("caption lengths differ: winner %q vs %q", got, longest)
	}
}

func TestWheelOfNames_UsesRemoteAPIWhenConfigured(t *testing.T) {
	var got wheelAPIRequest
	var gotAuthorization string
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/gif" {
			t.Errorf("path = %q, want /api/gif", r.URL.Path)
		}
		gotAuthorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("Decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write([]byte("GIF89a-remote"))
	}))
	defer server.Close()
	t.Setenv(wheelOfNamesAPIURLEnv, server.URL+"/api/gif")
	t.Setenv(wheelOfNamesAPITokenEnv, "remote-token")

	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice, Bob, Carol"))

	if calls != 1 {
		t.Fatalf("remote calls = %d, want 1", calls)
	}
	if gotAuthorization != "Bearer remote-token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuthorization)
	}
	if !slices.Equal(got.Options, []string{"Alice", "Bob", "Carol"}) {
		t.Fatalf("options = %#v, want parsed options", got.Options)
	}
	if got.WinnerIndex < 0 || got.WinnerIndex >= len(got.Options) {
		t.Fatalf("winnerIndex = %d, want in range", got.WinnerIndex)
	}
	assertWheelRemoteDefaults(t, got)

	call := rb.LastSent()
	if call.Method != "sendAnimation" {
		t.Fatalf("method = %q, want sendAnimation", call.Method)
	}
	wantCaption := wheelResultCaption(got.Options, got.WinnerIndex)
	if got := call.Form["caption"]; got != wantCaption {
		t.Fatalf("caption = %q, want %q", got, wantCaption)
	}
	if got := call.Form["parse_mode"]; got != "HTML" {
		t.Fatalf("parse_mode = %q, want HTML", got)
	}
	if got := call.Form["duration"]; got != "7" {
		t.Fatalf("duration = %q, want 7", got)
	}
	if got := call.Form["width"]; got != "512" {
		t.Fatalf("width = %q, want 512", got)
	}
	if got := call.Form["height"]; got != "512" {
		t.Fatalf("height = %q, want 512", got)
	}
}

func TestWheelOfNames_RemoteFailureFallsBackToRandomReply(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv(wheelOfNamesAPIURLEnv, server.URL+"/api/gif")
	t.Setenv(wheelOfNamesAPITokenEnv, "remote-token")

	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice"))

	if calls != 1 {
		t.Fatalf("remote calls = %d, want 1", calls)
	}
	call := rb.LastSent()
	if call.Method != "sendMessage" || call.Text() != "Alice" {
		t.Fatalf("fallback call = %+v, want sendMessage Alice", call)
	}
}

func TestWheelOfNames_NotConfiguredFallsBackToRandomReply(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice"))

	call := rb.LastSent()
	if call.Method != "sendMessage" || call.Text() != "Alice" {
		t.Fatalf("fallback call = %+v, want sendMessage Alice", call)
	}
}

func TestWheelOfNames_SendAnimationFailureFallsBackToRandomReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write([]byte("GIF89a-remote"))
	}))
	defer server.Close()
	t.Setenv(wheelOfNamesAPIURLEnv, server.URL+"/api/gif")

	rb, _ := installMisc(t, 999)
	rb.FailMethod("sendAnimation", http.StatusInternalServerError, "")
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice"))

	calls := rb.Sent()
	if len(calls) != 2 {
		t.Fatalf("calls = %+v, want sendAnimation then sendMessage", calls)
	}
	if calls[0].Method != "sendAnimation" {
		t.Fatalf("first method = %q, want sendAnimation", calls[0].Method)
	}
	if calls[1].Method != "sendMessage" || calls[1].Text() != "Alice" {
		t.Fatalf("fallback call = %+v, want sendMessage Alice", calls[1])
	}
}

func TestWheelOfNames_ForwardsMessageThreadID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write([]byte("GIF89a-remote"))
	}))
	defer server.Close()
	t.Setenv(wheelOfNamesAPIURLEnv, server.URL+"/api/gif")

	rb, _ := installMisc(t, 999)
	update := testutil.NewSupergroupMessage(-100, 7, "/wheelofnames Alice")
	update.Message.MessageThreadID = 42
	rb.Bot.ProcessUpdate(context.Background(), update)

	call := rb.LastSent()
	if call.Method != "sendAnimation" {
		t.Fatalf("method = %q, want sendAnimation", call.Method)
	}
	if got := call.Form["message_thread_id"]; got != "42" {
		t.Fatalf("message_thread_id = %q, want 42", got)
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

func TestTrongTruongHop_DefaultText(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/trongtruonghop",
		&models.User{ID: 7, Username: "boss", FirstName: "Boss"}))

	got := rb.LastSent().Text()
	want := fmt.Sprintf(trongTruongHopTemplate, defaultTarget, "@boss", "@boss")
	if got != want {
		t.Errorf("reply = %q, want %q", got, want)
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
	if strings.Contains(got, defaultTarget) {
		t.Errorf("reply unexpectedly contains default target: %q", got)
	}
	if n := strings.Count(got, "@boss"); n != 2 {
		t.Errorf("reply mentions @boss %d times, want 2: %q", n, got)
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

func TestTrongTruongHopVNG_DefaultText(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/trongtruonghopvng ignored arg",
		&models.User{ID: 7, Username: "boss", FirstName: "Boss"}))

	got := rb.LastSent().Text()
	want := fmt.Sprintf(trongTruongHopTemplate, vngTarget, "@boss", "@boss")
	if got != want {
		t.Errorf("reply = %q, want %q", got, want)
	}
}

func TestTTHVNGAlias_DefaultText(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/tthvng ignored arg",
		&models.User{ID: 7, Username: "boss", FirstName: "Boss"}))

	got := rb.LastSent().Text()
	want := fmt.Sprintf(trongTruongHopTemplate, vngTarget, "@boss", "@boss")
	if got != want {
		t.Errorf("reply = %q, want %q", got, want)
	}
}

func TestTrongTruongHop_NoUsernameFallsBackToDisplayNameMention(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/trongtruonghop",
		&models.User{ID: 42, FirstName: "Anh", LastName: "Le"}))

	got := rb.LastSent().Text()
	wantLink := `<a href="tg://user?id=42">Anh Le</a>`
	if n := strings.Count(got, wantLink); n != 2 {
		t.Errorf("reply contains display-name mention %q %d times, want 2: %q", wantLink, n, got)
	}
}

func TestTrongTruongHopVNG_NoUsernameFallsBackToDisplayNameMention(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), trongTruongHopUpdate(t, "/trongtruonghopvng ignored arg",
		&models.User{ID: 42, FirstName: "Anh", LastName: "Le"}))

	got := rb.LastSent().Text()
	wantLink := `<a href="tg://user?id=42">Anh Le</a>`
	if n := strings.Count(got, wantLink); n != 2 {
		t.Errorf("reply contains display-name mention %q %d times, want 2: %q", wantLink, n, got)
	}
}

func TestFF_DeniedToNonAdmin(t *testing.T) {
	rb, _ := installMisc(t, 999)

	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/ff"))
	if calls := rb.Sent(); len(calls) != 0 {
		t.Errorf("non-admin /ff replied: %+v", calls)
	}
}

func TestFF_RepliesTemplate(t *testing.T) {
	rb, _ := installMisc(t, 999)

	// Args are ignored — the reply is the same canned rant every time.
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(999, "/ff ignored arg"))
	if got := rb.LastSent().Text(); got != ffTemplate {
		t.Errorf("/ff reply = %q, want the ff template", got)
	}
}

// The template signs off with an uppercase /FF, so tapping that link must reach
// the same handler the lowercase form does.
func TestFF_UppercaseFormDispatches(t *testing.T) {
	rb, _ := installMisc(t, 999)

	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(999, "/FF"))
	if got := rb.LastSent().Text(); got != ffTemplate {
		t.Errorf("/FF reply = %q, want the ff template", got)
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
