package utils

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

// GetOrGenerateTraceID tries to extract a trace ID from gRPC metadata, or generates a new one if not present.
func GetOrGenerateTraceID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-trace-id"); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	return uuid.New().String()
}

// GetUserIDFromContext tries to extract a user ID from gRPC metadata.
func GetUserIDFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-user-id"); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	return ""
}
