package storage

import (
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoProvider is a KVProvider that creates one collection per module,
// mirroring FirestoreProvider. Collection-per-module IS the isolation — no key
// prefix wrapping is needed at this layer.
type MongoProvider struct {
	db *mongo.Database
}

// NewMongoProvider returns a provider over the given database handle. The
// underlying client must outlive every KVStore the provider hands out; callers
// own its Disconnect.
func NewMongoProvider(db *mongo.Database) *MongoProvider {
	return &MongoProvider{db: db}
}

// For returns a MongoKVStore writing to a collection named after the module.
// moduleName is re-validated against collectionNameRe — defense in depth
// against caller bugs that bypass modules.Build, identical to the firestore
// and dynamodb providers. An invalid name yields an invalidStore whose every
// op errors at first use.
func (p *MongoProvider) For(moduleName string) KVStore {
	if !collectionNameRe.MatchString(moduleName) {
		return invalidStore{name: moduleName}
	}
	return NewMongoKVStore(p.db.Collection(moduleName), moduleName)
}
