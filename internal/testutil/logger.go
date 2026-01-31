package testutil

import (
	"log/slog"
	"os"
)

// NewTestLogger creates a logger suitable for testing.
func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// NewNullLogger creates a logger that discards all output.
func NewNullLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.NewFile(0, os.DevNull), &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}
