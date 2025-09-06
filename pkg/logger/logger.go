package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
	once   sync.Once
)

// Init initializes a global zap logger writing JSON logs to a file.
func Init(logFilePath string) {
	once.Do(func() {
		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		encoderCfg := zap.NewProductionEncoderConfig()
		encoderCfg.TimeKey = "timestamp"
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(file),
			zapcore.InfoLevel,
		)

		logger = zap.New(core)
	})
}

// GetLogger returns the global zap logger instance
func GetLogger() *zap.Logger {
	if logger == nil {
		panic("Logger not initialized, call Init() first")
	}
	return logger
}
