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

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/tiennm99/miti99bot/internal/ai"
	"github.com/tiennm99/miti99bot/internal/cron"
	"github.com/tiennm99/miti99bot/internal/deploynotify"
	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/metrics"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/coin"
	"github.com/tiennm99/miti99bot/internal/modules/gold"
	"github.com/tiennm99/miti99bot/internal/modules/loldle"
	"github.com/tiennm99/miti99bot/internal/modules/lolschedule"
	"github.com/tiennm99/miti99bot/internal/modules/misc"
	"github.com/tiennm99/miti99bot/internal/modules/stats"
	"github.com/tiennm99/miti99bot/internal/modules/stock"
	"github.com/tiennm99/miti99bot/internal/modules/twentyq"
	"github.com/tiennm99/miti99bot/internal/modules/util"
	"github.com/tiennm99/miti99bot/internal/modules/wordle"
	"github.com/tiennm99/miti99bot/internal/server"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/telegram"
)

// gitSHA is populated at build time via `-ldflags "-X main.gitSHA=<sha>"`
// (see Makefile). Empty value means the binary was built without that flag —
// deploynotify treats it as a signal to stay silent.
var gitSHA string

// factories is the static module catalog. Adding a new module is a one-line
// change here. Lives in main rather than the modules package to avoid an
// import cycle (modules → util → modules).
func factories() map[string]modules.Factory {
	return map[string]modules.Factory{
		"util":        util.New,
		"misc":        misc.New,
		"wordle":      wordle.New,
		"loldle":      loldle.New,
		"lolschedule": lolschedule.New,
		"coin":        coin.New,
		"gold":        gold.New,
		"twentyq":     twentyq.New,
		"stock":       stock.New,
		"stats":       stats.New,
	}
}

// mongodbInitTimeout caps MongoDB connect+ping at startup (and Disconnect at
// shutdown). Atlas SRV DNS + TLS handshake can take a couple seconds on a cold
// container; 10s leaves headroom without hiding a wedged cluster.
const mongodbInitTimeout = 10 * time.Second

// ssmInitTimeout caps cold-start secret resolution. Secrets are fetched once
// at startup from Parameter Store when *_PARAMETER_NAME env vars are set.
const ssmInitTimeout = 5 * time.Second

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := loadConfig()
	if err := resolveSSMSecrets(rootCtx, &cfg); err != nil {
		log.Fatal("ssm secret resolution failed", "err", err)
	}
	if cfg.TelegramBotToken == "" {
		log.Fatal("missing required env", "key", "TELEGRAM_BOT_TOKEN")
	}
	exportOptionalEnv("GOLD_PRICE_API_URL", cfg.GoldPriceAPIURL)
	exportOptionalEnv("GOLD_FX_API_URL", cfg.GoldFXAPIURL)
	exportOptionalEnv("GOLD_VNAPP_API_URL", cfg.GoldVNAppAPIURL)
	exportOptionalEnv("GOLD_VNAPP_API_KEY", cfg.GoldVNAppAPIKey)
	exportOptionalEnv("COIN_BINANCE_API_URL", cfg.CoinBinanceAPIURL)
	exportOptionalEnv("COIN_COINBASE_API_URL", cfg.CoinCoinbaseAPIURL)
	exportOptionalEnv("COIN_COINGECKO_API_URL", cfg.CoinCoinGeckoAPIURL)

	// Periodic metrics flush. Cancels with rootCtx and emits one final
	// flush on shutdown so the trailing window isn't lost.
	go metrics.Run(rootCtx)

	provider, closeProvider, err := buildProvider(rootCtx, cfg)
	if err != nil {
		log.Fatal("storage init failed", "err", err)
	}
	defer closeProvider()

	b, err := telegram.NewBot(cfg.TelegramBotToken)
	if err != nil {
		log.Fatal("telegram bot init failed", "err", err)
	}

	// Gemini is optional: twentyq checks for nil and refuses the command at
	// handler time. A blank GEMINI_API_KEY is therefore not fatal — the rest
	// of the bot still runs.
	aiClient, err := ai.NewClient(rootCtx, cfg.GeminiAPIKey)
	if err != nil && !errors.Is(err, ai.ErrNotConfigured) {
		log.Fatal("gemini init failed", "err", err)
	}
	if aiClient == nil {
		log.Warn("GEMINI_API_KEY unset; AI-backed modules will refuse commands")
	} else {
		log.Info("gemini client initialised")
	}

	reg, err := modules.Build(cfg.Modules, factories(), provider, modules.BuildOptions{
		Chatter: aiClient,
		Bot:     b,
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

	// In-process cron scheduler. Replaces EventBridge Scheduler off AWS; runs
	// unconditionally so the long-lived container fires module crons (e.g. the
	// lolschedule daily push) on their Schedule. Cutover safety comes from
	// ordering + the per-date idempotency guard, not from a gate.
	stopCron, err := cron.Run(rootCtx, reg)
	if err != nil {
		log.Fatal("cron scheduler init failed", "err", err)
	}
	defer stopCron()

	if cfg.BotOwnerID == 0 {
		log.Warn("OWNER_ID unset; all Private + Protected commands will be denied")
	}

	// Clear any webhook left over from the AWS deployment at startup, before the
	// owner DM and before polling. getUpdates (long polling, below) returns HTTP
	// 409 while a webhook is set, so a stuck webhook silently breaks the bot.
	// Best-effort, one shot: a real failure here is logged, not retried.
	if err := telegram.DeleteWebhook(rootCtx, cfg.TelegramBotToken); err != nil {
		log.Warn("deleteWebhook failed; getUpdates may 409 if a webhook is set", "err", err)
	} else {
		log.Info("webhook cleared")
	}

	deploynotify.Run(rootCtx, deploynotify.Config{
		Bot:     b,
		Store:   deploynotify.NewStore(provider.Collection("deploynotify")),
		OwnerID: cfg.BotOwnerID,
		GitSHA:  gitSHA,
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
// runs (MODULES=). DynamoDB is no longer a runtime backend — it survives only
// as the one-off migration source (cmd/migrate-dynamo-to-mongo).
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
		// DynamoDB is no longer a runtime backend — it survives only as the
		// one-off migration source (cmd/migrate-dynamo-to-mongo).
		return nil, func() {}, fmt.Errorf("unknown KV_PROVIDER %q (want memory|mongodb)", backend)
	}
}

type config struct {
	Port                  string
	TelegramBotToken      string
	GeminiAPIKey          string
	GoldPriceAPIURL       string
	GoldFXAPIURL          string
	GoldVNAppAPIURL       string
	GoldVNAppAPIKey       string
	CoinBinanceAPIURL     string
	CoinCoinbaseAPIURL    string
	CoinCoinGeckoAPIURL   string
	Modules               []string
	BotOwnerID            int64
	AdminUserIDs          map[int64]bool
	KVProvider            string // empty = auto-detect; or "memory"|"mongodb"
	MongoURL              string // required when KVProvider=mongodb (Atlas SRV connection string; SECRET — never log)
	MongoDatabase         string // required when KVProvider=mongodb
	TelegramBotTokenParam string
	GeminiAPIKeyParam     string
	GoldVNAppAPIKeyParam  string
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
		Port:                  port,
		TelegramBotToken:      envMap["TELEGRAM_BOT_TOKEN"],
		GeminiAPIKey:          envMap["GEMINI_API_KEY"],
		GoldPriceAPIURL:       envMap["GOLD_PRICE_API_URL"],
		GoldFXAPIURL:          envMap["GOLD_FX_API_URL"],
		GoldVNAppAPIURL:       envMap["GOLD_VNAPP_API_URL"],
		GoldVNAppAPIKey:       envMap["GOLD_VNAPP_API_KEY"],
		CoinBinanceAPIURL:     envMap["COIN_BINANCE_API_URL"],
		CoinCoinbaseAPIURL:    envMap["COIN_COINBASE_API_URL"],
		CoinCoinGeckoAPIURL:   envMap["COIN_COINGECKO_API_URL"],
		Modules:               splitCSV(envMap["MODULES"]),
		BotOwnerID:            parseInt64(envMap["OWNER_ID"]),
		AdminUserIDs:          parseInt64Set(envMap["ADMIN_IDS"]),
		KVProvider:            envMap["KV_PROVIDER"],
		MongoURL:              envMap["MONGO_URL"],
		MongoDatabase:         envMap["MONGO_DATABASE"],
		TelegramBotTokenParam: strings.TrimSpace(envMap["TELEGRAM_BOT_TOKEN_PARAMETER_NAME"]),
		GeminiAPIKeyParam:     strings.TrimSpace(envMap["GEMINI_API_KEY_PARAMETER_NAME"]),
		GoldVNAppAPIKeyParam:  strings.TrimSpace(envMap["GOLD_VNAPP_API_KEY_PARAMETER_NAME"]),
	}
}

func resolveSSMSecrets(ctx context.Context, cfg *config) error {
	bindings := []struct {
		name   string
		target *string
	}{
		{name: cfg.TelegramBotTokenParam, target: &cfg.TelegramBotToken},
		{name: cfg.GeminiAPIKeyParam, target: &cfg.GeminiAPIKey},
		{name: cfg.GoldVNAppAPIKeyParam, target: &cfg.GoldVNAppAPIKey},
	}

	targetsByName := map[string][]*string{}
	names := make([]string, 0, len(bindings))
	for _, b := range bindings {
		if b.name == "" || *b.target != "" {
			continue
		}
		if _, ok := targetsByName[b.name]; !ok {
			names = append(names, b.name)
		}
		targetsByName[b.name] = append(targetsByName[b.name], b.target)
	}
	if len(names) == 0 {
		return nil
	}

	initCtx, cancel := context.WithTimeout(ctx, ssmInitTimeout)
	defer cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(initCtx, awsconfig.WithHTTPClient(&http.Client{
		Timeout: ssmInitTimeout,
	}))
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := ssm.NewFromConfig(awsCfg)
	out, err := client.GetParameters(initCtx, &ssm.GetParametersInput{
		Names:          names,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("get parameters: %w", err)
	}
	if len(out.InvalidParameters) > 0 {
		return fmt.Errorf("missing SSM parameters: %s", strings.Join(out.InvalidParameters, ","))
	}
	for _, p := range out.Parameters {
		name := aws.ToString(p.Name)
		value := aws.ToString(p.Value)
		for _, target := range targetsByName[name] {
			*target = value
		}
	}
	log.Info("loaded secrets from ssm", "count", len(out.Parameters))
	return nil
}

func exportOptionalEnv(key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if err := os.Setenv(key, value); err != nil {
		log.Warn("could not export optional env", "key", key, "err", err)
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
