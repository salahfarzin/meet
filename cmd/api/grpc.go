package api

import (
	"context"
	"fmt"
	"log"
	"net"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/salahfarzin/meet/pkg/logger"
	"github.com/salahfarzin/meet/router"
	"github.com/salahfarzin/meet/utils"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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

func NewGRPCServer(app *App) *GRPCServer {
	server := grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				loggingInterceptor(app),
				// Add more interceptors here
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

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	router.SetupGRPCRoutes(s.grpcServer, s.app.DB)

	reflection.Register(s.grpcServer) // Register reflection service on gRPC server
	log.Printf("gRPC server listening on %s", address)
	return s.grpcServer.Serve(listener)
}

func (s *GRPCServer) Stop() {
	s.grpcServer.GracefulStop()
}
