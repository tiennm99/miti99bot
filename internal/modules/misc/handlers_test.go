package misc

import (
	"bytes"
	"context"
	"image"
	"image/gif"
	"math"
	"math/rand/v2"
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

func withoutWheelDraftDelay(t *testing.T) {
	t.Helper()
	prev := wheelDraftFrameDelay
	wheelDraftFrameDelay = 0
	t.Cleanup(func() { wheelDraftFrameDelay = prev })
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

			if got := rb.LastSent().Text(); got != wheelOfNamesUsage {
				t.Errorf("wheelofnames reply = %q, want usage %q", got, wheelOfNamesUsage)
			}
		})
	}
}

func TestWheelOfNames_SingleOption(t *testing.T) {
	withoutWheelDraftDelay(t)
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice"))

	if got := rb.LastSent().Text(); got != "Alice" {
		t.Errorf("wheelofnames reply = %q, want Alice", got)
	}
}

func TestWheelOfNames_PicksFromTrimmedOptions(t *testing.T) {
	withoutWheelDraftDelay(t)
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice, Bob, Carol"))

	got := rb.LastSent().Text()
	if got != "Alice" && got != "Bob" && got != "Carol" {
		t.Errorf("wheelofnames reply = %q, want one of Alice/Bob/Carol", got)
	}
}

func TestWheelOfNames_IgnoresEmptySegments(t *testing.T) {
	withoutWheelDraftDelay(t)
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames , Alice , , Bob ,"))

	got := rb.LastSent().Text()
	if got != "Alice" && got != "Bob" {
		t.Errorf("wheelofnames reply = %q, want Alice or Bob", got)
	}
}

func TestWheelOfNames_StreamsDraftsBeforeFinalInPrivateChat(t *testing.T) {
	withoutWheelDraftDelay(t)
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice"))

	calls := rb.Sent()
	if len(calls) != 7 {
		t.Fatalf("calls = %d, want 6 drafts + final sendMessage: %+v", len(calls), calls)
	}
	draftID := ""
	for i := 0; i < 6; i++ {
		if calls[i].Method != "sendMessageDraft" {
			t.Fatalf("call %d method = %q, want sendMessageDraft", i, calls[i].Method)
		}
		if !strings.Contains(calls[i].Text(), "Alice") {
			t.Errorf("draft %d text = %q, want to preview Alice", i, calls[i].Text())
		}
		if got := calls[i].Form["draft_id"]; got == "" || got == "0" {
			t.Fatalf("draft %d id = %q, want non-zero", i, got)
		} else if draftID == "" {
			draftID = got
		} else if got != draftID {
			t.Fatalf("draft %d id = %q, want same draft id %q", i, got, draftID)
		}
	}
	if calls[6].Method != "sendMessage" || calls[6].Text() != "Alice" {
		t.Fatalf("final call = %+v, want sendMessage Alice", calls[6])
	}
}

func TestWheelOfNames_GroupSkipsDraftsButSendsFinal(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewGroupMessage(-100, 7, "/wheelofnames Alice"))

	calls := rb.Sent()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want only final sendMessage in groups: %+v", len(calls), calls)
	}
	if calls[0].Method != "sendMessage" || calls[0].Text() != "Alice" {
		t.Fatalf("group call = %+v, want sendMessage Alice", calls[0])
	}
}

func TestWheelOfNamesBeta_UsageWhenMissingOptions(t *testing.T) {
	for _, text := range []string{"/wheelofnamesbeta", "/wheelofnamesbeta , ,"} {
		t.Run(text, func(t *testing.T) {
			rb, _ := installMisc(t, 999)
			rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, text))

			if got := rb.LastSent().Text(); got != wheelOfNamesBetaUsage {
				t.Errorf("wheelofnamesbeta reply = %q, want usage %q", got, wheelOfNamesBetaUsage)
			}
		})
	}
}

func TestWheelOfNamesBeta_RenderGIFTiming(t *testing.T) {
	data, err := renderWheelOfNamesBetaGIF([]string{"Alice", "Bob"}, 0)
	if err != nil {
		t.Fatalf("renderWheelOfNamesBetaGIF: %v", err)
	}
	decoded, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if len(decoded.Image) != wheelBetaSpinFrames+wheelBetaHoldFrames {
		t.Fatalf("frames = %d, want %d", len(decoded.Image), wheelBetaSpinFrames+wheelBetaHoldFrames)
	}
	totalDelay := 0
	spinDelay := 0
	holdDelay := 0
	for i, delay := range decoded.Delay {
		totalDelay += delay
		if i < wheelBetaSpinFrames && delay != wheelBetaSpinDelay {
			t.Fatalf("spin delay[%d] = %d, want %d", i, delay, wheelBetaSpinDelay)
		}
		if i < wheelBetaSpinFrames {
			spinDelay += delay
			continue
		}
		if delay != wheelBetaHoldDelay {
			t.Fatalf("hold delay[%d] = %d, want %d", i, delay, wheelBetaHoldDelay)
		}
		holdDelay += delay
	}
	if spinDelay != wheelBetaSpinDuration*100 {
		t.Fatalf("spin delay total = %dcs, want %dcs", spinDelay, wheelBetaSpinDuration*100)
	}
	if holdDelay != wheelBetaHoldDuration*100 {
		t.Fatalf("hold delay total = %dcs, want %dcs", holdDelay, wheelBetaHoldDuration*100)
	}
	if totalDelay != wheelBetaDuration*100 {
		t.Fatalf("total delay = %dcs, want %dcs", totalDelay, wheelBetaDuration*100)
	}
	if equalPalettedFrames(decoded.Image[wheelBetaSpinFrames-1], decoded.Image[wheelBetaSpinFrames]) {
		t.Fatalf("first result frame matches last spin frame, want visible RESULT transition")
	}
	for i := wheelBetaSpinFrames + 1; i < len(decoded.Image); i++ {
		if !equalPalettedFrames(decoded.Image[wheelBetaSpinFrames], decoded.Image[i]) {
			t.Fatalf("result hold frame %d differs from first result frame", i)
		}
	}
	if decoded.LoopCount != -1 {
		t.Fatalf("loop count = %d, want -1", decoded.LoopCount)
	}
}

func equalPalettedFrames(a, b *image.Paletted) bool {
	if !a.Rect.Eq(b.Rect) || a.Stride != b.Stride {
		return false
	}
	return bytes.Equal(a.Pix, b.Pix)
}

func TestWheelOfNamesBeta_CurrentOptionTracksPointer(t *testing.T) {
	for winner := range []string{"Alice", "Bob", "Carol", "Dana"} {
		rotation := finalWheelRotation(4, winner)
		if got := currentWheelBetaIndex(4, rotation); got != winner {
			t.Fatalf("currentWheelBetaIndex at final rotation = %d, want %d", got, winner)
		}
	}
}

func TestWheelOfNamesBeta_RandomSpinProfileKeepsWinnerUnderPointer(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for optionCount := 2; optionCount <= 10; optionCount++ {
		for winner := 0; winner < optionCount; winner++ {
			for spin := 0; spin < 20; spin++ {
				profile := newWheelBetaSpinProfile(optionCount, winner, rng)
				if got := currentWheelBetaIndex(optionCount, profile.finalRotation); got != winner {
					t.Fatalf("optionCount=%d winner=%d spin=%d final index = %d", optionCount, winner, spin, got)
				}
				if got := profile.rotationAt(1); math.Abs(got-profile.finalRotation) > 1e-9 {
					t.Fatalf("rotationAt(1) = %f, want final %f", got, profile.finalRotation)
				}
			}
		}
	}
}

func TestWheelOfNamesBeta_SpinProfileVariesBetweenSpins(t *testing.T) {
	rng := rand.New(rand.NewPCG(10, 20))
	first := newWheelBetaSpinProfile(5, 2, rng)
	second := newWheelBetaSpinProfile(5, 2, rng)
	if first.startRotation == second.startRotation &&
		first.finalRotation == second.finalRotation &&
		first.accelEnd == second.accelEnd &&
		first.coastEnd == second.coastEnd &&
		first.wobblePhase == second.wobblePhase {
		t.Fatalf("spin profiles did not vary: %+v", first)
	}
}

func TestWheelOfNamesBeta_SpinProfileProgressIsMonotonic(t *testing.T) {
	rng := rand.New(rand.NewPCG(30, 40))
	profile := newWheelBetaSpinProfile(6, 4, rng)
	prev := -1.0
	for i := 0; i <= 100; i++ {
		progress := profile.progressAt(float64(i) / 100)
		if progress < prev {
			t.Fatalf("progress at step %d = %f, previous %f", i, progress, prev)
		}
		prev = progress
	}
}

func TestWheelOfNamesBeta_PointerPointsIntoWheel(t *testing.T) {
	img := renderWheelBetaFrame([]string{"Alice", "Bob"}, 0, finalWheelRotation(2, 0), false)
	cx := wheelBetaSize / 2
	tipY := wheelBetaSize/2 - wheelBetaRadius
	if got := img.ColorIndexAt(cx, tipY); got != 1 {
		t.Fatalf("pointer tip color = %d, want 1", got)
	}
	if got := img.ColorIndexAt(cx+5, tipY-10); got != 1 {
		t.Fatalf("pointer shoulder color = %d, want 1", got)
	}
	if got := img.ColorIndexAt(cx+5, tipY); got == 1 {
		t.Fatalf("pointer tip is too wide at color index %d", got)
	}
	if got := img.ColorIndexAt(cx+12, tipY+20); got == 1 {
		t.Fatalf("pointer widens below wheel edge at color index %d", got)
	}
}

func TestWheelOfNamesBeta_DrawsOptionLabelsInsideSlices(t *testing.T) {
	options := []string{"Student", "Teacher", "Parent", "Staff"}
	rotation := 0.0
	img := renderWheelBetaFrame(options, 0, rotation, false)
	segment := 2 * math.Pi / float64(len(options))
	angle := rotation + segment/2
	centerX := wheelBetaSize/2 + int(math.Round(math.Cos(angle)*float64(wheelBetaSliceLabelRadius)))
	baselineY := wheelBetaSize/2 + int(math.Round(math.Sin(angle)*float64(wheelBetaSliceLabelRadius))) + wheelBetaSliceLabelBaselineOffset
	bounds := image.Rect(centerX-28, baselineY-14, centerX+28, baselineY+2)
	if got := countColorIndex(img, bounds, 1); got == 0 {
		t.Fatalf("slice label dark pixels = %d, want > 0", got)
	}
}

func countColorIndex(img *image.Paletted, bounds image.Rectangle, colorIndex byte) int {
	bounds = bounds.Intersect(img.Bounds())
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.ColorIndexAt(x, y) == colorIndex {
				count++
			}
		}
	}
	return count
}

func TestWheelOfNamesBeta_SendsAnimationWithoutSpoilingCaption(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnamesbeta Alice"))

	call := rb.LastSent()
	if call.Method != "sendAnimation" {
		t.Fatalf("method = %q, want sendAnimation", call.Method)
	}
	if got := call.Form["caption"]; got != "Spinning..." {
		t.Fatalf("caption = %q, want Spinning...", got)
	}
	if strings.Contains(call.Form["caption"], "Alice") {
		t.Fatalf("caption spoils winner: %q", call.Form["caption"])
	}
	if got := call.Form["duration"]; got != "10" {
		t.Fatalf("duration = %q, want 10", got)
	}
	if got := call.Form["width"]; got != "320" {
		t.Fatalf("width = %q, want 320", got)
	}
	if got := call.Form["height"]; got != "320" {
		t.Fatalf("height = %q, want 320", got)
	}
}

func TestWheelOfNamesBeta_ForwardsMessageThreadID(t *testing.T) {
	rb, _ := installMisc(t, 999)
	update := testutil.NewSupergroupMessage(-100, 7, "/wheelofnamesbeta Alice")
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
