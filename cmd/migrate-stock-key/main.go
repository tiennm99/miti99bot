// Command migrate-stock-key copies a module's DynamoDB partition to a new
// partition key. It exists for the one-time "trading" -> "stock" module
// rename: the framework derives the DynamoDB partition key (pk) from the
// module name, so renaming the module orphans every row written under the old
// pk. This tool re-homes those rows under the new pk.
//
// Behaviour:
//   - Reuses the runtime KVStore (List -> Get -> Put), so the stored JSON
//     bytes round-trip unchanged; only pk changes.
//   - Idempotent: a key already present in the destination partition is left
//     untouched, so a re-run never clobbers data the renamed module has
//     written since the first migration.
//   - Non-destructive: source rows are kept in place (free-tier cost is
//     negligible and they double as a rollback snapshot).
//
// Defaults to a dry run; pass -apply to perform writes.
//
// Usage:
//
//	DYNAMODB_TABLE=miti99bot-prod go run ./cmd/migrate-stock-key            # dry run
//	DYNAMODB_TABLE=miti99bot-prod go run ./cmd/migrate-stock-key -apply     # migrate
//	DYNAMODB_LOCAL_URL=http://localhost:8001 ... (point at DynamoDB Local)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/tiennm99/miti99bot/internal/storage"
)

func main() {
	from := flag.String("from", "trading", "source DynamoDB partition key (old module name)")
	to := flag.String("to", "stock", "destination DynamoDB partition key (new module name)")
	apply := flag.Bool("apply", false, "perform writes; without it the tool only reports what would change")
	flag.Parse()

	if err := run(context.Background(), *from, *to, *apply); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, from, to string, apply bool) error {
	table := os.Getenv("DYNAMODB_TABLE")
	if table == "" {
		return errors.New("DYNAMODB_TABLE is required")
	}
	if from == to {
		return fmt.Errorf("-from and -to must differ (both %q)", from)
	}

	client, err := storage.NewDynamoDBClient(ctx, storage.DynamoDBEndpointFromEnv())
	if err != nil {
		return err
	}
	provider := storage.NewDynamoDBProvider(client, table)
	mode := "DRY RUN"
	if apply {
		mode = "APPLY"
	}
	fmt.Printf("[%s] table=%s  %s -> %s\n", mode, table, from, to)
	return migrate(ctx, provider.For(from), provider.For(to), from, to, apply, os.Stdout)
}

// migrate copies every key from src to dst, preserving the stored bytes.
// Keys already present in dst are skipped (idempotent re-runs never clobber
// fresher destination state). Source entries are never deleted. When apply is
// false it reports the planned copies without writing. Decoupled from
// DynamoDB/env so it is unit-testable against any KVStore pair (the in-memory
// provider prefixes keys by module name exactly like DynamoDB partitions).
func migrate(ctx context.Context, src, dst storage.KVStore, from, to string, apply bool, out io.Writer) error {
	keys, err := src.List(ctx, "")
	if err != nil {
		return fmt.Errorf("list source partition %q: %w", from, err)
	}
	fmt.Fprintf(out, "  %d source keys\n", len(keys))

	var copied, skipped int
	for _, key := range keys {
		// Skip keys the destination already has — the renamed module may have
		// written fresher state; the migration must never overwrite it.
		if _, err := dst.Get(ctx, key); err == nil {
			fmt.Fprintf(out, "  skip  %s (already in %s)\n", key, to)
			skipped++
			continue
		} else if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("probe destination %s/%s: %w", to, key, err)
		}

		if !apply {
			fmt.Fprintf(out, "  copy  %s (would migrate)\n", key)
			copied++
			continue
		}

		raw, err := src.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("read source %s/%s: %w", from, key, err)
		}
		if err := dst.Put(ctx, key, raw); err != nil {
			return fmt.Errorf("write destination %s/%s: %w", to, key, err)
		}
		fmt.Fprintf(out, "  copy  %s (%d bytes)\n", key, len(raw))
		copied++
	}

	verb := "would copy"
	if apply {
		verb = "copied"
	}
	fmt.Fprintf(out, "done: %s %d, skipped %d (source rows left in place)\n", verb, copied, skipped)
	return nil
}
