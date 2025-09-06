package logger

import (
	"context"

	"go.uber.org/zap"
)

// contextKeyLogger is the type used for storing the logger in context
type contextKeyLogger struct{}

// WithLogger returns a new context with the provided logger
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKeyLogger{}, logger)
}

// FromContext extracts the logger from context, or returns zap.L() if not found
func FromContext(ctx context.Context) *zap.Logger {
	l, ok := ctx.Value(contextKeyLogger{}).(*zap.Logger)
	if !ok || l == nil {
		return zap.L()
	}
	return l
}
