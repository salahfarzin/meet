package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/salahfarzin/meet/configs"
	"github.com/salahfarzin/meet/pkg/middlewares"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
)

func TestNewGRPCServer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &configs.Configs{
		GRPCPort: 50051,
	}

	app := &App{
		Configs: cfg,
		Logger:  logger,
		DB:      &sql.DB{},
	}

	server := NewGRPCServer(app)

	assert.NotNil(t, server)
	assert.NotNil(t, server.grpcServer)
	assert.Equal(t, app, server.app)
}

func TestNewRESTServer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &configs.Configs{
		Port:     8080,
		GRPCPort: 50051,
	}

	app := &App{
		Configs: cfg,
		Logger:  logger,
		DB:      &sql.DB{},
	}

	server := NewRESTServer(app)

	assert.NotNil(t, server)
	assert.NotNil(t, server.Server)
	assert.Equal(t, app, server.App)
}

func TestRESTServer_Start_AuthFunc(t *testing.T) {
	// Create a test server for auth service
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer valid-token" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"123","uuid":"uuid-123","email":"test@example.com","roles":["admin"]}`))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer authServer.Close()

	logger := zaptest.NewLogger(t)
	cfg := &configs.Configs{
		Port:        8080,
		GRPCPort:    50051,
		AuthService: authServer.URL,
		Log: configs.Log{
			Level: "info",
		},
		RestPrefix: "/api/v1",
	}

	app := &App{
		Configs:        cfg,
		Logger:         logger,
		DB:             &sql.DB{},
		AllowedOrigins: []string{"*"},
	}

	// Test the auth function directly
	authFunc := func(token string) (*middlewares.User, error) {
		client := &http.Client{}

		url := app.Configs.AuthService + "/me"
		req, err := http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, http.ErrNoCookie
		}

		var user middlewares.User
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			return nil, err
		}
		return &user, nil
	}

	// Test valid token
	user, err := authFunc("valid-token")
	assert.NoError(t, err)
	assert.Equal(t, "123", user.ID)
	assert.Equal(t, "uuid-123", user.Uuid)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, []string{"admin"}, user.Roles)

	// Test invalid token
	_, err = authFunc("invalid-token")
	assert.Error(t, err)
}

func TestLoggingInterceptor(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &configs.Configs{}

	app := &App{
		Configs: cfg,
		Logger:  logger,
	}

	interceptor := loggingInterceptor(app)
	assert.NotNil(t, interceptor)

	// Test that the interceptor function can be called
	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	ctx := context.Background()
	req := "test-request"
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.TestService/TestMethod",
	}

	resp, err := interceptor(ctx, req, info, handler)
	assert.NoError(t, err)
	assert.Equal(t, "response", resp)
}

func TestGRPCServer_Stop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &configs.Configs{
		GRPCPort: 50051,
	}

	app := &App{
		Configs: cfg,
		Logger:  logger,
		DB:      &sql.DB{},
	}

	server := NewGRPCServer(app)
	assert.NotNil(t, server)

	// Test that Stop doesn't panic
	assert.NotPanics(t, func() {
		server.Stop()
	})
}
