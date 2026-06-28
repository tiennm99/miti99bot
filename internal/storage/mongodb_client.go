package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// mongoServerSelectionTimeout bounds how long a single operation waits for a
// reachable server before failing. The self-hosted container holds one
// long-lived client for days against Atlas M0 (which idles / fails over); a
// tight selection timeout means a wedged DB surfaces as a fast error on the
// next op rather than an unbounded hang. The driver auto-reconnects on the
// following operation once the server is back.
const mongoServerSelectionTimeout = 5 * time.Second

// NewMongoClient connects to MongoDB using the full connection URI (including
// credentials for Atlas `mongodb+srv://` strings) and verifies reachability
// with a Ping under the caller's context deadline.
//
// The client is goroutine-safe and meant to be reused for the lifetime of the
// process; callers must defer Disconnect at shutdown.
//
// SECURITY: uri carries the username/password for Atlas. Callers MUST NOT log
// it — see buildProvider, which logs only the database name.
func NewMongoClient(ctx context.Context, uri string) (*mongo.Client, error) {
	if uri == "" {
		return nil, fmt.Errorf("storage: MONGO_URL is required for MongoDB")
	}
	opts := options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(mongoServerSelectionTimeout)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("storage: mongo.Connect: %w", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		// Best-effort cleanup; the connection never became usable.
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("storage: mongo ping: %w", err)
	}
	return client, nil
}

// NewMongoDatabase returns the named database handle from a connected client.
// The database is created lazily on first write by MongoDB; no round trip here.
func NewMongoDatabase(client *mongo.Client, database string) (*mongo.Database, error) {
	if database == "" {
		return nil, fmt.Errorf("storage: MONGO_DATABASE is required for MongoDB")
	}
	return client.Database(database), nil
}
