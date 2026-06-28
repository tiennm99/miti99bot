package wc

import (
	"context"
	"errors"
	"fmt"

	"github.com/tiennm99/miti99bot/internal/storage"
)

const subscribersKey = "subscribers"

// Subscriber is one chat/topic subscribed to the daily World Cup digest.
type Subscriber struct {
	ChatID   int64 `json:"chat_id" bson:"chat_id"`
	ThreadID int   `json:"thread_id,omitempty" bson:"thread_id,omitempty"`
}

type subscribersDoc struct {
	Subscribers []Subscriber `json:"subscribers" bson:"subscribers"`
}

// SubscriberStore is the typed store for subscriber documents.
type SubscriberStore = storage.DocStore[subscribersDoc]

func listSubscribers(ctx context.Context, store SubscriberStore) ([]Subscriber, error) {
	doc, _, err := store.Get(ctx, subscribersKey)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("wc listSubscribers: %w", err)
	}
	return doc.Subscribers, nil
}

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
		return false, fmt.Errorf("wc addSubscriber: %w", err)
	}
	return true, nil
}

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
		return false, fmt.Errorf("wc removeSubscriber: %w", err)
	}
	return true, nil
}

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
		return 0, fmt.Errorf("wc removeAllForChat: %w", err)
	}
	return removed, nil
}
