// Package logging configures the process-wide structured logger.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup installs a slog handler as the default logger and returns it.
//
// Development uses human-readable text; anything else uses JSON so log
// aggregators can parse it. LOG_LEVEL (debug|info|warn|error) overrides the
// default level of info.
func Setup(appEnv string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: levelFromEnv()}

	var h slog.Handler
	if appEnv == "development" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}

// levelFromEnv reads LOG_LEVEL, defaulting to info for empty or unknown values.
func levelFromEnv() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
