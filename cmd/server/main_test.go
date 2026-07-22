package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/stock"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

func TestResolveCommitSHA(t *testing.T) {
	prev := gitSHA
	defer func() { gitSHA = prev }()

	// Coolify runtime env wins over the baked fallback.
	gitSHA = "baked"
	if got := resolveCommitSHA("  runtime-sha "); got != "runtime-sha" {
		t.Errorf("env present: got %q, want trimmed runtime-sha", got)
	}

	// Empty env falls back to the VCS revision embedded by go build.
	if got := resolveCommitSHA(""); got != "baked" {
		t.Errorf("env empty: got %q, want baked fallback", got)
	}

	// Neither source set → "unknown" so the owner still gets the startup DM.
	gitSHA = ""
	if got := resolveCommitSHA("   "); got != "unknown" {
		t.Errorf("both empty: got %q, want unknown", got)
	}
}

func TestInitStockStoreRunsStartupMigration(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	if err := initStockStore(ctx, provider); err != nil {
		t.Fatalf("initStockStore: %v", err)
	}
	marker, exists, err := systemstate.New(provider.Collection(systemstate.CollectionName)).Get(ctx, "migration:stock-dividend-history-v1")
	if err != nil || !exists || marker.Status != "completed" {
		t.Fatalf("marker=%+v exists=%v err=%v", marker, exists, err)
	}
	if _, _, err := storage.Typed[stock.Portfolio](provider.Collection(stock.CollectionName)).Get(ctx, "user:1"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("unexpected portfolio lookup error: %v", err)
	}
}

func TestInitStockStorePropagatesMigrationError(t *testing.T) {
	want := errors.New("migration failed")
	err := initStockStoreWith(context.Background(), storage.NewMemoryProvider(), func(context.Context, storage.Collection, storage.Collection) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("initStockStoreWith error=%v, want %v", err, want)
	}
}

func TestShortCommitSHA(t *testing.T) {
	if got := shortCommitSHA(" 0123456789abcdef "); got != "0123456" {
		t.Errorf("full revision: got %q, want %q", got, "0123456")
	}
	if got := shortCommitSHA("abc123"); got != "abc123" {
		t.Errorf("short revision: got %q, want %q", got, "abc123")
	}
}

func TestComposeDoesNotOverrideSourceCommit(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "compose.yml"))
	if err != nil {
		t.Fatalf("read compose.yml: %v", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "SOURCE_COMMIT:") || strings.HasPrefix(trimmed, "- SOURCE_COMMIT") {
			t.Fatalf("compose.yml must not declare SOURCE_COMMIT; Coolify supplies it at runtime and an explicit Compose value can override it with empty")
		}
	}
}

func TestFactoriesIncludesExpectedModules(t *testing.T) {
	catalog := factories()
	for _, name := range []string{"gold", "coin"} {
		if catalog[name] == nil {
			t.Fatalf("factories missing %s", name)
		}
	}
	reg, err := modules.Build([]string{"gold", "coin"}, catalog, storage.NewMemoryProvider(), modules.BuildOptions{})
	if err != nil {
		t.Fatalf("Build selected modules: %v", err)
	}
	for _, name := range []string{
		"gold_price", "gold_topup", "gold_buy", "gold_sell", "gold_portfolio",
		"coin_price", "coin_topup", "coin_buy", "coin_sell", "coin_portfolio",
	} {
		if _, ok := reg.AllCommands[name]; !ok {
			t.Fatalf("missing command %s", name)
		}
	}
}
