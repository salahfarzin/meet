package configs

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Log struct {
	Level string `env:"LOG_LEVEL,required"`
	Path  string `env:"LOG_PATH,required"`
}

type DBDriver struct {
	Driver          string `env:"DB_DRIVER,required"`
	User            string `env:"DB_USER,required"`
	Password        string `env:"DB_PASSWORD,required"`
	Address         string `env:"DB_ADDRESS,required"`
	Name            string `env:"DB_NAME,required"`
	MaxOpenConns    int    `env:"DB_MAX_OPEN_CONNS,required"`
	MaxIdleConns    int    `env:"DB_MAX_IDLE_CONNS,required"`
	ConnMaxLifetime int    `env:"DB_CONN_MAX_LIFETIME,required"`
}

type CORS struct {
	AllowedOrigins []string
}

type Configs struct {
	AppName    string `env:"APP_NAME,required"`
	AppEnv     string `env:"APP_ENV,required"`
	Version    string `env:"APP_VERSION,required"`
	URL        string `env:"APP_URL,required"`
	Port       int64  `env:"APP_PORT,required"`
	GRPCPort   int64  `env:"GRPC_PORT,required"`
	RestPrefix string `env:"REST_PREFIX,required"`

	AuthService string `env:"AUTH_SERVICE,required"`

	Log  Log
	DB   DBDriver
	CORS CORS
}

func Init() *Configs {
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	return &Configs{
		AppName:     getEnv("APP_NAME", "AuthService"),
		AppEnv:      getEnv("APP_ENV", "development"),
		Version:     getEnv("APP_VERSION", "0.1.0"),
		URL:         getEnv("APP_URL", "http://localhost"),
		Port:        getEnvAsInt("APP_PORT", 8080),
		GRPCPort:    getEnvAsInt("APP_GRPC_PORT", 50052),
		RestPrefix:  getEnv("REST_PREFIX", "/api/v1"),
		AuthService: getEnv("AUTH_SERVICE", "localhost:8082"),
		Log: Log{
			Level: getEnv("LOG_LEVEL", "debug"),
			Path:  getEnv("LOG_PATH", "./storage/logs"),
		},
		DB: DBDriver{
			Driver:          getEnv("DB_DRIVER", "mysql"),
			User:            getEnv("DB_USER", "root"),
			Password:        getEnv("DB_PASSWORD", "mypassword"),
			Address:         fmt.Sprintf("%s:%s", getEnv("DB_HOST", "127.0.0.1"), getEnv("DB_PORT", "3306")),
			Name:            getEnv("DB_NAME", "ecom"),
			MaxOpenConns:    25,
			MaxIdleConns:    25,
			ConnMaxLifetime: 5,
		},
		CORS: CORS{
			AllowedOrigins: parseCORSOrigins(),
		},
	}
}

// Gets the env by key or fallbacks
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func getEnvAsInt(key string, fallback int64) int64 {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fallback
		}

		return i
	}

	return fallback
}

// parseCORSOrigins reads CORS allowed origins from environment variable
// Expected format: CORS_ALLOWED_ORIGINS=http://localhost:5173,https://dashboard.psychometrist.local,https://api.psychometrist.local
func parseCORSOrigins() []string {
	originsStr := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	return splitAndTrim(originsStr, ",")
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}
