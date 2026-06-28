package storage

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// MemoryProvider is a Provider backed by per-module in-process maps. Intended
// for tests and local no-database runs (MODULES= / no MONGO_URL); state is lost
// when the process exits. Production uses MongoProvider.
type MemoryProvider struct {
	mu   sync.Mutex
	cols map[string]*memoryCollection
}

// NewMemoryProvider returns a fresh in-process provider.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{cols: make(map[string]*memoryCollection)}
}

// Collection returns the per-module in-memory collection, creating it on first
// use. An invalid module name yields an invalidCollection so Typed produces a
// store whose every op errors — mirroring MongoProvider's defense in depth.
func (p *MemoryProvider) Collection(module string) Collection {
	if !collectionNameRe.MatchString(module) {
		return invalidCollection{name: module}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.cols[module]
	if !ok {
		c = &memoryCollection{rows: make(map[string]memoryRow)}
		p.cols[module] = c
	}
	return c
}

// memoryCollection holds one module's rows. Multiple typed views (different
// payload types under disjoint key prefixes) share the same map.
type memoryCollection struct {
	mu   sync.RWMutex
	rows map[string]memoryRow
}

func (*memoryCollection) isCollection() {}

// memoryRow stores the JSON-encoded value plus its version. Encoding to bytes
// isolates stored state from caller mutation, matching production where Mongo
// serializes the value on write.
type memoryRow struct {
	version int64
	data    []byte
}

// memoryDocStore is a typed DocStore over one memoryCollection.
type memoryDocStore[T any] struct {
	c *memoryCollection
}

func (s *memoryDocStore[T]) Get(_ context.Context, id string) (T, int64, error) {
	var out T
	if err := validateKey(id); err != nil {
		return out, 0, err
	}
	s.c.mu.RLock()
	defer s.c.mu.RUnlock()
	row, ok := s.c.rows[id]
	if !ok {
		return out, 0, ErrNotFound
	}
	if err := json.Unmarshal(row.data, &out); err != nil {
		return out, 0, err
	}
	return out, row.version, nil
}

func (s *memoryDocStore[T]) Put(_ context.Context, id string, val T) error {
	if err := validateKey(id); err != nil {
		return err
	}
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	// A plain Put bumps the version too, so a concurrent versioned writer that
	// read the old version correctly sees a conflict.
	s.c.rows[id] = memoryRow{version: s.c.rows[id].version + 1, data: data}
	return nil
}

func (s *memoryDocStore[T]) PutVersioned(_ context.Context, id string, expectedVersion int64, val T) error {
	if err := validateKey(id); err != nil {
		return err
	}
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	row, ok := s.c.rows[id]
	if expectedVersion == 0 {
		if ok {
			return ErrConflict
		}
	} else if !ok || row.version != expectedVersion {
		return ErrConflict
	}
	s.c.rows[id] = memoryRow{version: row.version + 1, data: data}
	return nil
}

func (s *memoryDocStore[T]) Delete(_ context.Context, id string) error {
	if err := validateKey(id); err != nil {
		return err
	}
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	delete(s.c.rows, id)
	return nil
}

func (s *memoryDocStore[T]) List(_ context.Context, prefix string) ([]string, error) {
	if err := validatePrefix(prefix); err != nil {
		return nil, err
	}
	s.c.mu.RLock()
	defer s.c.mu.RUnlock()
	keys := make([]string, 0)
	for k := range s.c.rows {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
