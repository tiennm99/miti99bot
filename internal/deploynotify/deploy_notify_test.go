package deploynotify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
)

// newNotifyStore returns a fresh in-memory typed notifyRecord store for tests.
func newNotifyStore() storage.DocStore[notifyRecord] {
	return storage.Typed[notifyRecord](storage.NewMemoryProvider().Collection("deploynotify"))
}

func TestShouldNotify_FirstRun(t *testing.T) {
	store := newNotifyStore()
	got, err := shouldNotify(context.Background(), store, "abc123")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !got {
		t.Errorf("first run with empty store → want true, got false")
	}
}

func TestShouldNotify_SameSHA(t *testing.T) {
	store := newNotifyStore()
	if err := markNotified(context.Background(), store, "abc123"); err != nil {
		t.Fatal(err)
	}
	got, err := shouldNotify(context.Background(), store, "abc123")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got {
		t.Errorf("same SHA → want false, got true")
	}
}

func TestShouldNotify_DifferentSHA(t *testing.T) {
	store := newNotifyStore()
	if err := markNotified(context.Background(), store, "old111"); err != nil {
		t.Fatal(err)
	}
	got, err := shouldNotify(context.Background(), store, "new222")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !got {
		t.Errorf("changed SHA → want true, got false")
	}
}

// recorder is a Sender that captures the last (chatID, text) it received and
// flags whether it was ever invoked. Replaces a full RecordingBot since this
// package only needs send-or-not signal.
type recorder struct {
	called bool
	chatID int64
	text   string
	err    error
}

func (r *recorder) send(_ context.Context, chatID int64, text string) error {
	r.called = true
	r.chatID = chatID
	r.text = text
	return r.err
}

func TestRun_SkipsWhenSHAEmpty(t *testing.T) {
	store := newNotifyStore()
	rec := &recorder{}
	Run(context.Background(), Config{
		Store:   store,
		OwnerID: 42,
		GitSHA:  "",
		Sender:  rec.send,
	})
	if rec.called {
		t.Errorf("empty SHA must not send; got call with text=%q", rec.text)
	}
	// And no store write either.
	if _, _, err := store.Get(context.Background(), storeKey); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("empty SHA must not write store; got err=%v", err)
	}
}

func TestRun_SkipsWhenNoOwner(t *testing.T) {
	store := newNotifyStore()
	rec := &recorder{}
	Run(context.Background(), Config{
		Store:   store,
		OwnerID: 0,
		GitSHA:  "abc123",
		Sender:  rec.send,
	})
	if rec.called {
		t.Errorf("zero owner must not send")
	}
	if _, _, err := store.Get(context.Background(), storeKey); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("zero owner must not write store; got err=%v", err)
	}
}

func TestRun_SendsAndPersistsOnFirstRun(t *testing.T) {
	store := newNotifyStore()
	rec := &recorder{}
	Run(context.Background(), Config{
		Store:   store,
		OwnerID: 42,
		GitSHA:  "abc123",
		Sender:  rec.send,
	})
	if !rec.called {
		t.Fatalf("first run with fresh store must send")
	}
	if rec.chatID != 42 {
		t.Errorf("chatID = %d, want 42", rec.chatID)
	}
	if !strings.Contains(rec.text, "abc123") {
		t.Errorf("message %q missing SHA", rec.text)
	}
	// Store must now hold the SHA so the next Run is silent.
	got, _, err := store.Get(context.Background(), storeKey)
	if err != nil {
		t.Fatalf("post-send store read: %v", err)
	}
	if got.SHA != "abc123" {
		t.Errorf("persisted SHA = %q, want abc123", got.SHA)
	}
}

func TestRun_SilentOnSecondRunSameSHA(t *testing.T) {
	store := newNotifyStore()
	if err := markNotified(context.Background(), store, "abc123"); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	Run(context.Background(), Config{
		Store:   store,
		OwnerID: 42,
		GitSHA:  "abc123",
		Sender:  rec.send,
	})
	if rec.called {
		t.Errorf("repeat run with same SHA must not send")
	}
}

func TestRun_DoesNotPersistOnSendFailure(t *testing.T) {
	store := newNotifyStore()
	rec := &recorder{err: errors.New("telegram is down")}
	Run(context.Background(), Config{
		Store:   store,
		OwnerID: 42,
		GitSHA:  "abc123",
		Sender:  rec.send,
	})
	if !rec.called {
		t.Fatalf("sender should have been called")
	}
	// Send failed → SHA must NOT be persisted, so the next cold start retries.
	if _, _, err := store.Get(context.Background(), storeKey); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("failed send must not write store; got err=%v", err)
	}
}

func TestRenderMessage_ContainsSHA(t *testing.T) {
	got := renderMessage("deadbeef")
	if !strings.Contains(got, "deadbeef") {
		t.Errorf("message %q missing SHA", got)
	}
	if !strings.Contains(got, "miti99bot") {
		t.Errorf("message %q missing bot name", got)
	}
}
