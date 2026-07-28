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
	"github.com/salahfarzin/meet/internal/meets"
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
	mux := newGatewayMux()

	if err := router.SetupRESTRoutes(ctx, mux, grpcAddr, []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}); err != nil {
		return err
	}
	log.Printf("REST gateway listening on %d", s.App.Configs.Port)

	var handler http.Handler = mux
	handler = s.buildMiddlewareStack(s.authFunc(ctx))(handler)

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

	registerDocsRoutes(prefix)

	server := &http.Server{
		Addr:         ":" + strconv.FormatInt(s.App.Configs.Port, 10),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server.ListenAndServe()
}

// newGatewayMux configures the grpc-gateway mux: it forwards the caller identity
// headers as metadata and marshals JSON in snake_case, matching the proto field names.
func newGatewayMux() *runtime.ServeMux {
	return runtime.NewServeMux(
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
}

// authFunc calls AUTH_SERVICE + "/me" with the caller's bearer token to resolve identity.
func (s *RESTServer) authFunc(ctx context.Context) func(token string) (*middlewares.User, error) {
	return func(token string) (*middlewares.User, error) {
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
}

// registerDocsRoutes serves the embedded OpenAPI/Postman files and Swagger-style docs UI.
func registerDocsRoutes(prefix string) {
	yamlFS, _ := fs.Sub(swagger.Gen, "gen")
	http.Handle("/openapi.yaml", http.FileServer(http.FS(yamlFS)))
	http.Handle("/postman_collection.json", http.FileServer(http.FS(yamlFS)))

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
}

// buildMiddlewareStack picks the auth strategy for the REST gateway based on config:
// AUTH_DISABLED bypasses the auth service entirely (local, non-public use only), test
// env swaps in a fixed test identity for E2E runs, and the default path delegates to
// the real auth service via authFunc.
func (s *RESTServer) buildMiddlewareStack(authFunc func(token string) (*middlewares.User, error)) middlewares.Middleware {
	base := []middlewares.Middleware{
		middlewares.JSONHeader,
		middlewares.CORSMiddleware(s.App.AllowedOrigins),
		middlewares.LoggingMiddleware(s.App.Logger, s.App.Configs.Log.Level),
	}

	switch {
	case s.App.Configs.AuthDisabled:
		log.Println("⚠️  AUTH_DISABLED=true - authentication is DISABLED (local use only)")
		base = append(base, localAuthBypassMiddleware())
	case s.App.Configs.AppEnv == "test":
		log.Println("⚠️  Running in TEST mode - authentication is DISABLED")
		base = append(base, middlewares.TestAuthMiddleware())
	default:
		base = append(base, middlewares.AuthMiddleware(authFunc))
	}

	return middlewares.CreateStack(base...)
}

// localAuthBypassMiddleware injects a fixed, fully-privileged local user for
// every request instead of calling the auth service. Unlike TestAuthMiddleware,
// it doesn't trust any client-supplied identity headers - there's nothing for a
// caller to spoof. Only safe when AUTH_DISABLED=true, i.e. the service isn't
// reachable from outside the local machine/network.
func localAuthBypassMiddleware() middlewares.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := &middlewares.User{
				ID:    "local",
				Uuid:  "local",
				Email: "local@local",
				Roles: []string{meets.RoleAdmin, meets.RoleSuperAdmin},
			}

			ctx := context.WithValue(r.Context(), middlewares.UserKey, user)

			r.Header.Set("x-user-id", user.ID)
			r.Header.Set("x-user-uuid", user.Uuid)
			r.Header.Set("x-user-roles", strings.Join(user.Roles, ","))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
