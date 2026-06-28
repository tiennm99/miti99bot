// Package keylock serialises compound operations that target the same key
// (typically a chat / user / subject identifier) across goroutines.
//
// Why a separate package: every game module needs a per-subject mutex to
// turn the store's single-op atomicity into safe Get→mutate→Put. The bot
// dispatcher runs each Telegram update in its own goroutine, so without
// explicit per-subject serialisation two updates to the same game could
// race and drop one write.
//
// Trade-off: the underlying sync.Map grows unboundedly with distinct keys
// (~32 B each). At the current bot scale, that is acceptable; add eviction if
// production cardinality starts growing materially.
package keylock

import "sync"

// Map gives each string key its own mutex, lazily created. Zero value is
// usable; do not copy after first use (sync.Map is non-copyable).
type Map struct {
	m sync.Map // key: string → val: *sync.Mutex
}

// Acquire locks the per-key mutex and returns its Unlock as a func so the
// caller can `defer m.Acquire(key)()` at the top of a critical section.
//
// Distinct keys never block each other; same-key callers run serially in the
// order Acquire was called.
func (m *Map) Acquire(key string) func() {
	v, _ := m.m.LoadOrStore(key, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
