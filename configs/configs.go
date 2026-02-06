package configs

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/salahfarzin/utils"
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
	SSLCA           string `env:"DB_SSL_CA"`
	SSLCert         string `env:"DB_SSL_CERT"`
	SSLKey          string `env:"DB_SSL_KEY"`
	SSLVerify       bool   `env:"DB_SSL_VERIFY,default=false"`
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
		AppName:     utils.GetEnv("APP_NAME", "Meet Service"),
		AppEnv:      utils.GetEnv("APP_ENV", "development"),
		Version:     utils.GetEnv("APP_VERSION", "0.1.0"),
		URL:         utils.GetEnv("APP_URL", "http://localhost"),
		Port:        utils.GetEnvAsInt("APP_PORT", 8080),
		GRPCPort:    utils.GetEnvAsInt("APP_GRPC_PORT", 50052),
		RestPrefix:  utils.GetEnv("REST_PREFIX", "/api/v1"),
		AuthService: utils.GetEnv("AUTH_SERVICE", "localhost:8082"),
		Log: Log{
			Level: utils.GetEnv("LOG_LEVEL", "debug"),
			Path:  utils.GetEnv("LOG_PATH", "./storage/logs"),
		},
		DB: DBDriver{
			Driver:          utils.GetEnv("DB_DRIVER", "mysql"),
			User:            utils.GetEnv("DB_USER", "root"),
			Password:        utils.GetEnv("DB_PASSWORD", "mypassword"),
			Address:         fmt.Sprintf("%s:%s", utils.GetEnv("DB_HOST", "127.0.0.1"), utils.GetEnv("DB_PORT", "3306")),
			Name:            utils.GetEnv("DB_NAME", "ecom"),
			MaxOpenConns:    25,
			MaxIdleConns:    25,
			ConnMaxLifetime: 5,
			SSLCA:           utils.GetEnv("DB_ATTR_SSL_CA", ""),
			SSLKey:          utils.GetEnv("DB_ATTR_SSL_KEY", ""),
			SSLCert:         utils.GetEnv("DB_ATTR_SSL_CERT", ""),
			SSLVerify:       utils.GetEnvAsBool("DB_ATTR_SSL_VERIFY_SERVER_CERT", false),
		},
		CORS: CORS{
			AllowedOrigins: utils.ParseCORSOrigins(),
		},
	}
}
