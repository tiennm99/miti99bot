package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// mongoLocalSetup connects to a local MongoDB and returns a store over a fresh,
// uniquely-named collection. Tests skip if MONGODB_TEST_URL is unset so CI
// without a Mongo container still builds (mirrors the DynamoDB Local gating).
func mongoLocalSetup(t *testing.T, module string) (*MongoKVStore, *mongo.Database, func()) {
	t.Helper()
	uri := os.Getenv("MONGODB_TEST_URL")
	if uri == "" {
		t.Skip("MONGODB_TEST_URL not set; skipping MongoDB integration test (run `make mongo-local` to start the local container)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewMongoClient(ctx, uri)
	if err != nil {
		t.Fatalf("NewMongoClient: %v", err)
	}
	// Unique DB per test so parallel runs and cross-module isolation checks
	// never collide. Mongo db names are capped at 63 bytes.
	dbName := fmt.Sprintf("miti99bot_test_%d", time.Now().UnixNano())
	if len(dbName) > 63 {
		dbName = dbName[:63]
	}
	db := client.Database(dbName)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	}
	return NewMongoKVStore(db.Collection(module), module), db, cleanup
}

func TestMongoKVStore_PutGetDelete(t *testing.T) {
	s, _, cleanup := mongoLocalSetup(t, "wordle")
	defer cleanup()

	ctx := context.Background()
	if err := s.Put(ctx, "user:1:state", []byte("hello")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, "user:1:state")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("Get: got %q, want %q", got, "hello")
	}

	// Overwrite preserves byte fidelity.
	if err := s.Put(ctx, "user:1:state", []byte("world")); err != nil {
		t.Fatalf("Put overwrite: %v", err)
	}
	got, _ = s.Get(ctx, "user:1:state")
	if string(got) != "world" {
		t.Errorf("Get after overwrite: got %q, want %q", got, "world")
	}

	if err := s.Delete(ctx, "user:1:state"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "user:1:state"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestMongoKVStore_GetMissing(t *testing.T) {
	s, _, cleanup := mongoLocalSetup(t, "wordle")
	defer cleanup()

	if _, err := s.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestMongoKVStore_NonUTF8RoundTrip(t *testing.T) {
	s, _, cleanup := mongoLocalSetup(t, "wordle")
	defer cleanup()

	ctx := context.Background()
	raw := []byte{0x00, 0xff, 0xfe, 0x01, 0x80}
	if err := s.Put(ctx, "bin", raw); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, "bin")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(raw) {
		t.Errorf("non-UTF-8 round trip: got %v, want %v", got, raw)
	}
}

func TestMongoKVStore_JSONRoundTrip(t *testing.T) {
	s, _, cleanup := mongoLocalSetup(t, "loldle")
	defer cleanup()

	ctx := context.Background()
	type state struct {
		Score int    `json:"score"`
		Name  string `json:"name"`
	}
	in := state{Score: 42, Name: "ezreal"}
	if err := s.PutJSON(ctx, "u1", in); err != nil {
		t.Fatalf("PutJSON: %v", err)
	}
	var out state
	if err := s.GetJSON(ctx, "u1", &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if out != in {
		t.Errorf("got %+v, want %+v", out, in)
	}
}

func TestMongoKVStore_ListPrefix(t *testing.T) {
	s, _, cleanup := mongoLocalSetup(t, "wordle")
	defer cleanup()

	ctx := context.Background()
	for _, k := range []string{"user:1:state", "user:2:state", "config:daily", "user:1:history"} {
		if err := s.Put(ctx, k, []byte("x")); err != nil {
			t.Fatalf("Put %s: %v", k, err)
		}
	}

	got, err := s.List(ctx, "user:")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := map[string]bool{"user:1:state": true, "user:2:state": true, "user:1:history": true}
	if len(got) != len(want) {
		t.Errorf("List: got %v (len=%d), want len=%d", got, len(got), len(want))
	}
	for _, k := range got {
		if !want[k] {
			t.Errorf("List: unexpected key %q", k)
		}
	}

	// Empty prefix returns everything in the collection.
	all, err := s.List(ctx, "")
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("List(\"\"): got %d keys, want 4", len(all))
	}
}

func TestMongoKVStore_CompareAndSwap(t *testing.T) {
	s, _, cleanup := mongoLocalSetup(t, "gold")
	defer cleanup()

	ctx := context.Background()
	// Create-if-absent succeeds once, then conflicts.
	if err := s.CompareAndSwap(ctx, "user:1", nil, []byte("v1")); err != nil {
		t.Fatalf("CompareAndSwap create: %v", err)
	}
	if err := s.CompareAndSwap(ctx, "user:1", nil, []byte("v2")); !errors.Is(err, ErrConflict) {
		t.Errorf("CompareAndSwap create over existing: got %v, want ErrConflict", err)
	}

	// Swap with matching expected succeeds; stale expected conflicts.
	if err := s.CompareAndSwap(ctx, "user:1", []byte("v1"), []byte("v2")); err != nil {
		t.Fatalf("CompareAndSwap matching: %v", err)
	}
	if err := s.CompareAndSwap(ctx, "user:1", []byte("v1"), []byte("v3")); !errors.Is(err, ErrConflict) {
		t.Errorf("CompareAndSwap stale: got %v, want ErrConflict", err)
	}
	got, err := s.Get(ctx, "user:1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v2" {
		t.Errorf("stored value = %q, want %q", got, "v2")
	}

	// Non-nil expected on a missing key conflicts (caller reloads and retries).
	if err := s.CompareAndSwap(ctx, "user:missing", []byte("v1"), []byte("v2")); !errors.Is(err, ErrConflict) {
		t.Errorf("CompareAndSwap missing key: got %v, want ErrConflict", err)
	}
}

// TestMongoKVStore_CompareAndSwap_ConcurrentInsert proves the absent-insert
// race is linearizable: N goroutines racing a nil-expected CAS on the same key
// must yield exactly one winner; every loser gets ErrConflict (never a silent
// overwrite). This is the blocking correctness gate from Phase 1.
func TestMongoKVStore_CompareAndSwap_ConcurrentInsert(t *testing.T) {
	s, _, cleanup := mongoLocalSetup(t, "coin")
	defer cleanup()

	ctx := context.Background()
	const n = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	var wins, conflicts, others int
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release all goroutines at once to maximize contention
			err := s.CompareAndSwap(ctx, "race", nil, []byte(fmt.Sprintf("v%d", i)))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, ErrConflict):
				conflicts++
			default:
				others++
				t.Errorf("unexpected CAS error: %v", err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if wins != 1 {
		t.Errorf("concurrent nil-expected CAS: got %d winners, want exactly 1", wins)
	}
	if conflicts != n-1 {
		t.Errorf("concurrent nil-expected CAS: got %d conflicts, want %d", conflicts, n-1)
	}
	if others != 0 {
		t.Errorf("concurrent CAS produced %d non-conflict errors", others)
	}
}

// TestMongoKVStore_GetDocWithoutValueField covers a malformed document edge
// case (a doc that exists but lacks the value field) — Get must surface a clear
// error, not panic or return empty bytes silently.
func TestMongoKVStore_GetDocWithoutValueField(t *testing.T) {
	s, db, cleanup := mongoLocalSetup(t, "wordle")
	defer cleanup()

	ctx := context.Background()
	// Insert a doc directly with no `value` field.
	if _, err := db.Collection("wordle").InsertOne(ctx, bson.M{"_id": "novalue", "other": 1}); err != nil {
		t.Fatalf("seed InsertOne: %v", err)
	}
	if _, err := s.Get(ctx, "novalue"); err == nil {
		t.Errorf("Get on doc without value field: got nil error, want a descriptive error")
	}
}

func TestMongoKVStore_DeleteMissingNoError(t *testing.T) {
	s, _, cleanup := mongoLocalSetup(t, "wordle")
	defer cleanup()

	if err := s.Delete(context.Background(), "never-existed"); err != nil {
		t.Errorf("Delete missing key: got %v, want nil (idempotent)", err)
	}
}
