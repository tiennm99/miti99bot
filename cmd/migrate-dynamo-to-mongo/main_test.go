package main

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/tiennm99/miti99bot/internal/storage"
)

func TestDecodeItem(t *testing.T) {
	raw := map[string]types.AttributeValue{
		"pk":    &types.AttributeValueMemberS{Value: "coin"},
		"sk":    &types.AttributeValueMemberS{Value: "user:1"},
		"value": &types.AttributeValueMemberS{Value: `{"x":1}`},
	}
	it, err := decodeItem(raw)
	if err != nil {
		t.Fatalf("decodeItem: %v", err)
	}
	if it.pk != "coin" || it.sk != "user:1" || string(it.value) != `{"x":1}` {
		t.Errorf("decoded %+v", it)
	}

	// Missing value attribute is a hard error.
	if _, err := decodeItem(map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "coin"},
		"sk": &types.AttributeValueMemberS{Value: "user:1"},
	}); err == nil {
		t.Error("decodeItem with missing value: want error, got nil")
	}
}

func TestSortedKeys(t *testing.T) {
	got := sortedKeys(map[string]int{"b": 1, "a": 2, "c": 3})
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("sortedKeys = %v", got)
	}
}

// TestMigrateAndVerify is the end-to-end gate: seed DynamoDB Local across two
// modules, migrate into Mongo, assert values round-trip byte-identically, and
// confirm --verify reports matching counts. Skips unless BOTH emulators are
// configured.
func TestMigrateAndVerify(t *testing.T) {
	ddbURL := os.Getenv("DYNAMODB_LOCAL_URL")
	mongoURL := os.Getenv("MONGODB_TEST_URL")
	mongoDB := os.Getenv("MONGO_DATABASE")
	if ddbURL == "" || mongoURL == "" || mongoDB == "" {
		t.Skip("set DYNAMODB_LOCAL_URL, MONGODB_TEST_URL, MONGO_DATABASE to run the migrator e2e test")
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "ap-southeast-1")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ddb, err := storage.NewDynamoDBClient(ctx, ddbURL)
	if err != nil {
		t.Fatalf("dynamodb client: %v", err)
	}
	table := "migrate-test"
	createTable(t, ctx, ddb, table)
	defer func() {
		_, _ = ddb.DeleteTable(ctx, &dynamodb.DeleteTableInput{TableName: aws.String(table)})
	}()

	seed := []item{
		{pk: "coin", sk: "user:1", value: []byte(`{"bal":100}`)},
		{pk: "coin", sk: "user:2", value: []byte(`{"bal":200}`)},
		{pk: "stock", sk: "user:1", value: []byte(`{"vnd":5000}`)},
	}
	for _, it := range seed {
		putDynamoItem(t, ctx, ddb, table, it)
	}

	// Migrate.
	if err := runMigrate(ctx, ddb, mongoDatabase(t, ctx, mongoURL, mongoDB), table, false); err != nil {
		t.Fatalf("runMigrate: %v", err)
	}
	// Re-run must stay idempotent.
	if err := runMigrate(ctx, ddb, mongoDatabase(t, ctx, mongoURL, mongoDB), table, false); err != nil {
		t.Fatalf("runMigrate re-run: %v", err)
	}
	// Verify passes.
	if err := runVerify(ctx, ddb, mongoDatabase(t, ctx, mongoURL, mongoDB), table); err != nil {
		t.Fatalf("runVerify: %v", err)
	}

	// Spot-check the migrated value round-trips through the typed store as a
	// flattened native doc: bal hoisted to the root, preserved as int64.
	db := mongoDatabase(t, ctx, mongoURL, mongoDB)
	provider := storage.NewMongoProvider(db)
	got, _, err := storage.Typed[bson.M](provider.Collection("coin")).Get(ctx, "user:1")
	if err != nil {
		t.Fatalf("Get migrated value: %v", err)
	}
	if got["bal"] != int64(100) {
		t.Errorf("migrated value bal = %v (%T), want int64(100)", got["bal"], got["bal"])
	}
}

func mongoDatabase(t *testing.T, ctx context.Context, uri, db string) *mongo.Database {
	t.Helper()
	client, err := storage.NewMongoClient(ctx, uri)
	if err != nil {
		t.Fatalf("NewMongoClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	mdb, err := storage.NewMongoDatabase(client, db)
	if err != nil {
		t.Fatalf("NewMongoDatabase: %v", err)
	}
	return mdb
}

func createTable(t *testing.T, ctx context.Context, c *dynamodb.Client, table string) {
	t.Helper()
	_, err := c.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(table),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
		},
	})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
}

func putDynamoItem(t *testing.T, ctx context.Context, c *dynamodb.Client, table string, it item) {
	t.Helper()
	_, err := c.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item: map[string]types.AttributeValue{
			"pk":    &types.AttributeValueMemberS{Value: it.pk},
			"sk":    &types.AttributeValueMemberS{Value: it.sk},
			"value": &types.AttributeValueMemberS{Value: string(it.value)},
		},
	})
	if err != nil {
		t.Fatalf("PutItem: %v", err)
	}
}
