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
	mongoVersionField   = "version"
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

// decodeValue extracts the stored value bytes from a decoded document,
// reconstructing the caller's JSON []byte from whichever representation is on
// disk:
//   - native object/array (bson.M / bson.A / bson.D) → re-serialized to JSON;
//   - string → the bytes verbatim (bare scalars / non-JSON values, e.g. a date);
//   - bson.Binary / []byte → legacy fallback for docs written by an earlier
//     build that stored value as a binary blob.
func (s *MongoKVStore) decodeValue(key string, doc bson.M) ([]byte, error) {
	raw, ok := doc[mongoValueField]
	if !ok {
		return nil, fmt.Errorf("mongo get %s/%s: missing %q field", s.moduleName, key, mongoValueField)
	}
	switch v := raw.(type) {
	case string:
		return []byte(v), nil
	case bson.M, bson.A, bson.D:
		return nativeToJSON(v)
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

// Put writes raw bytes at key, creating or overwriting, and bumps the version
// (so a concurrent versioned writer that read the old version sees a conflict).
func (s *MongoKVStore) Put(ctx context.Context, key string, val []byte) error {
	if err := validateKey(key); err != nil {
		return err
	}
	_, err := s.coll.UpdateOne(ctx,
		bson.M{mongoIDField: key},
		bson.M{
			"$set": bson.M{
				mongoValueField:     encodeValue(val),
				mongoUpdatedAtField: time.Now().UTC().UnixNano(),
			},
			"$inc": bson.M{mongoVersionField: 1},
		},
		options.UpdateOne().SetUpsert(true),
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

// GetVersioned returns the value and its version, or ErrNotFound. A document
// written by an older build that has no version field reports version 0 (and no
// error), so PutVersioned(expectedVersion=0) can adopt it.
func (s *MongoKVStore) GetVersioned(ctx context.Context, key string) ([]byte, int64, error) {
	if err := validateKey(key); err != nil {
		return nil, 0, err
	}
	var doc bson.M
	err := s.coll.FindOne(ctx, bson.M{mongoIDField: key}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, fmt.Errorf("mongo get %s/%s: %w", s.moduleName, key, err)
	}
	val, err := s.decodeValue(key, doc)
	if err != nil {
		return nil, 0, err
	}
	return val, decodeVersion(doc), nil
}

// PutVersioned writes val only if the stored version equals expectedVersion,
// then bumps it. expectedVersion == 0 means "create, or adopt a key that has no
// version field yet" (a legacy doc). ErrConflict on a version mismatch.
//
//   - expectedVersion == 0 → upsert matching {_id} with version absent or 0; a
//     key that already has version ≥ 1 fails the filter, the upsert attempts an
//     insert on the same _id, and the unique-_id index returns duplicate-key →
//     ErrConflict (linearizable single-winner for the first write).
//   - expectedVersion > 0 → UpdateOne on {_id, version:expected}; MatchedCount
//     of 0 means the version moved → ErrConflict.
func (s *MongoKVStore) PutVersioned(ctx context.Context, key string, expectedVersion int64, val []byte) error {
	if err := validateKey(key); err != nil {
		return err
	}
	now := time.Now().UTC().UnixNano()
	if expectedVersion == 0 {
		_, err := s.coll.UpdateOne(ctx,
			bson.M{
				mongoIDField: key,
				"$or": bson.A{
					bson.M{mongoVersionField: bson.M{"$exists": false}},
					bson.M{mongoVersionField: int64(0)},
				},
			},
			bson.M{"$set": bson.M{
				mongoValueField:     encodeValue(val),
				mongoUpdatedAtField: now,
				mongoVersionField:   int64(1),
			}},
			options.UpdateOne().SetUpsert(true),
		)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return ErrConflict
			}
			return fmt.Errorf("mongo put-versioned %s/%s: %w", s.moduleName, key, err)
		}
		return nil
	}
	res, err := s.coll.UpdateOne(ctx,
		bson.M{mongoIDField: key, mongoVersionField: expectedVersion},
		bson.M{
			"$set": bson.M{mongoValueField: encodeValue(val), mongoUpdatedAtField: now},
			"$inc": bson.M{mongoVersionField: 1},
		},
	)
	if err != nil {
		return fmt.Errorf("mongo put-versioned %s/%s: %w", s.moduleName, key, err)
	}
	if res.MatchedCount == 0 {
		return ErrConflict
	}
	return nil
}

// decodeVersion reads the version field from a decoded doc, defaulting to 0 for
// legacy docs that predate the version field. The driver may decode a BSON int
// as int32 or int64.
func decodeVersion(doc bson.M) int64 {
	switch v := doc[mongoVersionField].(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
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
