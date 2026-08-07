package utils

import (
	"log/slog"
	"os"
)

var defaultLevel = slog.LevelInfo

// SetupLogger returns a named structured logger.
func SetupLogger(name string) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: defaultLevel,
	})).With("component", name)
}

// SetLogLevel changes the global log level for newly created loggers.
func SetLogLevel(level slog.Level) {
	defaultLevel = level
}
