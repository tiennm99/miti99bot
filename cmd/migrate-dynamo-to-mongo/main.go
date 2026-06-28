// Command migrate-dynamo-to-mongo copies every item from the prod DynamoDB KV
// table into MongoDB Atlas using the same document schema the live app writes,
// then verifies per-module parity. It is idempotent (re-runs overwrite by key,
// never duplicate) and read-only on DynamoDB (Scan only).
//
// Usage:
//
//	migrate-dynamo-to-mongo [--dynamodb-table miti99bot-data] [--dry-run] [--verify]
//
// Required env: MONGO_URL, MONGO_DATABASE. DynamoDB credentials come from the
// AWS default chain — use a DEDICATED READ-ONLY profile whose policy grants
// exactly `dynamodb:Scan` on the table ARN (nothing else, no write actions).
// For local testing, set DYNAMODB_LOCAL_URL to point at DynamoDB Local.
//
//   - default: scan + write every item through MongoKVStore.Put (same value
//     encoding as the live app — JSON objects stored as native BSON), then
//     report per-module counts.
//   - --dry-run: scan + report counts, write nothing.
//   - --verify: tally DynamoDB per pk via Scan vs Mongo CountDocuments per
//     collection; print a table and exit non-zero on any mismatch.
//
// Note: writing through Put gives each doc the app's native-BSON value encoding
// and a fresh updatedAt/version; nothing in the app reads updatedAt (write-only
// today), and --verify compares counts, so this is intentional and harmless.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/tiennm99/miti99bot/internal/storage"
)

const defaultTable = "miti99bot-data"

func main() {
	table := flag.String("dynamodb-table", defaultTable, "source DynamoDB table name")
	dryRun := flag.Bool("dry-run", false, "scan and report counts without writing")
	verify := flag.Bool("verify", false, "compare per-module counts DynamoDB vs Mongo and exit non-zero on mismatch")
	flag.Parse()

	if err := run(*table, *dryRun, *verify); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(table string, dryRun, verify bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	mongoURL := os.Getenv("MONGO_URL")
	mongoDB := os.Getenv("MONGO_DATABASE")
	if mongoURL == "" || mongoDB == "" {
		return fmt.Errorf("MONGO_URL and MONGO_DATABASE are required")
	}

	ddb, err := storage.NewDynamoDBClient(ctx, storage.DynamoDBEndpointFromEnv())
	if err != nil {
		return fmt.Errorf("dynamodb client: %w", err)
	}

	mclient, err := storage.NewMongoClient(ctx, mongoURL)
	if err != nil {
		return fmt.Errorf("mongo client: %w", err)
	}
	defer func() { _ = mclient.Disconnect(context.Background()) }()
	mdb, err := storage.NewMongoDatabase(mclient, mongoDB)
	if err != nil {
		return err
	}

	if verify {
		return runVerify(ctx, ddb, mdb, table)
	}
	return runMigrate(ctx, ddb, mdb, table, dryRun)
}

// item is one decoded DynamoDB KV row.
type item struct {
	pk    string // module name → collection
	sk    string // user key → _id
	value []byte // raw value bytes
}

// scanTable reads every row from the table via a full Scan (the KV table is
// small) and decodes pk/sk/value. Scan is the ONLY DynamoDB action used, so the
// runner needs just `dynamodb:Scan` on the table ARN.
func scanTable(ctx context.Context, ddb *dynamodb.Client, table string) ([]item, error) {
	var items []item
	pager := dynamodb.NewScanPaginator(ddb, &dynamodb.ScanInput{TableName: aws.String(table)})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", table, err)
		}
		for _, raw := range page.Items {
			it, err := decodeItem(raw)
			if err != nil {
				return nil, err
			}
			items = append(items, it)
		}
	}
	return items, nil
}

func decodeItem(raw map[string]types.AttributeValue) (item, error) {
	pk, ok := raw["pk"].(*types.AttributeValueMemberS)
	if !ok {
		return item{}, fmt.Errorf("item missing string pk: %v", raw)
	}
	sk, ok := raw["sk"].(*types.AttributeValueMemberS)
	if !ok {
		return item{}, fmt.Errorf("item %s missing string sk", pk.Value)
	}
	val, ok := raw["value"].(*types.AttributeValueMemberS)
	if !ok {
		return item{}, fmt.Errorf("item %s/%s missing string value", pk.Value, sk.Value)
	}
	return item{pk: pk.Value, sk: sk.Value, value: []byte(val.Value)}, nil
}

// sortedKeys returns the map keys sorted for stable report output.
func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// runMigrate scans the table, then (unless dry-run) writes every item through
// MongoKVStore.Put so the value uses the same native-BSON encoding the live app
// writes — guaranteeing idempotent re-runs and correct version-based CAS. Put
// also runs validateKey on each sk and upserts by _id, so a re-run produces no
// duplicates and an invalid key fails loud rather than writing data
// the app's read path cannot load.
func runMigrate(ctx context.Context, ddb *dynamodb.Client, mdb *mongo.Database, table string, dryRun bool) error {
	items, err := scanTable(ctx, ddb, table)
	if err != nil {
		return err
	}

	counts := map[string]int{}
	provider := storage.NewMongoProvider(mdb)
	for _, it := range items {
		counts[it.pk]++
		if dryRun {
			continue
		}
		// For invalid module names For returns a store that errors on Put;
		// Put validates the key. Either way an error aborts loudly.
		if err := provider.For(it.pk).Put(ctx, it.sk, it.value); err != nil {
			return fmt.Errorf("put %s/%s: %w", it.pk, it.sk, err)
		}
	}

	mode := "MIGRATED"
	if dryRun {
		mode = "DRY-RUN (no writes)"
	}
	fmt.Printf("%s — %d items across %d modules\n", mode, len(items), len(counts))
	for _, pk := range sortedKeys(counts) {
		fmt.Printf("  %-20s %d\n", pk, counts[pk])
	}
	return nil
}

// runVerify tallies DynamoDB per pk via a Scan (so only dynamodb:Scan is
// needed — never Query) and compares against Mongo CountDocuments per
// collection. Prints a table and returns an error on any mismatch so the
// process exits non-zero.
func runVerify(ctx context.Context, ddb *dynamodb.Client, mdb *mongo.Database, table string) error {
	items, err := scanTable(ctx, ddb, table)
	if err != nil {
		return err
	}
	ddbCounts := map[string]int{}
	for _, it := range items {
		ddbCounts[it.pk]++
	}

	mongoCounts := map[string]int{}
	for pk := range ddbCounts {
		n, err := mdb.Collection(pk).CountDocuments(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("count mongo collection %s: %w", pk, err)
		}
		mongoCounts[pk] = int(n)
	}

	fmt.Printf("%-20s %10s %10s %s\n", "MODULE", "DYNAMODB", "MONGO", "STATUS")
	mismatch := false
	for _, pk := range sortedKeys(ddbCounts) {
		status := "OK"
		if ddbCounts[pk] != mongoCounts[pk] {
			status = "MISMATCH"
			mismatch = true
		}
		fmt.Printf("%-20s %10d %10d %s\n", pk, ddbCounts[pk], mongoCounts[pk], status)
	}
	if mismatch {
		return fmt.Errorf("verification failed: per-module counts differ")
	}
	fmt.Println("verification OK: all per-module counts match")
	return nil
}
