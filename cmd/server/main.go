package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tiennm99/miti99bot/internal/cron"
	"github.com/tiennm99/miti99bot/internal/deploynotify"
	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/metrics"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/coin"
	"github.com/tiennm99/miti99bot/internal/modules/gold"
	"github.com/tiennm99/miti99bot/internal/modules/lol"
	"github.com/tiennm99/miti99bot/internal/modules/loldle"
	"github.com/tiennm99/miti99bot/internal/modules/misc"
	"github.com/tiennm99/miti99bot/internal/modules/stats"
	"github.com/tiennm99/miti99bot/internal/modules/stock"
	"github.com/tiennm99/miti99bot/internal/modules/util"
	"github.com/tiennm99/miti99bot/internal/modules/wc"
	"github.com/tiennm99/miti99bot/internal/modules/wordle"
	"github.com/tiennm99/miti99bot/internal/server"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/telegram"
)

// gitSHA is the local-build fallback, populated via `-ldflags "-X
// main.gitSHA=<sha>"` (see Makefile). On Coolify the commit comes from the
// SOURCE_COMMIT runtime env instead (resolveCommitSHA prefers it). Empty from
// both sources means deploynotify stays silent.
var gitSHA string

// resolveCommitSHA returns the commit identifier for the deploy notification.
// Coolify injects SOURCE_COMMIT into the container environment at runtime, so
// prefer it; fall back to the ldflags-baked gitSHA for local builds; and when
// neither is set, report "unknown" so the owner still gets the startup DM.
func resolveCommitSHA(envSourceCommit string) string {
	if s := strings.TrimSpace(envSourceCommit); s != "" {
		return s
	}
	if gitSHA != "" {
		return gitSHA
	}
	return "unknown"
}

// factories is the static module catalog. Adding a new module is a one-line
// change here. Lives in main rather than the modules package to avoid an
// import cycle (modules → util → modules).
func factories() map[string]modules.Factory {
	return map[string]modules.Factory{
		"util":             util.New,
		"misc":             misc.New,
		"wordle":           wordle.New,
		"loldle":           loldle.New,
		lol.CollectionName: lol.New,
		"wc":               wc.New,
		"coin":             coin.New,
		"gold":             gold.New,
		"stock":            stock.New,
		"stats":            stats.New,
	}
}

// mongodbInitTimeout caps MongoDB connect+ping at startup (and Disconnect at
// shutdown). Atlas SRV DNS + TLS handshake can take a couple seconds on a cold
// container; 10s leaves headroom without hiding a wedged cluster.
const mongodbInitTimeout = 10 * time.Second

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := loadConfig()
	if cfg.TelegramBotToken == "" {
		log.Fatal("missing required env", "key", "TELEGRAM_BOT_TOKEN")
	}

	// Periodic metrics flush. Cancels with rootCtx and emits one final
	// flush on shutdown so the trailing window isn't lost.
	go metrics.Run(rootCtx)

	provider, closeProvider, err := buildProvider(rootCtx, cfg)
	if err != nil {
		log.Fatal("storage init failed", "err", err)
	}
	defer closeProvider()

	if err := stats.InitStore(rootCtx, provider.Collection("stats")); err != nil {
		log.Fatal("stats storage init failed", "err", err)
	}
	if err := lol.InitStore(rootCtx, provider.Collection(lol.CollectionName)); err != nil {
		log.Fatal("lol storage init failed", "err", err)
	}

	b, err := telegram.NewBot(cfg.TelegramBotToken)
	if err != nil {
		log.Fatal("telegram bot init failed", "err", err)
	}

	reg, err := modules.Build(cfg.Modules, factories(), provider, modules.BuildOptions{
		Bot: b,
	})
	if err != nil {
		log.Fatal("module registry build failed", "err", err)
	}
	auth := modules.Auth{BotOwnerID: cfg.BotOwnerID, AdminUserIDs: cfg.AdminUserIDs}
	modules.Install(b, reg, auth)
	log.Info("modules loaded",
		"modules", len(reg.Modules),
		"commands", len(reg.AllCommands),
		"crons", len(reg.Crons()))
	if n, err := registerCommandMenu(rootCtx, b, reg); err != nil {
		log.Warn("telegram command menu registration failed", "commands", n, "err", err)
	} else {
		log.Info("telegram command menu registered", "commands", n)
	}

	// In-process cron scheduler runs unconditionally so the long-lived container
	// fires module crons (e.g. lol daily push) on their Schedule.
	stopCron, err := cron.Run(rootCtx, reg)
	if err != nil {
		log.Fatal("cron scheduler init failed", "err", err)
	}
	defer stopCron()

	if cfg.BotOwnerID == 0 {
		log.Warn("OWNER_ID unset; all Private + Protected commands will be denied")
	}

	// Clear any existing webhook at startup before the owner DM and before
	// polling. getUpdates returns HTTP 409 while a webhook is set, so a stuck
	// webhook silently breaks the bot. Best-effort, one shot: a real failure
	// here is logged, not retried.
	if err := telegram.DeleteWebhook(rootCtx, cfg.TelegramBotToken); err != nil {
		log.Warn("deleteWebhook failed; getUpdates may 409 if a webhook is set", "err", err)
	} else {
		log.Info("webhook cleared")
	}

	deploynotify.Run(rootCtx, deploynotify.Config{
		Bot:     b,
		OwnerID: cfg.BotOwnerID,
		GitSHA:  resolveCommitSHA(cfg.SourceCommit),
	})

	handler := server.New()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// The only route is GET / (health). It responds instantly, so a tight
		// write deadline is ample and bounds any slow-loris write.
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info("server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server crashed", "err", err)
		}
	}()

	// Long polling is the sole Telegram transport (no webhook, no public
	// ingress). Telegram permits exactly one getUpdates consumer per bot token,
	// so deploy exactly one replica. The webhook was cleared at startup above.
	go func() {
		log.Info("telegram long polling started")
		b.Start(rootCtx) // returns when rootCtx is cancelled
		log.Info("telegram long polling stopped")
	}()

	<-rootCtx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
	}
}

// buildProvider picks the storage backend. Selection order:
//  1. Explicit KV_PROVIDER env (memory|mongodb) wins.
//  2. Auto-detect: MONGO_URL set → mongodb; otherwise memory.
//
// The self-host default is mongodb (just set MONGO_URL + MONGO_DATABASE — no
// KV_PROVIDER needed). The memory backend is for tests and local no-database
// runs (MODULES=).
//
// Returned closer is always non-nil and safe to call exactly once.
func buildProvider(ctx context.Context, cfg config) (storage.Provider, func(), error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.KVProvider))
	if backend == "" {
		if cfg.MongoURL != "" {
			backend = "mongodb"
		} else {
			backend = "memory"
		}
	}

	switch backend {
	case "memory":
		log.Warn("KV backend: in-memory (data lost on restart)")
		return storage.NewMemoryProvider(), func() {}, nil

	case "mongodb":
		if cfg.MongoURL == "" || cfg.MongoDatabase == "" {
			return nil, func() {}, errors.New("KV_PROVIDER=mongodb requires MONGO_URL and MONGO_DATABASE")
		}
		initCtx, cancel := context.WithTimeout(ctx, mongodbInitTimeout)
		defer cancel()
		client, err := storage.NewMongoClient(initCtx, cfg.MongoURL)
		if err != nil {
			return nil, func() {}, err
		}
		db, err := storage.NewMongoDatabase(client, cfg.MongoDatabase)
		if err != nil {
			_ = client.Disconnect(context.Background())
			return nil, func() {}, err
		}
		closer := func() {
			discCtx, cancel := context.WithTimeout(context.Background(), mongodbInitTimeout)
			defer cancel()
			if err := client.Disconnect(discCtx); err != nil {
				log.Error("mongo disconnect failed", "err", err)
			}
		}
		// NEVER log MONGO_URL — it is mongodb+srv://user:pass@host and is a
		// credential. Log only the (non-secret) database name.
		log.Info("storage backend", "backend", "mongodb", "database", cfg.MongoDatabase)
		return storage.NewMongoProvider(db), closer, nil

	default:
		return nil, func() {}, fmt.Errorf("unknown KV_PROVIDER %q (want memory|mongodb)", backend)
	}
}

type config struct {
	Port             string
	TelegramBotToken string
	SourceCommit     string // Coolify-injected commit SHA (runtime env) for deploynotify
	Modules          []string
	BotOwnerID       int64
	AdminUserIDs     map[int64]bool
	KVProvider       string // empty = auto-detect; or "memory"|"mongodb"
	MongoURL         string // required when KVProvider=mongodb (Atlas SRV connection string; SECRET — never log)
	MongoDatabase    string // required when KVProvider=mongodb
}

func loadConfig() config {
	envMap := make(map[string]string, len(os.Environ()))
	for _, kv := range os.Environ() {
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			envMap[kv[:eq]] = kv[eq+1:]
		}
	}
	port := envMap["PORT"]
	if port == "" {
		port = "8080"
	}
	// PORT must be numeric — http.Server constructs ":<port>" verbatim, so a
	// junk value would surface only at ListenAndServe time. Fail fast here
	// instead. Range check is delegated to http.Server (it handles 0/65535).
	if n, err := strconv.Atoi(port); err != nil || n < 0 || n > 65535 {
		log.Fatal("invalid PORT", "value", port)
	}
	return config{
		Port:             port,
		TelegramBotToken: envMap["TELEGRAM_BOT_TOKEN"],
		SourceCommit:     envMap["SOURCE_COMMIT"],
		Modules:          splitCSV(envMap["MODULES"]),
		BotOwnerID:       parseInt64(envMap["OWNER_ID"]),
		AdminUserIDs:     parseInt64Set(envMap["ADMIN_IDS"]),
		KVProvider:       envMap["KV_PROVIDER"],
		MongoURL:         envMap["MONGO_URL"],
		MongoDatabase:    envMap["MONGO_DATABASE"],
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// parseInt64 returns 0 (the "unset" sentinel) when s is empty or invalid.
// Telegram user IDs are positive int64 so 0 is unambiguously "no value".
func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		log.Warn("invalid int64 in env", "value", s, "err", err)
		return 0
	}
	return n
}

// parseInt64Set parses a comma-separated list of int64 IDs into a set. Bad
// entries are logged and skipped — one malformed admin ID does not deny the
// rest.
func parseInt64Set(s string) map[int64]bool {
	if s == "" {
		return nil
	}
	out := map[int64]bool{}
	for _, p := range strings.Split(s, ",") {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			log.Warn("invalid admin id", "value", t, "err", err)
			continue
		}
		out[n] = true
	}
	return out
}
