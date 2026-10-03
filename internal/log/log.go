// Package log is a thin facade over stdlib log/slog with a JSON handler
// configured for container stdout. JSON lines keep fields structured in the
// hosting log sink.
//
// Why a facade instead of importing slog directly: (1) callers stay
// log-package-agnostic (we can swap to logrus/zap later by editing one file);
// (2) the Fatal helper preserves stdlib's "log + exit 1" ergonomic; (3)
// LOG_LEVEL env is honoured at process start without every caller wiring it.
//
// Usage:
//
//	log.Info("server starting", "port", 8080)
//	log.Error("store write failed", "module", "misc", "command", "ping", "err", err)
//	log.Fatal("missing required env", "key", "TELEGRAM_BOT_TOKEN")
//
// slog escapes newlines and quotes in field values, so user-controlled text in
// a field cannot forge an extra log record.
package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// level is the default logger's minimum level. It starts at Info so logging
// works before configuration is read (and in tests); SetLevel applies the
// configured LOG_LEVEL at startup.
var level = new(slog.LevelVar)

// defaultLogger writes JSON to stdout. Tests can swap it via SetDefault — but
// the public Info/Warn/Error/Fatal helpers always read the current default so
// test substitutions take effect immediately.
var defaultLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

// ParseLevel maps a LOG_LEVEL value to a slog.Level. There is no fallback: an
// empty or unknown value is an error, so a typo cannot silently change what
// gets logged.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q: want debug, info, warn, or error", s)
	}
}

// SetLevel sets the default logger's minimum level.
func SetLevel(l slog.Level) { level.Set(l) }

// SetDefault swaps the package-level logger. Used by tests to capture output;
// production code never calls this.
func SetDefault(l *slog.Logger) { defaultLogger = l }

// Default returns the current logger. Useful when a caller needs the *slog.Logger
// directly (e.g. to pass into a third-party API that wants slog).
func Default() *slog.Logger { return defaultLogger }

// Debug, Info, Warn, Error route to the default logger. args is alternating
// key/value pairs (slog convention) or pre-built slog.Attr values.
func Debug(msg string, args ...any) { defaultLogger.Debug(msg, args...) }
func Info(msg string, args ...any)  { defaultLogger.Info(msg, args...) }
func Warn(msg string, args ...any)  { defaultLogger.Warn(msg, args...) }
func Error(msg string, args ...any) { defaultLogger.Error(msg, args...) }

// Fatal logs at Error level then exits with status 1, mirroring stdlib's
// log.Fatal ergonomic. Use only at startup boundaries — handlers should
// return errors, not exit.
func Fatal(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
	os.Exit(1)
}

// With returns a child logger that inlines the given attrs into every record.
// Useful for per-request scopes (e.g. attach a trace id once).
func With(args ...any) *slog.Logger { return defaultLogger.With(args...) }

// LogAttrs is a small re-export so callers can use slog.LogAttrs ergonomics
// (typed attrs, no allocation) without importing slog themselves.
func LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	defaultLogger.LogAttrs(ctx, level, msg, attrs...)
}
