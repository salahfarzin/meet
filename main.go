package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/salahfarzin/logger"
	"github.com/salahfarzin/meet/cmd/api"
	"github.com/salahfarzin/meet/configs"
	"github.com/salahfarzin/meet/pkg/db"
	"go.uber.org/zap"
)

func main() {
	cfg := configs.Init()
	curPath, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get current working directory:", err)
	}
	currentDate := time.Now().Format("2006-01-02")
	loggerFile := "app-" + currentDate + ".log"

	logger.Init(&zap.Config{OutputPaths: []string{filepath.Join(curPath, cfg.Log.Path, loggerFile)}, Level: zap.NewAtomicLevelAt(zap.DebugLevel)})
	defer logger.Sync() // flush logs on shutdown

	dbConn, err := db.NewMySQLStorage(cfg)
	if err != nil {
		log.Fatal(err)
	}

	app := &api.App{
		Configs: cfg,
		Logger:  logger.Get(),
		DB:      dbConn,
		AllowedOrigins: []string{
			"http://localhost:5173",
			"https://dashboard.psychometrist.local",
			"https://api.psychometrist.local",
		},
	}

	logger.Init(&zap.Config{OutputPaths: []string{filepath.Join(curPath, cfg.Log.Path, loggerFile)}, Level: zap.NewAtomicLevelAt(zap.DebugLevel)})
	defer logger.Sync()

	app.Serve()
}
