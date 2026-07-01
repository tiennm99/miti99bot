package lol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tiennm99/miti99bot/internal/storage"
)

// subscribersKey is the store slot holding the per-module subscriber list.
const subscribersKey = "subscribers"

// Subscriber is one row in the subscriber list. ThreadID is the Telegram
// forum-topic id the user subscribed from; 0 means the chat's General topic
// (or a non-forum chat). Uniqueness key is (ChatID, ThreadID) so the same
// chat can subscribe independently in multiple topics.
//
// Telegram routes outgoing messages with an absent/zero message_thread_id
// to the General topic, so carrying ThreadID alongside ChatID is what keeps
// the daily push landing in the topic the user subscribed from.
type Subscriber struct {
	ChatID   int64 `json:"chat_id" bson:"chat_id"`
	ThreadID int   `json:"thread_id,omitempty" bson:"thread_id,omitempty"`
}

// subscribersDoc wraps the subscriber list so it can be stored as a named
// root field in a Mongo document (a bare JSON array cannot be a root doc).
type subscribersDoc struct {
	Subscribers []Subscriber `json:"subscribers" bson:"subscribers"`
}

// SubscriberStore is the typed store for subscriber documents.
type SubscriberStore = storage.DocStore[subscribersDoc]

// listSubscribers returns the current subscriber list, or an empty slice
// if none have ever subscribed.
//
// Decoder accepts two shapes for forward-compatibility with rows written
// before topic-aware subscriptions existed:
//   - current: [{"chat_id":123,"thread_id":7}, ...]
//   - legacy:  [123, 456, ...]   (decoded with ThreadID = 0)
//
// The decoder handles legacy rows indefinitely; any successful add/remove
// rewrites the slot in the new shape, but a no-op duplicate add leaves the
// legacy bytes in place — that's fine, the fallback path stays correct.
func listSubscribers(ctx context.Context, store SubscriberStore) ([]Subscriber, error) {
	doc, _, err := store.Get(ctx, subscribersKey)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("lol listSubscribers: %w", err)
	}
	if doc.Subscribers != nil {
		return doc.Subscribers, nil
	}
	return nil, nil
}

// listSubscribersLegacy decodes a raw JSON byte slice that may be either the
// current [{chat_id,thread_id}] shape or the legacy [int64] shape. Used by
// the legacy-migration path when a key was written by an older version.
func listSubscribersLegacy(raw []byte) ([]Subscriber, error) {
	var subs []Subscriber
	if err := json.Unmarshal(raw, &subs); err == nil {
		return subs, nil
	}
	var legacy []int64
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, fmt.Errorf("lol listSubscribers decode: %w", err)
	}
	out := make([]Subscriber, len(legacy))
	for i, id := range legacy {
		out[i] = Subscriber{ChatID: id}
	}
	return out, nil
}

// addSubscriber appends (chatID, threadID) if that exact pair is absent.
// Returns true on first-add, false when already subscribed (idempotent).
//
// Concurrency: the list lives in a single store slot, so a concurrent
// Get→mutate→Put from two chats subscribing in the same millisecond would
// drop one write. Callers MUST serialize through state.subscribersMu (or an
// equivalent module-scoped lock) before calling this.
func addSubscriber(ctx context.Context, store SubscriberStore, chatID int64, threadID int) (bool, error) {
	subs, err := listSubscribers(ctx, store)
	if err != nil {
		return false, err
	}
	for _, s := range subs {
		if s.ChatID == chatID && s.ThreadID == threadID {
			return false, nil
		}
	}
	subs = append(subs, Subscriber{ChatID: chatID, ThreadID: threadID})
	if err := store.Put(ctx, subscribersKey, subscribersDoc{Subscribers: subs}); err != nil {
		return false, fmt.Errorf("lol addSubscriber: %w", err)
	}
	return true, nil
}

// removeSubscriber drops the single (chatID, threadID) entry. Returns true
// when removed, false when that exact pair wasn't present (idempotent).
//
// Concurrency: same single-slot Get→mutate→Put as addSubscriber; callers
// must hold state.subscribersMu.
func removeSubscriber(ctx context.Context, store SubscriberStore, chatID int64, threadID int) (bool, error) {
	subs, err := listSubscribers(ctx, store)
	if err != nil {
		return false, err
	}
	out := make([]Subscriber, 0, len(subs))
	removed := false
	for _, s := range subs {
		if s.ChatID == chatID && s.ThreadID == threadID {
			removed = true
			continue
		}
		out = append(out, s)
	}
	if !removed {
		return false, nil
	}
	if err := store.Put(ctx, subscribersKey, subscribersDoc{Subscribers: out}); err != nil {
		return false, fmt.Errorf("lol removeSubscriber: %w", err)
	}
	return true, nil
}

// removeAllForChat drops every entry for chatID regardless of ThreadID.
// Used when a send fails with a chat-wide terminal error (bot blocked,
// chat deactivated, kicked, deleted) — every topic subscription in that
// chat is dead, not just the one the failing send targeted. Returns the
// number of entries actually removed.
//
// Concurrency: callers must hold state.subscribersMu.
func removeAllForChat(ctx context.Context, store SubscriberStore, chatID int64) (int, error) {
	subs, err := listSubscribers(ctx, store)
	if err != nil {
		return 0, err
	}
	out := make([]Subscriber, 0, len(subs))
	removed := 0
	for _, s := range subs {
		if s.ChatID == chatID {
			removed++
			continue
		}
		out = append(out, s)
	}
	if removed == 0 {
		return 0, nil
	}
	if err := store.Put(ctx, subscribersKey, subscribersDoc{Subscribers: out}); err != nil {
		return 0, fmt.Errorf("lol removeAllForChat: %w", err)
	}
	return removed, nil
}
