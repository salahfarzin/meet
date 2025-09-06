package api

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"

	"github.com/salahfarzin/meet/configs"
	"go.uber.org/zap"
)

type App struct {
	Configs        *configs.Configs
	Logger         *zap.Logger
	DB             *sql.DB
	AllowedOrigins []string
}

func (app *App) Serve() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// start grpc server
	go func() {
		if err := NewGRPCServer(app).Start(); err != nil {
			app.Logger.Fatal("gRPC server error", zap.Error(err))
		}
	}()

	// start rest server
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		if err := NewRESTServer(app).Start(ctx); err != nil {
			app.Logger.Fatal("REST server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	app.Logger.Info("Shutting down gracefully")
}
