package api

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/salahfarzin/logger"
	"github.com/salahfarzin/meet/internal/identity"
	"github.com/salahfarzin/meet/router"
	"github.com/salahfarzin/meet/utils"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
)

type GRPCServer struct {
	grpcServer *grpc.Server
	app        *App
}

func loggingInterceptor(app *App) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		enrichedLogger := app.Logger.With(
			zap.String("trace_id", utils.GetOrGenerateTraceID(ctx)),
			zap.String("user_id", utils.GetUserIDFromContext(ctx)),
		)
		ctx = logger.WithLogger(ctx, enrichedLogger)
		return handler(ctx, req)
	}
}

// bearerInterceptor extracts the bearer token from incoming gRPC metadata
// (checks "authorization" and "grpcgateway-authorization" headers) and
// attaches it to the context via identity.WithBearer so the identity HTTP
// client can forward it on outbound requests.
func bearerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			token := extractBearer(md, "authorization", "grpcgateway-authorization")
			if token != "" {
				ctx = identity.WithBearer(ctx, token)
			}
		}
		return handler(ctx, req)
	}
}

// extractBearer returns the raw token from the first matching metadata key that
// carries a "Bearer <token>" value.
func extractBearer(md metadata.MD, keys ...string) string {
	for _, key := range keys {
		for _, v := range md.Get(key) {
			if strings.HasPrefix(strings.ToLower(v), "bearer ") {
				return v[7:] // strip "Bearer " prefix
			}
		}
	}
	return ""
}

func NewGRPCServer(app *App) *GRPCServer {
	server := grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				loggingInterceptor(app),
				bearerInterceptor(),
			),
		),
	)

	return &GRPCServer{
		grpcServer: server,
		app:        app,
	}
}

func (s *GRPCServer) Start() error {
	address := fmt.Sprintf(":%d", s.app.Configs.GRPCPort)

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", address)
	if err != nil {
		return err
	}

	idClient := identity.NewHTTPClient(
		s.app.Configs.UserService,
		&http.Client{Timeout: 10 * time.Second},
	)
	router.SetupGRPCRoutes(s.grpcServer, s.app.DB, idClient)

	reflection.Register(s.grpcServer) // Register reflection service on gRPC server
	log.Printf("gRPC server listening on %s", address)
	return s.grpcServer.Serve(listener)
}

func (s *GRPCServer) Stop() {
	s.grpcServer.GracefulStop()
}
