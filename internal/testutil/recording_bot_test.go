package testutil

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
)

// Smoke test: the recording bot must capture an outbound SendMessage so
// downstream handler tests can rely on Sent() / AssertSentText.
func TestRecordingBot_CapturesSendMessage(t *testing.T) {
	rb := NewRecordingBot(t)

	_, err := rb.Bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID: int64(42),
		Text:   "hello",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	calls := rb.Sent()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1: %+v", len(calls), calls)
	}
	if calls[0].Method != "sendMessage" {
		t.Errorf("method = %q, want sendMessage", calls[0].Method)
	}
	if calls[0].Text() != "hello" {
		t.Errorf("text = %q, want hello", calls[0].Text())
	}
	if calls[0].ChatID() != "42" {
		t.Errorf("chat_id = %q, want 42", calls[0].ChatID())
	}
}

func TestRecordingBot_AssertSentText(t *testing.T) {
	rb := NewRecordingBot(t)
	_, err := rb.Bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID: int64(1),
		Text:   "Welcome to the bot",
	})
	if err != nil {
		t.Fatal(err)
	}
	rb.AssertSentText(t, "Welcome")
}

func TestRecordingBot_Reset(t *testing.T) {
	rb := NewRecordingBot(t)
	_, _ = rb.Bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID: int64(1), Text: "first",
	})
	rb.Reset()
	if got := len(rb.Sent()); got != 0 {
		t.Errorf("Sent() after Reset = %d, want 0", got)
	}
}

func TestUpdateBuilders_BotCommandEntity(t *testing.T) {
	tests := []struct {
		text string
		off  int
		ln   int
	}{
		{"/wordle", 0, 7},
		{"/wordle apple", 0, 7},
		{"/wordle@bot apple", 0, 7},
	}
	for _, tt := range tests {
		got := botCommandEntity(tt.text)
		if got.Offset != tt.off || got.Length != tt.ln {
			t.Errorf("botCommandEntity(%q) = (%d,%d), want (%d,%d)",
				tt.text, got.Offset, got.Length, tt.off, tt.ln)
		}
	}
}

// StubMethod exists because the bare harness answers every non-message method
// with `{"ok":true,"result":true}`, which any struct-decoding method rejects.
// Without it, getStickerSet/getFile/uploadStickerFile/getMe can only error.
func TestRecordingBot_StubMethodDecodesIntoStruct(t *testing.T) {
	rb := NewRecordingBot(t)
	rb.StubMethod("getStickerSet", `{"name":"pack_by_bot","title":"My Pack","sticker_type":"regular","stickers":[{"file_id":"f1","file_unique_id":"u1","type":"regular"}]}`)

	set, err := rb.Bot.GetStickerSet(context.Background(), &bot.GetStickerSetParams{Name: "pack_by_bot"})
	if err != nil {
		t.Fatalf("GetStickerSet: %v", err)
	}
	if set.Name != "pack_by_bot" || set.Title != "My Pack" {
		t.Errorf("set = %+v, want name pack_by_bot / title My Pack", set)
	}
	if len(set.Stickers) != 1 || set.Stickers[0].FileID != "f1" {
		t.Errorf("stickers = %+v, want one sticker f1", set.Stickers)
	}
}

// A registered failure must beat a registered stub, so a test can override a
// stubbed happy path without unregistering it.
func TestRecordingBot_FailureWinsOverStub(t *testing.T) {
	rb := NewRecordingBot(t)
	rb.StubMethod("getStickerSet", `{"name":"pack_by_bot"}`)
	rb.FailMethodCode("getStickerSet", 400, "Bad Request: STICKERSET_INVALID")

	if _, err := rb.Bot.GetStickerSet(context.Background(), &bot.GetStickerSetParams{Name: "pack_by_bot"}); err == nil {
		t.Fatal("GetStickerSet succeeded; want the registered failure to win over the stub")
	}
}

// The library classifies errors by the error_code in the response body, not the
// HTTP status (raw_request.go), so only a coded failure produces the sentinel
// that production error handling matches on.
func TestRecordingBot_FailMethodCodeYieldsSentinel(t *testing.T) {
	rb := NewRecordingBot(t)
	rb.FailMethodCode("getStickerSet", 400, "Bad Request: STICKERSET_INVALID")

	_, err := rb.Bot.GetStickerSet(context.Background(), &bot.GetStickerSetParams{Name: "gone_by_bot"})
	if err == nil {
		t.Fatal("GetStickerSet succeeded; want an error")
	}
	if !errors.Is(err, bot.ErrorBadRequest) {
		t.Errorf("err = %v, want errors.Is(err, bot.ErrorBadRequest)", err)
	}
	if !strings.Contains(err.Error(), "STICKERSET_INVALID") {
		t.Errorf("err = %v, want it to carry the MTProto code", err)
	}
}

// The contrast that FailMethod's doc comment promises: a codeless failure does
// NOT take the sentinel shape. Handlers that classify with errors.Is must be
// tested through FailMethodCode instead.
func TestRecordingBot_FailMethodIsCodeless(t *testing.T) {
	rb := NewRecordingBot(t)
	rb.FailMethod("getStickerSet", 400, `{"ok":false,"description":"Bad Request: STICKERSET_INVALID"}`)

	_, err := rb.Bot.GetStickerSet(context.Background(), &bot.GetStickerSetParams{Name: "gone_by_bot"})
	if err == nil {
		t.Fatal("GetStickerSet succeeded; want an error")
	}
	if errors.Is(err, bot.ErrorBadRequest) {
		t.Errorf("err = %v is bot.ErrorBadRequest; a codeless failure must not classify", err)
	}
}

// A malformed multipart body must not be answered 200 with an empty form.
//
// The harness tolerates a parse failure only for parameterless calls, which
// send no parts at all. Widening that to every parse failure would make the
// server answer a corrupt request as though it carried no fields — quietly
// satisfying any test that asserts a field is absent.
func TestRecordingBot_RejectsMalformedMultipart(t *testing.T) {
	rb := NewRecordingBot(t)

	resp, err := http.Post(rb.Server.URL+"/bottest-token/sendMessage",
		"multipart/form-data; boundary=zzz", strings.NewReader("not a multipart body at all"))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for a corrupt multipart body", resp.StatusCode, http.StatusBadRequest)
	}
}

// A parameterless call sends a multipart content type with no parts, and must
// still be served — several API methods take no arguments.
func TestRecordingBot_ServesParameterlessCall(t *testing.T) {
	rb := NewRecordingBot(t)
	rb.StubMethod("getMe", `{"id":7,"is_bot":true,"first_name":"T","username":"testbot"}`)

	me, err := rb.Bot.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if me.Username != "testbot" {
		t.Errorf("username = %q, want testbot", me.Username)
	}
}
