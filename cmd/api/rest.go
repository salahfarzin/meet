package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/salahfarzin/meet/internal/health"
	"github.com/salahfarzin/meet/pkg/swagger"
	"github.com/salahfarzin/meet/router"
	"github.com/salahfarzin/utils/middlewares"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

type RESTServer struct {
	App *App
}

func NewRESTServer(app *App) *RESTServer {
	return &RESTServer{
		App: app,
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
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true, // Use snake_case from proto files
				EmitUnpopulated: true, // Optional: includes fields with default values
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
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

	// Skip authentication in test mode for E2E testing
	if s.App.Configs.AppEnv == "test" {
		log.Println("⚠️  Running in TEST mode - authentication is DISABLED")
		handler = middlewares.CreateStack(
			middlewares.JSONHeader,
			middlewares.CORSMiddleware(s.App.AllowedOrigins),
			middlewares.LoggingMiddleware(s.App.Logger, s.App.Configs.Log.Level),
			middlewares.TestAuthMiddleware(), // Use test auth instead of real auth
		)(handler)
	} else {
		handler = middlewares.CreateStack(
			middlewares.JSONHeader,
			middlewares.CORSMiddleware(s.App.AllowedOrigins),
			middlewares.LoggingMiddleware(s.App.Logger, s.App.Configs.Log.Level),
			middlewares.AuthMiddleware(authFunc),
			// add more middlewares here
		)(handler)
	}

	prefix := s.App.Configs.RestPrefix
	if prefix == "" {
		prefix = "/api/v1"
	}

	// Health check endpoints (bypass authentication middleware)
	healthHandler := health.NewHealthHandler(s.App.DB, s.App.Configs.Version)
	http.HandleFunc(prefix+"/health", healthHandler.Health)
	http.HandleFunc(prefix+"/live", healthHandler.Live)
	http.HandleFunc(prefix+"/ready", healthHandler.Ready)

	http.Handle(prefix+"/", http.StripPrefix(prefix, handler))

	// Serve openapi.yaml and postman_collection.json from embedded files
	yamlFS, _ := fs.Sub(swagger.Gen, "gen")
	http.Handle("/openapi.yaml", http.FileServer(http.FS(yamlFS)))
	http.Handle("/postman_collection.json", http.FileServer(http.FS(yamlFS)))

	// Serve Documentation UI from embedded files
	uiFS, _ := fs.Sub(swagger.UI, "assets")
	docsTmpl, err := template.ParseFS(swagger.UI, "assets/index.html")
	if err != nil {
		log.Printf("failed to parse docs template: %v", err)
	}

	http.HandleFunc(prefix+"/docs/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, prefix+"/docs")
		if (path == "/" || path == "" || path == "/index.html") && docsTmpl != nil {
			data := map[string]string{
				"PostmanCollection": "/postman_collection.json",
				"OpenapiYaml":       "/openapi.yaml",
			}
			if err := docsTmpl.ExecuteTemplate(w, "index.html", data); err != nil {
				log.Printf("docs template error: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}
		http.StripPrefix(prefix+"/docs", http.FileServer(http.FS(uiFS))).ServeHTTP(w, r)
	})

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
