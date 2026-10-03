// Package logging builds the JSON slog logger and the self-redacting Secret type.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

const redactedPlaceholder = "[REDACTED]"

// ParseLevel accepts "debug", "info", "warn" or "error" (case-insensitive).
func ParseLevel(levelName string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(levelName)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("unknown log level")
}

// New returns a JSON logger that writes one object per line.
func New(output io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level}))
}

// Secret holds a sensitive value that prints as a placeholder everywhere
// except through Reveal.
type Secret struct {
	value string
}

// NewSecret wraps a sensitive value.
func NewSecret(value string) Secret { return Secret{value: value} }

// Reveal returns the real value. Call it only where the secret is consumed.
func (secret Secret) Reveal() string { return secret.value }

// Len reports the length without exposing the value.
func (secret Secret) Len() int { return len(secret.value) }

// LogValue hides the secret from every slog handler.
func (Secret) LogValue() slog.Value { return slog.StringValue(redactedPlaceholder) }

// String hides the secret from fmt verbs such as %s and %v.
func (Secret) String() string { return redactedPlaceholder }

// GoString hides the secret from %#v.
func (Secret) GoString() string { return redactedPlaceholder }

// MarshalText hides the secret from encoding/json and similar encoders.
func (Secret) MarshalText() ([]byte, error) { return []byte(redactedPlaceholder), nil }

type contextKey struct{}

// WithLogger stores a request-scoped logger in the context.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext returns the request-scoped logger, or the fallback when none is set.
func FromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return logger
	}
	return fallback
}
