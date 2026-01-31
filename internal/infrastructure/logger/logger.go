package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// coloredHandler wraps a slog.TextHandler and adds ANSI color codes based on log level.
type coloredHandler struct {
	handler slog.Handler
	writer  io.Writer
	enabled bool
}

func newColoredHandler(w io.Writer, opts *slog.HandlerOptions) *coloredHandler {
	// Check if output is a TTY to enable colors
	enabled := isTerminal(w)
	
	// Use a colorWriter to intercept and colorize the output
	coloredWriter := &colorWriter{
		writer: w,
		enabled: enabled,
	}
	
	return &coloredHandler{
		handler: slog.NewTextHandler(coloredWriter, opts),
		writer:  w,
		enabled: enabled,
	}
}

func (h *coloredHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *coloredHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.handler.Handle(ctx, record)
}

func (h *coloredHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &coloredHandler{
		handler: h.handler.WithAttrs(attrs),
		writer:  h.writer,
		enabled: h.enabled,
	}
}

func (h *coloredHandler) WithGroup(name string) slog.Handler {
	return &coloredHandler{
		handler: h.handler.WithGroup(name),
		writer:  h.writer,
		enabled: h.enabled,
	}
}

// colorWriter wraps an io.Writer and adds color codes around the level string.
type colorWriter struct {
	writer io.Writer
	enabled bool
}

func (cw *colorWriter) Write(p []byte) (n int, err error) {
	if !cw.enabled {
		// If colors are disabled, write directly
		return cw.writer.Write(p)
	}
	
	// Convert to string to manipulate
	text := string(p)
	
	// Add colors to level indicators (slog.TextHandler format: "level=INFO")
	text = strings.ReplaceAll(text, "level=DEBUG", colorCyan+"level=DEBUG"+colorReset)
	text = strings.ReplaceAll(text, "level=INFO", colorGreen+"level=INFO"+colorReset)
	text = strings.ReplaceAll(text, "level=WARN", colorYellow+"level=WARN"+colorReset)
	text = strings.ReplaceAll(text, "level=ERROR", colorRed+"level=ERROR"+colorReset)
	
	// Write the colored text
	_, err = cw.writer.Write([]byte(text))
	return len(p), err
}

// isTerminal checks if the writer is a terminal (TTY).
func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	
	// Check if it's a terminal
	info, err := file.Stat()
	if err != nil {
		return false
	}
	
	// On Unix systems, check if it's a character device
	mode := info.Mode()
	return (mode & os.ModeCharDevice) != 0
}

// New builds a structured slog logger honoring the configured level and environment.
// For development environments (local, dev, development), it uses colored text output.
// For production environments (prod, production, staging), it uses JSON output.
func New(appName, level, environment string) *slog.Logger {
	env := strings.ToLower(strings.TrimSpace(environment))
	opts := &slog.HandlerOptions{
		Level:     parseLevel(level),
		AddSource: true,
	}

	var handler slog.Handler

	// Use colored text handler for development environments
	if env == "local" || env == "dev" || env == "development" {
		handler = newColoredHandler(os.Stdout, opts)
	} else {
		// Use JSON handler for production and other environments
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler).With("app", appName)
}

func parseLevel(level string) slog.Leveler {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
