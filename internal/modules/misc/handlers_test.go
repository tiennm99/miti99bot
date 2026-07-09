package misc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"math"
	"math/rand/v2"
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

			if got := rb.LastSent().Text(); got != wheelOfNamesUsage {
				t.Errorf("wheelofnames reply = %q, want usage %q", got, wheelOfNamesUsage)
			}
		})
	}
}

func TestWheelOfNames_RenderGIFTiming(t *testing.T) {
	data, err := renderWheelOfNamesGIF([]string{"Alice", "Bob"}, 0)
	if err != nil {
		t.Fatalf("renderWheelOfNamesGIF: %v", err)
	}
	decoded, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if len(decoded.Image) != wheelSpinFrames+wheelHoldFrames {
		t.Fatalf("frames = %d, want %d", len(decoded.Image), wheelSpinFrames+wheelHoldFrames)
	}
	totalDelay := 0
	spinDelay := 0
	holdDelay := 0
	for i, delay := range decoded.Delay {
		totalDelay += delay
		if i < wheelSpinFrames && delay != wheelSpinDelay {
			t.Fatalf("spin delay[%d] = %d, want %d", i, delay, wheelSpinDelay)
		}
		if i < wheelSpinFrames {
			spinDelay += delay
			continue
		}
		if delay != wheelHoldDelay {
			t.Fatalf("hold delay[%d] = %d, want %d", i, delay, wheelHoldDelay)
		}
		holdDelay += delay
	}
	if spinDelay != wheelSpinDuration*100 {
		t.Fatalf("spin delay total = %dcs, want %dcs", spinDelay, wheelSpinDuration*100)
	}
	if holdDelay != wheelHoldDuration*100 {
		t.Fatalf("hold delay total = %dcs, want %dcs", holdDelay, wheelHoldDuration*100)
	}
	if totalDelay != wheelDuration*100 {
		t.Fatalf("total delay = %dcs, want %dcs", totalDelay, wheelDuration*100)
	}
	if equalPalettedFrames(decoded.Image[wheelSpinFrames-1], decoded.Image[wheelSpinFrames]) {
		t.Fatalf("first result frame matches last spin frame, want visible RESULT transition")
	}
	if equalPalettedFrames(decoded.Image[wheelSpinFrames], decoded.Image[wheelSpinFrames+1]) {
		t.Fatalf("first celebration frame matches second celebration frame, want visible result burst")
	}
	firstStableHoldFrame := wheelSpinFrames + wheelCelebrateFrames
	for i := firstStableHoldFrame + 1; i < len(decoded.Image); i++ {
		if !equalPalettedFrames(decoded.Image[firstStableHoldFrame], decoded.Image[i]) {
			t.Fatalf("stable result hold frame %d differs from frame %d", i, firstStableHoldFrame)
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

func TestWheelOfNames_CurrentOptionTracksPointer(t *testing.T) {
	for winner := range []string{"Alice", "Bob", "Carol", "Dana"} {
		rotation := finalWheelRotation(4, winner)
		if got := currentWheelIndex(4, rotation); got != winner {
			t.Fatalf("currentWheelIndex at final rotation = %d, want %d", got, winner)
		}
	}
}

func TestWheelOfNames_RandomSpinProfileKeepsWinnerUnderPointer(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for optionCount := 2; optionCount <= 10; optionCount++ {
		for winner := 0; winner < optionCount; winner++ {
			for spin := 0; spin < 20; spin++ {
				profile := newWheelSpinProfile(optionCount, winner, rng)
				if got := currentWheelIndex(optionCount, profile.finalRotation); got != winner {
					t.Fatalf("optionCount=%d winner=%d spin=%d final index = %d", optionCount, winner, spin, got)
				}
				if got := profile.rotationAt(1); math.Abs(got-profile.finalRotation) > 1e-9 {
					t.Fatalf("rotationAt(1) = %f, want final %f", got, profile.finalRotation)
				}
			}
		}
	}
}

func TestWheelOfNames_SpinProfileVariesBetweenSpins(t *testing.T) {
	rng := rand.New(rand.NewPCG(10, 20))
	first := newWheelSpinProfile(5, 2, rng)
	second := newWheelSpinProfile(5, 2, rng)
	if first.startRotation == second.startRotation &&
		first.finalRotation == second.finalRotation &&
		first.accelEnd == second.accelEnd &&
		first.decelSharpness == second.decelSharpness &&
		first.wobblePhase == second.wobblePhase {
		t.Fatalf("spin profiles did not vary: %+v", first)
	}
}

func TestWheelOfNames_SpinProfileProgressIsMonotonic(t *testing.T) {
	rng := rand.New(rand.NewPCG(30, 40))
	profile := newWheelSpinProfile(6, 4, rng)
	prev := -1.0
	for i := 0; i <= 100; i++ {
		progress := profile.progressAt(float64(i) / 100)
		if progress < prev {
			t.Fatalf("progress at step %d = %f, previous %f", i, progress, prev)
		}
		prev = progress
	}
}

func TestWheelOfNames_PointerPointsIntoWheel(t *testing.T) {
	img := renderWheelFrame([]string{"Alice", "Bob"}, 0, finalWheelRotation(2, 0), false)
	cx := wheelSize / 2
	cy := wheelSize / 2
	tipX := cx + wheelRadius
	if got := img.ColorIndexAt(tipX, cy); got != 1 {
		t.Fatalf("pointer tip color = %d, want 1", got)
	}
	if got := img.ColorIndexAt(tipX+10, cy+5); got != 1 {
		t.Fatalf("pointer shoulder color = %d, want 1", got)
	}
	if got := img.ColorIndexAt(tipX+12, cy); got != wheelSliceColorIndexes[0] {
		t.Fatalf("pointer body color = %d, want current slice color %d", got, wheelSliceColorIndexes[0])
	}
	if got := img.ColorIndexAt(tipX, cy+5); got == 1 {
		t.Fatalf("pointer tip is too tall at color index %d", got)
	}
	if got := img.ColorIndexAt(tipX-20, cy+12); got == 1 {
		t.Fatalf("pointer widens inside wheel edge at color index %d", got)
	}
}

func TestWheelOfNames_FinalSliceRendersAtRightPointer(t *testing.T) {
	options := []string{"Alice", "Bob", "Carol", "Dana"}
	winner := 2
	img := renderWheelFrame(options, winner, finalWheelRotation(len(options), winner), true)
	x := wheelSize/2 + wheelRadius - 20
	y := wheelSize / 2
	want := wheelSliceColorIndexes[winner%len(wheelSliceColorIndexes)]
	if got := img.ColorIndexAt(x, y); got != want {
		t.Fatalf("right pointer slice color = %d, want winner slice color %d", got, want)
	}
}

func TestWheelOfNames_DrawsOptionLabelsInsideSlices(t *testing.T) {
	options := []string{"Student", "Teacher", "Parent", "Staff"}
	rotation := 0.0
	img := renderWheelFrame(options, 0, rotation, false)
	segment := 2 * math.Pi / float64(len(options))
	angle := rotation + segment/2
	centerX := wheelSize/2 + int(math.Round(math.Cos(angle)*float64(wheelSliceLabelRadius)))
	centerY := wheelSize/2 + int(math.Round(math.Sin(angle)*float64(wheelSliceLabelRadius)))
	bounds := image.Rect(centerX-28, centerY-28, centerX+28, centerY+28)
	if got := countColorIndex(img, bounds, 1); got == 0 {
		t.Fatalf("slice label dark pixels = %d, want > 0", got)
	}
}

func TestWheelOfNames_RotatesOptionLabelsWithSlices(t *testing.T) {
	options := []string{"Rotate", "Teacher", "Parent", "Staff"}
	rotation := -3 * math.Pi / 4
	img := renderWheelFrame(options, 0, rotation, false)

	segment := 2 * math.Pi / float64(len(options))
	angle := rotation + segment/2
	centerX := wheelSize/2 + int(math.Round(math.Cos(angle)*float64(wheelSliceLabelRadius)))
	centerY := wheelSize/2 + int(math.Round(math.Sin(angle)*float64(wheelSliceLabelRadius)))
	searchBounds := image.Rect(centerX-28, centerY-32, centerX+28, centerY+32)
	labelBounds, ok := colorIndexBounds(img, searchBounds, wheelInkColorIndex)
	if !ok {
		t.Fatalf("slice label dark pixels missing in %v", searchBounds)
	}
	if labelBounds.Dy() <= labelBounds.Dx() {
		t.Fatalf("slice label bounds = %v, want taller than wide for rotated label", labelBounds)
	}
}

func TestWheelOfNames_DisplayTextNormalizesVietnamese(t *testing.T) {
	input := "không dấu Tiếng Việt Đặng Ơ Ư ấ ệ"
	want := "khong dau Tieng Viet Dang O U a e"
	got := wheelDisplayText(input, 64)
	if got != want {
		t.Fatalf("wheelDisplayText() = %q, want %q", got, want)
	}
	if strings.Contains(got, "?") {
		t.Fatalf("wheelDisplayText() replaced Vietnamese with ?: %q", got)
	}
}

func TestWheelOfNames_DisplayTextNormalizesDecomposedVietnamese(t *testing.T) {
	got := wheelDisplayText("tie\u0302\u0301ng Vie\u0323t", 32)
	if got != "tieng Viet" {
		t.Fatalf("wheelDisplayText() = %q, want %q", got, "tieng Viet")
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

func colorIndexBounds(img *image.Paletted, bounds image.Rectangle, colorIndex byte) (image.Rectangle, bool) {
	bounds = bounds.Intersect(img.Bounds())
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.ColorIndexAt(x, y) != colorIndex {
				continue
			}
			found = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	if !found {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX, maxY), true
}

func TestWheelOfNames_SendsAnimationWithSpoilerCaption(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/wheelofnames Alice"))

	call := rb.LastSent()
	if call.Method != "sendAnimation" {
		t.Fatalf("method = %q, want sendAnimation", call.Method)
	}
	if got := call.Form["caption"]; got != `Result: <span class="tg-spoiler">Alice</span>` {
		t.Fatalf("caption = %q, want result spoiler", got)
	}
	if got := call.Form["parse_mode"]; got != "HTML" {
		t.Fatalf("parse_mode = %q, want HTML", got)
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

func TestWheelOfNames_ResultCaptionEscapesHTML(t *testing.T) {
	got := wheelResultCaption(`<Alice & Bob>`)
	want := `Result: <span class="tg-spoiler">&lt;Alice &amp; Bob&gt;</span>`
	if got != want {
		t.Fatalf("wheelResultCaption() = %q, want %q", got, want)
	}
}

func TestWheelOfNames_ResultCaptionTruncatesLongResult(t *testing.T) {
	got := wheelResultCaption(strings.Repeat("a", wheelResultCaptionMaxRunes+1))
	want := `Result: <span class="tg-spoiler">` + strings.Repeat("a", wheelResultCaptionMaxRunes) + `...</span>`
	if got != want {
		t.Fatalf("wheelResultCaption() length = %d, want truncated caption length %d", len(got), len(want))
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
	wantCaption := wheelResultCaption(got.Options[got.WinnerIndex])
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

func TestWheelOfNames_RemoteFailureFallsBackToLocalAnimation(t *testing.T) {
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
	if call.Method != "sendAnimation" {
		t.Fatalf("method = %q, want sendAnimation", call.Method)
	}
	if got := call.Form["caption"]; got != `Result: <span class="tg-spoiler">Alice</span>` {
		t.Fatalf("caption = %q, want result spoiler", got)
	}
	if got := call.Form["parse_mode"]; got != "HTML" {
		t.Fatalf("parse_mode = %q, want HTML", got)
	}
	if got := call.Form["duration"]; got != "10" {
		t.Fatalf("duration = %q, want local duration 10", got)
	}
	if got := call.Form["width"]; got != "320" {
		t.Fatalf("width = %q, want local width 320", got)
	}
	if got := call.Form["height"]; got != "320" {
		t.Fatalf("height = %q, want local height 320", got)
	}
}

func TestWheelOfNames_ForwardsMessageThreadID(t *testing.T) {
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
