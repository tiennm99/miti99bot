package storage

import (
	"context"
	"errors"
)

// ErrNotFound is returned by KVStore implementations when a key has no value.
var ErrNotFound = errors.New("storage: key not found")

// ErrConflict is returned by conditional writes when the stored value changed.
var ErrConflict = errors.New("storage: write conflict")

// KVStore is the per-module key-value contract. Implementations must be safe
// for concurrent use and must return ErrNotFound for missing keys.
type KVStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	GetJSON(ctx context.Context, key string, dst any) error
	Put(ctx context.Context, key string, val []byte) error
	PutJSON(ctx context.Context, key string, val any) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// VersionedStore is implemented by stores that support version-based optimistic
// locking. It replaces value-bytes compare-and-swap: each key carries a
// monotonic version, so a writer swaps only if the version it read is unchanged.
// This decouples concurrency control from the value encoding (so values can be
// stored as native documents rather than exact byte blobs).
//
// Contract:
//   - GetVersioned returns the value and its current version, or ErrNotFound.
//     A key that exists without a recorded version (written by an older build)
//     reports version 0 and no error.
//   - PutVersioned writes val only if the stored version still equals
//     expectedVersion, then bumps the version. expectedVersion == 0 means
//     "create, or adopt a not-yet-versioned key". A mismatch returns ErrConflict.
type VersionedStore interface {
	GetVersioned(ctx context.Context, key string) (val []byte, version int64, err error)
	PutVersioned(ctx context.Context, key string, expectedVersion int64, val []byte) error
}
