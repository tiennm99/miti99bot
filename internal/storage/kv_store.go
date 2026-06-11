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

// CompareAndSwapStore is implemented by stores that can conditionally replace
// a value only when it still equals the bytes read by the caller. A nil expected
// value means the key must not exist.
type CompareAndSwapStore interface {
	CompareAndSwap(ctx context.Context, key string, expected []byte, val []byte) error
}
