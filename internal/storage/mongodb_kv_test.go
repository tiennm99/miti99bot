package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
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

// TestMongoKVStore_NativeValueRepresentation verifies a JSON object value is
// persisted as a NATIVE BSON document (expandable/queryable in Atlas), and a
// bare non-JSON value as a string — and that both round-trip.
func TestMongoKVStore_NativeValueRepresentation(t *testing.T) {
	s, db, cleanup := mongoLocalSetup(t, "wordle")
	defer cleanup()

	ctx := context.Background()
	obj := []byte(`{"word":"chào","score":42}`) // multi-byte UTF-8 object
	if err := s.Put(ctx, "obj", obj); err != nil {
		t.Fatalf("Put obj: %v", err)
	}
	// A bare string (lolschedule date-guard style) is not JSON → stored as string.
	if err := s.Put(ctx, "bare", []byte("2026-06-28")); err != nil {
		t.Fatalf("Put bare: %v", err)
	}

	var objDoc, bareDoc bson.M
	if err := db.Collection("wordle").FindOne(ctx, bson.M{"_id": "obj"}).Decode(&objDoc); err != nil {
		t.Fatalf("raw FindOne obj: %v", err)
	}
	switch objDoc["value"].(type) {
	case bson.M, bson.D:
		// native object — good
	default:
		t.Errorf("object value stored as %T, want native bson document", objDoc["value"])
	}
	if err := db.Collection("wordle").FindOne(ctx, bson.M{"_id": "bare"}).Decode(&bareDoc); err != nil {
		t.Fatalf("raw FindOne bare: %v", err)
	}
	if _, ok := bareDoc["value"].(string); !ok {
		t.Errorf("bare value stored as %T, want string", bareDoc["value"])
	}

	// Round-trips.
	if got, _ := s.Get(ctx, "bare"); string(got) != "2026-06-28" {
		t.Errorf("bare round trip: got %q", got)
	}
	var back map[string]any
	if err := s.GetJSON(ctx, "obj", &back); err != nil {
		t.Fatalf("GetJSON obj: %v", err)
	}
	if back["word"] != "chào" {
		t.Errorf("object round trip lost data: %v", back)
	}
}

// TestMongoKVStore_Int64Fidelity is the codec gate: an int64 field must survive
// the JSON↔BSON round trip as an integer, not collapse to a float (which would
// make GetJSON into an int64 field fail or drift).
func TestMongoKVStore_Int64Fidelity(t *testing.T) {
	s, db, cleanup := mongoLocalSetup(t, "coin")
	defer cleanup()

	ctx := context.Background()
	type meta struct {
		CreatedAt int64   `json:"createdAt"` // UnixMilli ~1.7e12
		Invested  float64 `json:"invested"`
	}
	type portfolio struct {
		USD  float64          `json:"usd"`
		Meta meta             `json:"meta"`
		Qty  map[string]int64 `json:"qty"`
	}
	in := portfolio{USD: 1234.56, Meta: meta{CreatedAt: 1719500000000, Invested: 5000}, Qty: map[string]int64{"BTC": 3}}
	if err := s.PutJSON(ctx, "u1", in); err != nil {
		t.Fatalf("PutJSON: %v", err)
	}
	var out portfolio
	if err := s.GetJSON(ctx, "u1", &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("fidelity lost: got %+v, want %+v", out, in)
	}
	// The stored CreatedAt must be a BSON int (int32/int64), not a double.
	var doc bson.M
	_ = db.Collection("coin").FindOne(ctx, bson.M{"_id": "u1"}).Decode(&doc)
	if m, ok := doc["value"].(bson.M); ok {
		if meta, ok := m["meta"].(bson.M); ok {
			switch meta["createdAt"].(type) {
			case int64, int32:
				// good — preserved as integer
			default:
				t.Errorf("createdAt stored as %T, want int64 (json.Number codec)", meta["createdAt"])
			}
		}
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

func TestMongoKVStore_Versioned(t *testing.T) {
	s, _, cleanup := mongoLocalSetup(t, "gold")
	defer cleanup()

	ctx := context.Background()
	// Create-if-absent (version 0) succeeds once, then conflicts.
	if err := s.PutVersioned(ctx, "user:1", 0, []byte("v1")); err != nil {
		t.Fatalf("PutVersioned create: %v", err)
	}
	if err := s.PutVersioned(ctx, "user:1", 0, []byte("v2")); !errors.Is(err, ErrConflict) {
		t.Errorf("create over existing: got %v, want ErrConflict", err)
	}

	val, ver, err := s.GetVersioned(ctx, "user:1")
	if err != nil || string(val) != "v1" || ver != 1 {
		t.Fatalf("GetVersioned: got (%q, v=%d, %v), want (v1, 1, nil)", val, ver, err)
	}

	// Swap with matching version succeeds and bumps; stale version conflicts.
	if err := s.PutVersioned(ctx, "user:1", 1, []byte("v2")); err != nil {
		t.Fatalf("swap matching version: %v", err)
	}
	if err := s.PutVersioned(ctx, "user:1", 1, []byte("v3")); !errors.Is(err, ErrConflict) {
		t.Errorf("stale version: got %v, want ErrConflict", err)
	}
	if got, _ := s.Get(ctx, "user:1"); string(got) != "v2" {
		t.Errorf("stored value = %q, want %q", got, "v2")
	}

	// Swap on a missing key conflicts.
	if err := s.PutVersioned(ctx, "user:missing", 1, []byte("v2")); !errors.Is(err, ErrConflict) {
		t.Errorf("swap missing key: got %v, want ErrConflict", err)
	}
}

// TestMongoKVStore_PutVersioned_AdoptsLegacyDoc proves a document written by an
// older build (no version field) is treated as version 0 and updated cleanly,
// so existing users' portfolios keep working after the rollout.
func TestMongoKVStore_PutVersioned_AdoptsLegacyDoc(t *testing.T) {
	s, db, cleanup := mongoLocalSetup(t, "coin")
	defer cleanup()

	ctx := context.Background()
	// Seed a legacy doc: value string, NO version field.
	if _, err := db.Collection("coin").InsertOne(ctx, bson.M{"_id": "user:9", "value": "legacy"}); err != nil {
		t.Fatalf("seed legacy doc: %v", err)
	}
	val, ver, err := s.GetVersioned(ctx, "user:9")
	if err != nil || string(val) != "legacy" || ver != 0 {
		t.Fatalf("GetVersioned legacy: got (%q, v=%d, %v), want (legacy, 0, nil)", val, ver, err)
	}
	// Claim it at version 0 — must succeed (no spurious conflict).
	if err := s.PutVersioned(ctx, "user:9", 0, []byte("migrated")); err != nil {
		t.Fatalf("adopt legacy doc: %v", err)
	}
	if got, ver, _ := s.GetVersioned(ctx, "user:9"); string(got) != "migrated" || ver != 1 {
		t.Errorf("after adopt: got (%q, v=%d), want (migrated, 1)", got, ver)
	}
}

// TestMongoKVStore_PutVersioned_ConcurrentCreate proves the absent-create race
// is linearizable: N goroutines racing PutVersioned(_, 0, _) on one key yield
// exactly one winner; losers get ErrConflict. Blocking correctness gate.
func TestMongoKVStore_PutVersioned_ConcurrentCreate(t *testing.T) {
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
			<-start
			err := s.PutVersioned(ctx, "race", 0, []byte(fmt.Sprintf("v%d", i)))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, ErrConflict):
				conflicts++
			default:
				others++
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if wins != 1 {
		t.Errorf("concurrent create: got %d winners, want exactly 1", wins)
	}
	if conflicts != n-1 {
		t.Errorf("concurrent create: got %d conflicts, want %d", conflicts, n-1)
	}
	if others != 0 {
		t.Errorf("concurrent create produced %d non-conflict errors", others)
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
