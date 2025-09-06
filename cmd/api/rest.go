package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/salahfarzin/meet/pkg/middlewares"
	"github.com/salahfarzin/meet/router"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type responseRecorder struct {
	http.ResponseWriter
	body       *[]byte
	statusCode int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	*r.body = append(*r.body, b...)
	return len(b), nil
}

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

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// or use your middleware stack
	router.SetupRESTRoutes(ctx, mux, grpcAddr, opts)

	log.Printf("REST gateway listening on %d", s.App.Configs.Port)

	authFunc := func(token string) (*middlewares.User, error) {
		client := &http.Client{}
		url := s.App.Configs.AuthService + "/me"
		req, err := http.NewRequest("GET", url, nil)
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
		fmt.Println("Auth service response:", resp)
		// Example: parse JSON {"id": "...", "email": "...", "roles": ["..."]}
		var user middlewares.User
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			return nil, err
		}
		return &user, nil
	}

	var handler http.Handler = mux
	handler = middlewares.CreateStack(
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

	if err := http.ListenAndServe(":"+strconv.FormatInt(s.App.Configs.Port, 10), nil); err != nil {
		return err
	}

	return nil
}
