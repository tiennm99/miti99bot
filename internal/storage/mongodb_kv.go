package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoDB document fields. `_id` holds the user key; `value` holds the raw
// bytes; `updatedAt` is unix-nanos for observability + future TTL.
const (
	mongoIDField        = "_id"
	mongoValueField     = "value"
	mongoUpdatedAtField = "updatedAt"
)

// MongoKVStore is a KVStore backed by a single MongoDB collection. The caller
// (MongoProvider) creates one per module so cross-module isolation is
// "different collection" — no key prefix needed at this layer, mirroring
// FirestoreKVStore.
type MongoKVStore struct {
	coll       *mongo.Collection
	moduleName string
}

// NewMongoKVStore returns a store writing to the given collection. Callers must
// validate the collection/module name beforehand (MongoProvider does).
func NewMongoKVStore(coll *mongo.Collection, moduleName string) *MongoKVStore {
	return &MongoKVStore{coll: coll, moduleName: moduleName}
}

// decodeValue extracts the stored value bytes from a decoded document. The
// driver decodes a BSON string into string and a BSON binary into bson.Binary;
// accept both so values written by any path round-trip. String is the current
// encoding (human-readable in the Atlas UI); the binary case is retained for
// backward compatibility with any documents written by an earlier build that
// stored value as BSON binary.
func (s *MongoKVStore) decodeValue(key string, doc bson.M) ([]byte, error) {
	raw, ok := doc[mongoValueField]
	if !ok {
		return nil, fmt.Errorf("mongo get %s/%s: missing %q field", s.moduleName, key, mongoValueField)
	}
	switch v := raw.(type) {
	case string:
		return []byte(v), nil
	case bson.Binary:
		return v.Data, nil
	case []byte:
		return v, nil
	default:
		return nil, fmt.Errorf("mongo get %s/%s: unexpected value type %T", s.moduleName, key, raw)
	}
}

// Get returns the raw bytes stored at key, or ErrNotFound.
func (s *MongoKVStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	var doc bson.M
	err := s.coll.FindOne(ctx, bson.M{mongoIDField: key}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("mongo get %s/%s: %w", s.moduleName, key, err)
	}
	return s.decodeValue(key, doc)
}

// GetJSON decodes the value at key into dst.
func (s *MongoKVStore) GetJSON(ctx context.Context, key string, dst any) error {
	raw, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("mongo get %s/%s: json decode: %w", s.moduleName, key, err)
	}
	return nil
}

// doc builds the persisted document for key/val with a fresh updatedAt stamp.
// value is stored as a BSON string so it is human-readable in the Atlas/Compass
// UI (mirrors DynamoDB's String storage, dynamodb_kv.go). Every current caller
// writes JSON, which is UTF-8 safe; non-UTF-8 callers must encode upstream
// (e.g. base64), same constraint as DynamoDB. updatedAt is int64 unix-nanos to
// match DynamoDB and keep migration faithful.
func (s *MongoKVStore) doc(key string, val []byte) bson.M {
	return bson.M{
		mongoIDField:        key,
		mongoValueField:     string(val),
		mongoUpdatedAtField: time.Now().UTC().UnixNano(),
	}
}

// Put writes raw bytes at key, creating or overwriting.
func (s *MongoKVStore) Put(ctx context.Context, key string, val []byte) error {
	if err := validateKey(key); err != nil {
		return err
	}
	_, err := s.coll.ReplaceOne(ctx,
		bson.M{mongoIDField: key},
		s.doc(key, val),
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("mongo put %s/%s: %w", s.moduleName, key, err)
	}
	return nil
}

// PutJSON marshals val and writes the bytes at key.
func (s *MongoKVStore) PutJSON(ctx context.Context, key string, val any) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("mongo put %s/%s: json encode: %w", s.moduleName, key, err)
	}
	return s.Put(ctx, key, raw)
}

// CompareAndSwap conditionally replaces the value only when it still equals
// expected. A nil expected means the key must not yet exist.
//
//   - expected == nil → InsertOne; the unique _id index makes the absent-insert
//     race linearizable (exactly one writer wins, losers get a duplicate-key
//     error → ErrConflict). This is a LIVE first-write path (every new
//     coin/gold portfolio), not an edge case.
//   - expected != nil → UpdateOne filtered on the matching value; MatchedCount
//     of 0 means the stored value changed (or the key is absent) → ErrConflict.
func (s *MongoKVStore) CompareAndSwap(ctx context.Context, key string, expected []byte, val []byte) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if expected == nil {
		_, err := s.coll.InsertOne(ctx, s.doc(key, val))
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return ErrConflict
			}
			return fmt.Errorf("mongo compare-and-swap %s/%s: %w", s.moduleName, key, err)
		}
		return nil
	}
	res, err := s.coll.UpdateOne(ctx,
		bson.M{
			mongoIDField:    key,
			mongoValueField: string(expected),
		},
		bson.M{"$set": bson.M{
			mongoValueField:     string(val),
			mongoUpdatedAtField: time.Now().UTC().UnixNano(),
		}},
	)
	if err != nil {
		return fmt.Errorf("mongo compare-and-swap %s/%s: %w", s.moduleName, key, err)
	}
	if res.MatchedCount == 0 {
		return ErrConflict
	}
	return nil
}

// Delete removes the document at key. Deleting a missing key is not an error
// (idempotent) — DeleteOne with a zero match count returns nil.
func (s *MongoKVStore) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	_, err := s.coll.DeleteOne(ctx, bson.M{mongoIDField: key})
	if err != nil {
		return fmt.Errorf("mongo delete %s/%s: %w", s.moduleName, key, err)
	}
	return nil
}

// List returns all document IDs in the collection that start with prefix.
// Implemented as a half-open range scan on _id (reusing prefixSuccessor) so it
// uses the _id index and avoids regex injection. Empty prefix returns the whole
// collection.
func (s *MongoKVStore) List(ctx context.Context, prefix string) ([]string, error) {
	if err := validatePrefix(prefix); err != nil {
		return nil, err
	}
	filter := bson.M{}
	if prefix != "" {
		filter[mongoIDField] = bson.M{
			"$gte": prefix,
			"$lt":  prefixSuccessor(prefix),
		}
	}
	cur, err := s.coll.Find(ctx, filter, options.Find().SetProjection(bson.M{mongoIDField: 1}))
	if err != nil {
		return nil, fmt.Errorf("mongo list %s prefix=%q: %w", s.moduleName, prefix, err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var keys []string
	for cur.Next(ctx) {
		var doc struct {
			ID string `bson:"_id"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("mongo list %s prefix=%q: decode: %w", s.moduleName, prefix, err)
		}
		keys = append(keys, doc.ID)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("mongo list %s prefix=%q: %w", s.moduleName, prefix, err)
	}
	return keys, nil
}
