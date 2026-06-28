package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// memEntry is one stored value plus its optimistic-locking version.
type memEntry struct {
	val     []byte
	version int64
}

// MemoryKVStore is an in-process KVStore for tests and local smoke runs.
// Data is lost on restart; production uses the MongoDB provider.
type MemoryKVStore struct {
	mu sync.RWMutex
	m  map[string]memEntry
}

// NewMemoryKVStore returns an empty in-memory store.
func NewMemoryKVStore() *MemoryKVStore {
	return &MemoryKVStore{m: make(map[string]memEntry)}
}

func (s *MemoryKVStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.m[key]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]byte, len(e.val))
	copy(out, e.val)
	return out, nil
}

func (s *MemoryKVStore) GetJSON(ctx context.Context, key string, dst any) error {
	raw, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.NewDecoder(bytes.NewReader(raw)).Decode(dst)
}

func (s *MemoryKVStore) Put(_ context.Context, key string, val []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := make([]byte, len(val))
	copy(stored, val)
	// A plain Put bumps the version too, so a concurrent versioned writer that
	// read the old version correctly sees a conflict.
	s.m[key] = memEntry{val: stored, version: s.m[key].version + 1}
	return nil
}

// GetVersioned returns the value and its version, or ErrNotFound.
func (s *MemoryKVStore) GetVersioned(_ context.Context, key string) ([]byte, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.m[key]
	if !ok {
		return nil, 0, ErrNotFound
	}
	out := make([]byte, len(e.val))
	copy(out, e.val)
	return out, e.version, nil
}

// PutVersioned writes val only if the stored version equals expectedVersion
// (0 = must not exist yet), then bumps the version. ErrConflict on mismatch.
func (s *MemoryKVStore) PutVersioned(_ context.Context, key string, expectedVersion int64, val []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[key]
	if expectedVersion == 0 {
		if ok {
			return ErrConflict
		}
	} else if !ok || e.version != expectedVersion {
		return ErrConflict
	}
	stored := make([]byte, len(val))
	copy(stored, val)
	s.m[key] = memEntry{val: stored, version: e.version + 1}
	return nil
}

func (s *MemoryKVStore) PutJSON(ctx context.Context, key string, val any) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return s.Put(ctx, key, raw)
}

func (s *MemoryKVStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}

func (s *MemoryKVStore) List(_ context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0)
	for k := range s.m {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
