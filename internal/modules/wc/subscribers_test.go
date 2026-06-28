package wc

import (
	"context"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
)

func newSubscriberStore() SubscriberStore {
	return storage.Typed[subscribersDoc](storage.NewMemoryProvider().Collection("wc"))
}

func TestSubscribers_AddRemoveAndTopics(t *testing.T) {
	ctx := context.Background()
	store := newSubscriberStore()

	for _, tid := range []int{0, 5, 9} {
		added, err := addSubscriber(ctx, store, 100, tid)
		if err != nil || !added {
			t.Fatalf("add(100,%d): added=%v err=%v", tid, added, err)
		}
	}
	if added, _ := addSubscriber(ctx, store, 100, 5); added {
		t.Fatal("duplicate topic subscription should be no-op")
	}

	if removed, _ := removeSubscriber(ctx, store, 100, 5); !removed {
		t.Fatal("remove(100,5) should remove")
	}
	subs, _ := listSubscribers(ctx, store)
	if len(subs) != 2 {
		t.Fatalf("subs = %v, want 2", subs)
	}
	for _, sub := range subs {
		if sub.ThreadID == 5 {
			t.Fatalf("removed topic still present: %v", subs)
		}
	}
}

func TestSubscribers_RemoveAllForChat(t *testing.T) {
	ctx := context.Background()
	store := newSubscriberStore()
	for _, sub := range []Subscriber{{ChatID: 100}, {ChatID: 100, ThreadID: 9}, {ChatID: 200}} {
		if _, err := addSubscriber(ctx, store, sub.ChatID, sub.ThreadID); err != nil {
			t.Fatal(err)
		}
	}
	n, err := removeAllForChat(ctx, store, 100)
	if err != nil || n != 2 {
		t.Fatalf("removeAllForChat = %d, %v; want 2, nil", n, err)
	}
	subs, _ := listSubscribers(ctx, store)
	if len(subs) != 1 || subs[0].ChatID != 200 {
		t.Fatalf("subs = %v, want only chat 200", subs)
	}
}
