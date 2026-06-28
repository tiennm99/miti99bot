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

// mongoLocalSetup connects to a local MongoDB and returns a fresh, uniquely
// named database plus cleanup. Tests skip if MONGODB_TEST_URL is unset so CI
// without a Mongo container still builds.
func mongoLocalSetup(t *testing.T) (*mongo.Database, func()) {
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
	return db, cleanup
}

type portfolioLike struct {
	USD    float64            `json:"usd" bson:"usd"`
	Assets map[string]float64 `json:"assets" bson:"assets"`
}

func mongoStore[T any](t *testing.T, module string) (DocStore[T], *mongo.Collection, func()) {
	t.Helper()
	db, cleanup := mongoLocalSetup(t)
	coll := db.Collection(module)
	return Typed[T](NewMongoProvider(db).Collection(module)), coll, cleanup
}

// rawDoc fetches the on-disk BSON document for assertions about its shape.
func rawDoc(t *testing.T, coll *mongo.Collection, id string) bson.M {
	t.Helper()
	var doc bson.M
	if err := coll.FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatalf("raw FindOne %s: %v", id, err)
	}
	return doc
}

func TestMongoDocStore_RootShape(t *testing.T) {
	store, coll, cleanup := mongoStore[portfolioLike](t, "coin")
	defer cleanup()
	ctx := context.Background()

	if err := store.Put(ctx, "user:7", portfolioLike{USD: 1000.25, Assets: map[string]float64{"BTC": 1}}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	doc := rawDoc(t, coll, "user:7")

	if _, ok := doc["value"]; ok {
		t.Error("doc has legacy 'value' envelope field")
	}
	if _, ok := doc["_payload"]; ok {
		t.Error("doc has '_payload' fallback field")
	}
	if _, ok := doc["usd"]; !ok {
		t.Error("payload field 'usd' not hoisted to root")
	}
	if _, ok := doc["assets"]; !ok {
		t.Error("payload field 'assets' not hoisted to root")
	}
	if _, ok := doc["updatedAt"].(bson.DateTime); !ok {
		t.Errorf("updatedAt is %T, want BSON Date", doc["updatedAt"])
	}

	got, version, err := store.Get(ctx, "user:7")
	if err != nil || version != 1 || got.USD != 1000.25 || got.Assets["BTC"] != 1 {
		t.Fatalf("Get round-trip: %+v v=%d err=%v", got, version, err)
	}
}

func TestMongoDocStore_OverwriteRemovesStaleFields(t *testing.T) {
	store, coll, cleanup := mongoStore[map[string]int](t, "misc")
	defer cleanup()
	ctx := context.Background()

	if err := store.Put(ctx, "k", map[string]int{"a": 1, "b": 2}); err != nil {
		t.Fatalf("Put A: %v", err)
	}
	if err := store.Put(ctx, "k", map[string]int{"a": 3}); err != nil {
		t.Fatalf("Put B: %v", err)
	}
	doc := rawDoc(t, coll, "k")
	if _, ok := doc["b"]; ok {
		t.Error("stale field 'b' survived overwrite")
	}
	if doc["a"] != int32(3) && doc["a"] != int64(3) {
		t.Errorf("a = %v (%T), want 3", doc["a"], doc["a"])
	}
}

func TestMongoDocStore_PutVersionedConcurrentCreate(t *testing.T) {
	store, _, cleanup := mongoStore[portfolioLike](t, "coin")
	defer cleanup()
	ctx := context.Background()

	const n = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := store.PutVersioned(ctx, "race", 0, portfolioLike{USD: 1}); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			} else if !errors.Is(err, ErrConflict) {
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("concurrent create winners = %d, want exactly 1", wins)
	}
}

func TestMongoDocStore_PutVersionedStaleConflict(t *testing.T) {
	store, _, cleanup := mongoStore[portfolioLike](t, "coin")
	defer cleanup()
	ctx := context.Background()

	_ = store.PutVersioned(ctx, "k", 0, portfolioLike{USD: 1})
	if err := store.PutVersioned(ctx, "k", 99, portfolioLike{USD: 2}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale CAS = %v, want ErrConflict", err)
	}
	if err := store.PutVersioned(ctx, "k", 1, portfolioLike{USD: 2}); err != nil {
		t.Fatalf("fresh CAS: %v", err)
	}
}

// wrappedScalar / wrappedArray prove non-object values become named root fields
// (the lolschedule pattern) rather than an envelope or fallback.
type wrappedScalar struct {
	Date string `json:"date" bson:"date"`
}
type wrappedArray struct {
	Subscribers []int `json:"subscribers" bson:"subscribers"`
}

func TestMongoDocStore_WrappedScalarAndArray(t *testing.T) {
	db, cleanup := mongoLocalSetup(t)
	defer cleanup()
	ctx := context.Background()
	p := NewMongoProvider(db)

	scalar := Typed[wrappedScalar](p.Collection("lolschedule"))
	if err := scalar.Put(ctx, "last_push_date", wrappedScalar{Date: "2026-06-28"}); err != nil {
		t.Fatalf("scalar Put: %v", err)
	}
	doc := rawDoc(t, db.Collection("lolschedule"), "last_push_date")
	if doc["date"] != "2026-06-28" {
		t.Errorf("scalar root field date = %v", doc["date"])
	}

	arr := Typed[wrappedArray](p.Collection("lolschedule"))
	if err := arr.Put(ctx, "subscribers", wrappedArray{Subscribers: []int{1, 2, 3}}); err != nil {
		t.Fatalf("array Put: %v", err)
	}
	doc = rawDoc(t, db.Collection("lolschedule"), "subscribers")
	if _, ok := doc["subscribers"].(bson.A); !ok {
		t.Errorf("array root field subscribers = %T, want bson.A", doc["subscribers"])
	}
}
