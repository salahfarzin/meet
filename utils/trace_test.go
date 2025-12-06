package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestGetOrGenerateTraceID_WithoutTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := GetOrGenerateTraceID(ctx)

	assert.NotEmpty(t, traceID)
	assert.Len(t, traceID, 36) // UUID v4 length
}

func TestGetOrGenerateTraceID_WithEmptyTraceID(t *testing.T) {
	md := metadata.MD{
		"x-trace-id": []string{""},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)
	traceID := GetOrGenerateTraceID(ctx)

	assert.NotEmpty(t, traceID)
	assert.Len(t, traceID, 36) // Should generate new UUID
}

func TestGetOrGenerateTraceID_WithMultipleTraceIDs(t *testing.T) {
	md := metadata.MD{
		"x-trace-id": []string{"first-trace", "second-trace"},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)
	traceID := GetOrGenerateTraceID(ctx)

	assert.Equal(t, "first-trace", traceID) // Should return first value
}

func TestGetOrGenerateTraceID_WithExistingTraceID(t *testing.T) {
	md := metadata.MD{
		"x-trace-id": []string{"existing-trace-id"},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)
	traceID := GetOrGenerateTraceID(ctx)

	assert.Equal(t, "existing-trace-id", traceID)
}

func TestGetUserIDFromContext_WithoutUserID(t *testing.T) {
	ctx := context.Background()
	userID := GetUserIDFromContext(ctx)

	assert.Equal(t, "", userID)
}

func TestGetUserIDFromContext_WithEmptyUserID(t *testing.T) {
	md := metadata.MD{
		"x-user-id": []string{""},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)
	userID := GetUserIDFromContext(ctx)

	assert.Equal(t, "", userID)
}

func TestGetUserIDFromContext_WithUserID(t *testing.T) {
	md := metadata.MD{
		"x-user-id": []string{"user-123"},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)
	userID := GetUserIDFromContext(ctx)

	assert.Equal(t, "user-123", userID)
}

func TestGetUserIDFromContext_WithMultipleUserIDs(t *testing.T) {
	md := metadata.MD{
		"x-user-id": []string{"user-123", "user-456"},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)
	userID := GetUserIDFromContext(ctx)

	assert.Equal(t, "user-123", userID) // Should return first value
}
