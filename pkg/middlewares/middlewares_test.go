package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"
)

func TestGetUser(t *testing.T) {
	user := &User{ID: "123", Uuid: "uuid-123", Email: "test@example.com"}

	// Test with user in context
	ctx := context.WithValue(context.Background(), userKey, user)
	retrievedUser, ok := GetUser(ctx)
	assert.True(t, ok)
	assert.Equal(t, user, retrievedUser)

	// Test without user in context
	ctx = context.Background()
	retrievedUser, ok = GetUser(ctx)
	assert.False(t, ok)
	assert.Nil(t, retrievedUser)
}

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	middleware := CORSMiddleware([]string{"http://localhost:3000"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	middleware := CORSMiddleware([]string{"http://localhost:3000"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_OptionsRequest(t *testing.T) {
	middleware := CORSMiddleware([]string{"http://localhost:3000"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called for OPTIONS request")
	}))

	req := httptest.NewRequest("OPTIONS", "/test", http.NoBody)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
}

func TestJSONHeader(t *testing.T) {
	handler := JSONHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestLoggingMiddleware(t *testing.T) {
	logger := zaptest.NewLogger(t)
	middleware := LoggingMiddleware(logger, "debug")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	authService := func(token string) (*User, error) {
		if token == "valid-token" {
			return &User{ID: "123", Uuid: "uuid-123", Email: "test@example.com", Roles: []string{"admin"}}, nil
		}
		return nil, http.ErrNoCookie
	}

	middleware := AuthMiddleware(authService)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUser(r.Context())
		assert.True(t, ok)
		assert.Equal(t, "123", user.ID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "123", req.Header.Get("x-user-id"))
	assert.Equal(t, "uuid-123", req.Header.Get("x-user-uuid"))
	assert.Equal(t, "admin", req.Header.Get("x-user-roles"))
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	authService := func(token string) (*User, error) {
		return nil, http.ErrNoCookie
	}

	middleware := AuthMiddleware(authService)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	authService := func(token string) (*User, error) {
		return &User{}, nil
	}

	middleware := AuthMiddleware(authService)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExtractToken_FromHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer test-token")

	token := extractToken(req)
	assert.Equal(t, "test-token", token)
}

func TestExtractToken_FromCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-token"})

	token := extractToken(req)
	assert.Equal(t, "cookie-token", token)
}

func TestExtractToken_NoToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", http.NoBody)

	token := extractToken(req)
	assert.Equal(t, "", token)
}

func TestGetUserFromContext_HTTP(t *testing.T) {
	user := &User{ID: "123", Uuid: "uuid-123", Email: "test@example.com", Roles: []string{"admin"}}
	ctx := context.WithValue(context.Background(), userKey, user)

	result := GetUserFromContext(ctx)
	assert.Equal(t, "123", result.ID)
	assert.Equal(t, "uuid-123", result.Uuid)
	assert.Equal(t, "test@example.com", result.Email)
	assert.Equal(t, []string{"admin"}, result.Roles)
}

func TestGetUserFromContext_gRPC(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
		"x-user-id":    []string{"grpc-123"},
		"x-user-uuid":  []string{"grpc-uuid-123"},
		"x-user-email": []string{"grpc@example.com"},
		"x-user-roles": []string{"admin,user"},
	})

	result := GetUserFromContext(ctx)
	assert.Equal(t, "grpc-123", result.ID)
	assert.Equal(t, "grpc-uuid-123", result.Uuid)
	assert.Equal(t, "grpc@example.com", result.Email)
	assert.Equal(t, []string{"admin", "user"}, result.Roles)
}

func TestGetUserFromContext_NoUser(t *testing.T) {
	ctx := context.Background()

	result := GetUserFromContext(ctx)
	assert.Equal(t, "", result.ID)
	assert.Equal(t, "", result.Uuid)
	assert.Equal(t, "", result.Email)
	assert.Empty(t, result.Roles)
}

func TestCreateStack(t *testing.T) {
	callOrder := []string{}

	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "middleware1")
			next.ServeHTTP(w, r)
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "middleware2")
			next.ServeHTTP(w, r)
		})
	}

	middleware3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "middleware3")
			next.ServeHTTP(w, r)
		})
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})

	stack := CreateStack(middleware1, middleware2, middleware3)
	handler := stack(finalHandler)

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Middlewares should be applied in reverse order (LIFO)
	assert.Equal(t, []string{"middleware1", "middleware2", "middleware3", "handler"}, callOrder)
}

func TestCreateStack_Empty(t *testing.T) {
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	stack := CreateStack()
	handler := stack(finalHandler)

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
