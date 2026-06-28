package storage

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// ErrNotFound is returned by a DocStore when a key has no value.
var ErrNotFound = errors.New("storage: key not found")

// ErrConflict is returned by versioned writes when the stored version changed.
var ErrConflict = errors.New("storage: write conflict")

// ErrInvalidModuleName is returned by every op on a store built from an invalid
// module/collection name — surfacing the configuration bug at first use rather
// than silently reading/writing an attacker-controllable collection name.
var ErrInvalidModuleName = errors.New("storage: invalid module name")

// DocStore is the per-module, per-payload-type storage contract. A value is
// persisted as a native document (its fields hoisted to the document root)
// alongside an optimistic-locking version. Implementations are safe for
// concurrent use and return ErrNotFound for missing keys.
//
// Concurrency control is version-based: each key carries a monotonic version, so
// a writer swaps only if the version it read is unchanged. This decouples the
// concurrency control from the value encoding (values are native documents, not
// exact byte blobs).
//
// Contract:
//   - Get returns the value and its current version, or ErrNotFound.
//   - Put overwrites unconditionally and bumps the version.
//   - PutVersioned writes only if the stored version equals expectedVersion,
//     then bumps it. expectedVersion == 0 means "create (or adopt a not-yet-
//     versioned key)". A mismatch returns ErrConflict.
type DocStore[T any] interface {
	Get(ctx context.Context, id string) (val T, version int64, err error)
	Put(ctx context.Context, id string, val T) error
	PutVersioned(ctx context.Context, id string, expectedVersion int64, val T) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// Provider yields a per-module Collection handle. Implementations decide how
// isolation is achieved: MemoryProvider keeps one map per module, MongoProvider
// uses one MongoDB collection per module. Modules never construct stores
// directly — they receive a Collection through their factory's Deps and build
// typed views with Typed.
type Provider interface {
	Collection(module string) Collection
}

// Collection is an opaque per-module storage handle. Build a typed store over it
// with Typed[T]. A module may build several typed views over the same
// Collection (one per payload type) as long as their key prefixes are disjoint.
type Collection interface {
	// isCollection seals the interface to this package's implementations so
	// Typed's type switch is exhaustive.
	isCollection()
}

// Typed returns a DocStore[T] over c. Go methods cannot be generic, so this is a
// package-level function rather than a Provider method; it is the single switch
// point that binds a Collection to its backend's typed store.
//
// It panics if T is a struct whose BSON field names collide with a reserved root
// field (_id, version, updatedAt) — a programmer error caught at startup, in the
// same spirit as Prefixed panicking on an empty prefix.
func Typed[T any](c Collection) DocStore[T] {
	if err := checkReservedFields[T](); err != nil {
		panic(err)
	}
	switch h := c.(type) {
	case mongoCollection:
		return &mongoDocStore[T]{coll: h.coll, module: h.module}
	case *memoryCollection:
		return &memoryDocStore[T]{c: h}
	case invalidCollection:
		return invalidDocStore[T](h)
	default:
		panic(fmt.Sprintf("storage: unknown collection type %T", c))
	}
}

// reservedRootFields are the document-root field names the store owns; a payload
// type must not define BSON tags that collide with them (inline marshalling
// would otherwise produce duplicate keys).
var reservedRootFields = map[string]bool{
	mongoIDField:        true,
	mongoVersionField:   true,
	mongoUpdatedAtField: true,
}

// checkReservedFields reports an error if struct type T declares a BSON field
// name colliding with a reserved root field. Non-struct T (e.g. memory-only
// test usage) is skipped — only the inline Mongo encoding needs the guard.
func checkReservedFields[T any]() error {
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := bsonFieldName(f)
		if reservedRootFields[name] {
			return fmt.Errorf("storage: payload type %s field %q maps to reserved root field %q", t.Name(), f.Name, name)
		}
	}
	return nil
}

// bsonFieldName returns the BSON document field name for a struct field: the
// explicit `bson:"name"` tag if present, else the Go field name (matching the
// driver's default of using the field name verbatim).
func bsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("bson")
	if tag == "" {
		return f.Name
	}
	name := strings.Split(tag, ",")[0]
	if name == "" {
		return f.Name
	}
	return name
}

// invalidCollection is returned by a Provider for a module name that fails
// validation. Typed turns it into an invalidDocStore.
type invalidCollection struct{ name string }

func (invalidCollection) isCollection() {}

// invalidDocStore errors on every operation, wrapping ErrInvalidModuleName.
type invalidDocStore[T any] struct{ name string }

func (s invalidDocStore[T]) err(op string) error {
	return fmt.Errorf("%w: %q (op=%s)", ErrInvalidModuleName, s.name, op)
}

func (s invalidDocStore[T]) Get(context.Context, string) (T, int64, error) {
	var z T
	return z, 0, s.err("Get")
}
func (s invalidDocStore[T]) Put(context.Context, string, T) error { return s.err("Put") }
func (s invalidDocStore[T]) PutVersioned(context.Context, string, int64, T) error {
	return s.err("PutVersioned")
}
func (s invalidDocStore[T]) Delete(context.Context, string) error { return s.err("Delete") }
func (s invalidDocStore[T]) List(context.Context, string) ([]string, error) {
	return nil, s.err("List")
}
