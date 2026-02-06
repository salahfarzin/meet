package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/salahfarzin/logger"
	"github.com/salahfarzin/meet/cmd/api"
	"github.com/salahfarzin/meet/configs"
	"github.com/salahfarzin/utils/db"
	"go.uber.org/zap"
)

func main() {
	app, cleanup, err := setup()
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	app.Serve()
}

func setup() (*api.App, func(), error) {
	cfg := configs.Init()
	curPath, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	currentDate := time.Now().Format("2006-01-02")
	loggerFile := "app-" + currentDate + ".log"

	// Ensure log directory exists
	logDir := filepath.Join(curPath, cfg.Log.Path)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, err
	}

	logger.Init(&zap.Config{OutputPaths: []string{filepath.Join(logDir, loggerFile)}, Level: zap.NewAtomicLevelAt(zap.DebugLevel)})

	dbConn, err := db.NewMySQLStorage(db.MySQLConfig{
		User:            cfg.DB.User,
		Password:        cfg.DB.Password,
		Address:         cfg.DB.Address,
		Name:            cfg.DB.Name,
		SSLCA:           cfg.DB.SSLCA,
		SSLCert:         cfg.DB.SSLCert,
		SSLKey:          cfg.DB.SSLKey,
		SSLVerify:       cfg.DB.SSLVerify,
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
	})
	if err != nil {
		_ = logger.Sync() // flush logs before exit
		return nil, nil, err
	}

	app := &api.App{
		Configs:        cfg,
		Logger:         logger.Get(),
		DB:             dbConn,
		AllowedOrigins: cfg.CORS.AllowedOrigins,
	}

	cleanup := func() {
		_ = logger.Sync()
		if dbConn != nil {
			dbConn.Close()
		}
	}

	return app, cleanup, nil
}
