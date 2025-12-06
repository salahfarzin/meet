package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/salahfarzin/meet/pkg/middlewares"
	"github.com/salahfarzin/meet/router"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RESTServer struct {
	Server *runtime.ServeMux
	App    *App
}

func NewRESTServer(app *App) *RESTServer {
	return &RESTServer{
		Server: runtime.NewServeMux(),
		App:    app,
	}
}

func (s *RESTServer) Start(ctx context.Context) error {
	grpcAddr := fmt.Sprintf(":%d", s.App.Configs.GRPCPort)

	// Configure gateway to forward custom headers as metadata
	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			switch strings.ToLower(key) {
			case "x-user", "x-user-uuid", "x-user-roles":
				return key, true
			default:
				return runtime.DefaultHeaderMatcher(key)
			}
		}),
	)
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// or use your middleware stack
	err := router.SetupRESTRoutes(ctx, mux, grpcAddr, opts)
	if err != nil {
		return err
	}

	log.Printf("REST gateway listening on %d", s.App.Configs.Port)

	authFunc := func(token string) (*middlewares.User, error) {
		client := &http.Client{}

		url := s.App.Configs.AuthService + "/me"
		req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
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
			return nil, fmt.Errorf("invalid token, status: %d", resp.StatusCode)
		}

		var user middlewares.User
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			return nil, err
		}
		return &user, nil
	}

	var handler http.Handler = mux
	handler = middlewares.CreateStack(
		middlewares.JSONHeader,
		middlewares.CORSMiddleware(s.App.AllowedOrigins),
		middlewares.LoggingMiddleware(s.App.Logger, s.App.Configs.Log.Level),
		middlewares.AuthMiddleware(authFunc),
		// add more middlewares here
	)(handler)

	prefix := s.App.Configs.RestPrefix
	if prefix == "" {
		prefix = "/api/v1"
	}

	http.Handle(prefix+"/", http.StripPrefix(prefix, handler))

	server := &http.Server{
		Addr:         ":" + strconv.FormatInt(s.App.Configs.Port, 10),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		return err
	}

	return nil
}
