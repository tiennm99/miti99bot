package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Reserved document-root field names owned by the store. The caller's payload
// fields are hoisted to the root alongside these. `_id` holds the key.
const (
	mongoIDField        = "_id"
	mongoVersionField   = "version"
	mongoUpdatedAtField = "updatedAt"
)

// mongoPutAttempts bounds the get→put-versioned retry loop a plain Put uses to
// overwrite unconditionally while still bumping the version.
const mongoPutAttempts = 5

// mongoCollection is a Collection backed by a single MongoDB collection (one per
// module — collection-per-module IS the isolation, so no key prefix is needed).
type mongoCollection struct {
	coll   *mongo.Collection
	module string
}

func (mongoCollection) isCollection() {}

// storedDoc is the on-disk document shape. The payload's fields are hoisted to
// the document root via bson inline, so the value renders as an expandable,
// queryable native document in Compass — never wrapped in a `value` envelope.
// Reserved metadata (version, updatedAt) sit alongside the payload fields.
type storedDoc[T any] struct {
	ID        string    `bson:"_id"`
	Version   int64     `bson:"version"`
	UpdatedAt time.Time `bson:"updatedAt"`
	Payload   T         `bson:",inline"`
}

// mongoDocStore is a typed DocStore over one collection.
type mongoDocStore[T any] struct {
	coll   *mongo.Collection
	module string
}

func (s *mongoDocStore[T]) Get(ctx context.Context, id string) (T, int64, error) {
	var out storedDoc[T]
	if err := validateKey(id); err != nil {
		return out.Payload, 0, err
	}
	err := s.coll.FindOne(ctx, bson.M{mongoIDField: id}).Decode(&out)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return out.Payload, 0, ErrNotFound
		}
		return out.Payload, 0, fmt.Errorf("mongo get %s/%s: %w", s.module, id, err)
	}
	return out.Payload, out.Version, nil
}

// replacement builds the whole document to store. Using a full replacement
// (never $set) guarantees stale fields from a previous value cannot survive an
// overwrite.
func (s *mongoDocStore[T]) replacement(id string, version int64, val T) storedDoc[T] {
	return storedDoc[T]{ID: id, Version: version, UpdatedAt: time.Now().UTC(), Payload: val}
}

// PutVersioned writes val only if the stored version equals expectedVersion,
// then bumps it.
//
//   - expectedVersion == 0 → upsert matching {_id} with version absent or 0; a
//     key that already has version ≥ 1 fails the filter, the upsert attempts an
//     insert on the same _id, and the unique-_id index returns duplicate-key →
//     ErrConflict (linearizable single-winner for the first write).
//   - expectedVersion > 0 → ReplaceOne on {_id, version:expected}; MatchedCount
//     of 0 means the version moved → ErrConflict.
func (s *mongoDocStore[T]) PutVersioned(ctx context.Context, id string, expectedVersion int64, val T) error {
	if err := validateKey(id); err != nil {
		return err
	}
	if expectedVersion == 0 {
		_, err := s.coll.ReplaceOne(ctx,
			bson.M{
				mongoIDField: id,
				"$or": bson.A{
					bson.M{mongoVersionField: bson.M{"$exists": false}},
					bson.M{mongoVersionField: int64(0)},
				},
			},
			s.replacement(id, 1, val),
			options.Replace().SetUpsert(true),
		)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return ErrConflict
			}
			return fmt.Errorf("mongo put-versioned %s/%s: %w", s.module, id, err)
		}
		return nil
	}
	res, err := s.coll.ReplaceOne(ctx,
		bson.M{mongoIDField: id, mongoVersionField: expectedVersion},
		s.replacement(id, expectedVersion+1, val),
	)
	if err != nil {
		return fmt.Errorf("mongo put-versioned %s/%s: %w", s.module, id, err)
	}
	if res.MatchedCount == 0 {
		return ErrConflict
	}
	return nil
}

// Put overwrites unconditionally and bumps the version, implemented as a bounded
// get→put-versioned loop so it shares the whole-document replacement (stale-field
// removal) and version semantics with PutVersioned.
func (s *mongoDocStore[T]) Put(ctx context.Context, id string, val T) error {
	if err := validateKey(id); err != nil {
		return err
	}
	for attempt := 0; attempt < mongoPutAttempts; attempt++ {
		_, version, err := s.Get(ctx, id)
		switch {
		case errors.Is(err, ErrNotFound):
			version = 0
		case err != nil:
			return err
		}
		err = s.PutVersioned(ctx, id, version, val)
		if errors.Is(err, ErrConflict) {
			continue // a concurrent writer moved the version; re-read and retry.
		}
		return err
	}
	return fmt.Errorf("mongo put %s/%s: %w after %d attempts", s.module, id, ErrConflict, mongoPutAttempts)
}

func (s *mongoDocStore[T]) Delete(ctx context.Context, id string) error {
	if err := validateKey(id); err != nil {
		return err
	}
	if _, err := s.coll.DeleteOne(ctx, bson.M{mongoIDField: id}); err != nil {
		return fmt.Errorf("mongo delete %s/%s: %w", s.module, id, err)
	}
	return nil
}

// List returns all document IDs whose key starts with prefix, via a half-open
// range scan on _id (reusing prefixSuccessor) so it uses the _id index and
// avoids regex injection. Empty prefix returns the whole collection.
func (s *mongoDocStore[T]) List(ctx context.Context, prefix string) ([]string, error) {
	if err := validatePrefix(prefix); err != nil {
		return nil, err
	}
	filter := bson.M{}
	if prefix != "" {
		filter[mongoIDField] = bson.M{"$gte": prefix, "$lt": prefixSuccessor(prefix)}
	}
	cur, err := s.coll.Find(ctx, filter, options.Find().SetProjection(bson.M{mongoIDField: 1}))
	if err != nil {
		return nil, fmt.Errorf("mongo list %s prefix=%q: %w", s.module, prefix, err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var keys []string
	for cur.Next(ctx) {
		var doc struct {
			ID string `bson:"_id"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("mongo list %s prefix=%q: decode: %w", s.module, prefix, err)
		}
		keys = append(keys, doc.ID)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("mongo list %s prefix=%q: %w", s.module, prefix, err)
	}
	return keys, nil
}
