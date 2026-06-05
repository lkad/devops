// Package logger is a thin wrapper around log/slog. It exists so the
// rest of the codebase has one constructor to call, and so sensitive
// value masking has a single home (see MaskValue).
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// sensitiveKeys lists field names that should be redacted when logged.
// Match is case-insensitive on the suffix, so "DB_PASSWORD" matches "password".
var sensitiveKeys = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"api_key",
	"apikey",
	"private_key",
	"privatekey",
	"credential",
}

// MaskValue returns "***" when key matches a sensitive pattern, else
// the value unchanged. Used by config and middleware to keep secrets
// out of log lines.
func MaskValue(key, value string) string {
	lower := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(lower, s) {
			return "***"
		}
	}
	return value
}

// Options configures a Logger. Use the With* helpers for safe construction.
type Options struct {
	Writer io.Writer
	Level  string
	Format string // "json" or "text"
}

// Option mutates Options in place.
type Option func(*Options)

// WithWriter sets the output destination. Defaults to os.Stdout.
func WithWriter(w io.Writer) Option {
	return func(o *Options) {
		if w != nil {
			o.Writer = w
		}
	}
}

// WithLevel sets the level by name. Unknown values fall back to Info.
func WithLevel(level string) Option {
	return func(o *Options) {
		o.Level = strings.ToLower(strings.TrimSpace(level))
	}
}

// WithFormat sets "json" or "text". Unknown values fall back to json.
func WithFormat(format string) Option {
	return func(o *Options) {
		o.Format = strings.ToLower(strings.TrimSpace(format))
	}
}

// Logger is the concrete wrapper. It embeds *slog.Logger so the
// standard Debug/Info/Warn/Error methods are available directly.
type Logger struct {
	*slog.Logger
	level slog.Level
}

// New builds a Logger from the given options. Missing options get
// safe defaults (stdout, info, json). It never returns nil.
func New(opts ...Option) *Logger {
	o := &Options{
		Writer: os.Stdout,
		Level:  "info",
		Format: "json",
	}
	for _, opt := range opts {
		opt(o)
	}

	level := parseLevel(o.Level)
	handlerOpts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	switch o.Format {
	case "text":
		h = slog.NewTextHandler(o.Writer, handlerOpts)
	default:
		h = slog.NewJSONHandler(o.Writer, handlerOpts)
	}

	return &Logger{Logger: slog.New(h), level: level}
}

// Level returns the configured minimum level. Used by tests and by
// callers that want to filter their own work.
func (l *Logger) Level() slog.Level {
	return l.level
}

// parseLevel converts a string to a slog.Level. Unknown/empty is Info.
func parseLevel(s string) slog.Level {
	switch s {
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
