package storage

import (
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoProvider is a Provider that creates one collection per module.
// Collection-per-module IS the isolation — no key prefix wrapping is needed.
type MongoProvider struct {
	db *mongo.Database
}

// NewMongoProvider returns a provider over the given database handle. The
// underlying client must outlive every store the provider hands out; callers
// own its Disconnect.
func NewMongoProvider(db *mongo.Database) *MongoProvider {
	return &MongoProvider{db: db}
}

// Collection returns a handle to the collection named after the module.
// moduleName is re-validated against collectionNameRe — defense in depth against
// caller bugs that bypass modules.Build. An invalid name yields an
// invalidCollection whose Typed store errors at first use.
func (p *MongoProvider) Collection(moduleName string) Collection {
	if !collectionNameRe.MatchString(moduleName) {
		return invalidCollection{name: moduleName}
	}
	return mongoCollection{coll: p.db.Collection(moduleName), module: moduleName}
}

// MongoCollection returns the native MongoDB collection behind c when the
// active backend is MongoDB. Modules with query-shaped data can opt into Mongo
// operators while still keeping the memory backend for tests/local runs.
func MongoCollection(c Collection) (*mongo.Collection, bool) {
	h, ok := c.(mongoCollection)
	if !ok {
		return nil, false
	}
	return h.coll, true
}
