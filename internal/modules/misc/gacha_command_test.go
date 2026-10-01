package misc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/tiennm99/miti99bot/internal/testutil"
)

var mp4Bytes = []byte("\x00\x00\x00\x18ftypisom-remote")

func TestParseGachaOptions(t *testing.T) {
	got := parseGachaOptions(" Pizza*5, Pho *4 , Rice, Bun * 3, *5, , Cake*7, a*b ")
	want := []gachaOption{
		{Label: "Pizza", Rarity: 5},
		{Label: "Pho", Rarity: 4},
		{Label: "Rice", Rarity: 3},
		{Label: "Bun", Rarity: 3},
		{Label: "Cake*7", Rarity: 3},
		{Label: "a*b", Rarity: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseGachaOptions() = %#v, want %#v", got, want)
	}
}

func TestGacha_EmptyArgsRepliesUsage(t *testing.T) {
	for _, text := range []string{"/gacha", "/gacha , ,", "/gacha *5"} {
		t.Run(text, func(t *testing.T) {
			rb, _ := installMisc(t, 999)
			rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, text))

			if got := rb.LastSent().Text(); got != gachaUsage {
				t.Errorf("gacha reply = %q, want usage %q", got, gachaUsage)
			}
		})
	}
}

func TestGacha_NotConfiguredRepliesWithStars(t *testing.T) {
	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/gacha Pizza*5"))

	calls := rb.Sent()
	if len(calls) != 1 || calls[0].Method != "sendMessage" || calls[0].Text() != "★★★★★ Pizza" {
		t.Fatalf("calls = %+v, want a single ★★★★★ Pizza reply", calls)
	}
}

func TestGacha_UsesRemoteAPIWhenConfigured(t *testing.T) {
	var got gachaAPIRequest
	var gotPath, gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("Decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(mp4Bytes)
	}))
	defer server.Close()
	t.Setenv(wheelOfNamesAPIURLEnv, server.URL+"/api/gif")
	t.Setenv(wheelOfNamesAPITokenEnv, "remote-token")

	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/gacha Pho*4"))

	if gotPath != "/api/gacha" {
		t.Fatalf("path = %q, want /api/gacha", gotPath)
	}
	if gotAuthorization != "Bearer remote-token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuthorization)
	}
	want := gachaAPIRequest{Label: "Pho", Rarity: 4, FPS: gachaRemoteFPS, Width: gachaRemoteWidth}
	if got != want {
		t.Fatalf("request = %+v, want %+v", got, want)
	}

	sent := rb.Sent()
	if len(sent) != 3 {
		t.Fatalf("calls = %+v, want placeholder, sendAnimation, deleteMessage", sent)
	}
	if sent[0].Method != "sendMessage" || sent[0].Text() != gachaPlaceholder {
		t.Fatalf("first call = %+v, want placeholder %q", sent[0], gachaPlaceholder)
	}
	if sent[2].Method != "deleteMessage" || sent[2].Form["message_id"] != "1" {
		t.Fatalf("last call = %+v, want deleteMessage of the placeholder", sent[2])
	}
	call := sent[1]
	if call.Method != "sendAnimation" {
		t.Fatalf("method = %q, want sendAnimation", call.Method)
	}
	if want := `Result: <span class="tg-spoiler">★★★★ Pho</span>`; call.Form["caption"] != want {
		t.Fatalf("caption = %q, want %q", call.Form["caption"], want)
	}
	for field, want := range map[string]string{"parse_mode": "HTML", "duration": "7", "width": "640", "height": "360"} {
		if got := call.Form[field]; got != want {
			t.Fatalf("%s = %q, want %q", field, got, want)
		}
	}
}

func TestGacha_RemoteFailureFallsBackToText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv(wheelOfNamesAPIURLEnv, server.URL+"/api/gif")

	rb, _ := installMisc(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/gacha Rice"))

	sent := rb.Sent()
	if len(sent) != 2 {
		t.Fatalf("calls = %+v, want placeholder then in-place edit", sent)
	}
	if sent[1].Method != "editMessageText" || sent[1].Text() != "★★★ Rice" {
		t.Fatalf("fallback call = %+v, want editMessageText ★★★ Rice", sent[1])
	}
}

func TestGacha_SendAnimationFailureFallsBackToText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(mp4Bytes)
	}))
	defer server.Close()
	t.Setenv(wheelOfNamesAPIURLEnv, server.URL+"/api/gif")

	rb, _ := installMisc(t, 999)
	rb.FailMethod("sendAnimation", http.StatusInternalServerError, "")
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(7, "/gacha Rice"))

	call := rb.LastSent()
	if call.Method != "editMessageText" || call.Text() != "★★★ Rice" {
		t.Fatalf("fallback call = %+v, want editMessageText ★★★ Rice", call)
	}
}

func TestGacha_ForwardsMessageThreadID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(mp4Bytes)
	}))
	defer server.Close()
	t.Setenv(wheelOfNamesAPIURLEnv, server.URL+"/api/gif")

	rb, _ := installMisc(t, 999)
	update := testutil.NewSupergroupMessage(-100, 7, "/gacha Rice")
	update.Message.MessageThreadID = 42
	rb.Bot.ProcessUpdate(context.Background(), update)

	animations := 0
	for _, call := range rb.Sent() {
		switch call.Method {
		case "sendMessage", "sendAnimation":
			if got := call.Form["message_thread_id"]; got != "42" {
				t.Fatalf("%s message_thread_id = %q, want 42", call.Method, got)
			}
			if call.Method == "sendAnimation" {
				animations++
			}
		}
	}
	if animations != 1 {
		t.Fatalf("sendAnimation calls = %d, want 1", animations)
	}
}
