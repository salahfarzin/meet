package configs

import (
	"os"
	"testing"

	"github.com/salahfarzin/utils"
	"github.com/stretchr/testify/assert"
)

// configEnvVars lists every env var read by Init, used to snapshot/restore the
// environment around tests so they don't leak state into each other.
var configEnvVars = []string{
	"APP_NAME", "APP_ENV", "APP_VERSION", "APP_URL", "APP_PORT", "APP_GRPC_PORT",
	"REST_PREFIX", "AUTH_SERVICE", "LOG_LEVEL", "LOG_PATH", "DB_DRIVER", "DB_USER", "DB_PASSWORD",
	"DB_HOST", "DB_PORT", "DB_NAME",
}

// withClearedConfigEnv clears configEnvVars for the duration of the test and restores
// the original values (or absence thereof) on cleanup.
func withClearedConfigEnv(t *testing.T) {
	t.Helper()
	originalEnv := make(map[string]string)
	for _, env := range configEnvVars {
		if value, exists := os.LookupEnv(env); exists {
			originalEnv[env] = value
		}
		_ = os.Unsetenv(env)
	}

	t.Cleanup(func() {
		for _, env := range configEnvVars {
			if value, exists := originalEnv[env]; exists {
				_ = os.Setenv(env, value)
			} else {
				_ = os.Unsetenv(env)
			}
		}
	})
}

func TestInit(t *testing.T) {
	withClearedConfigEnv(t)

	// Test with default values
	config := Init()

	// Check default values
	assert.Equal(t, "Meet Service", config.AppName)
	assert.Equal(t, "development", config.AppEnv)
	assert.Equal(t, "0.1.0", config.Version)
	assert.Equal(t, "http://localhost", config.URL)
	assert.Equal(t, int64(8080), config.Port)
	assert.Equal(t, int64(50052), config.GRPCPort)
	assert.Equal(t, "/api/v1", config.RestPrefix)
	assert.Equal(t, "localhost:8082", config.AuthService)
	assert.Equal(t, "debug", config.Log.Level)
	assert.Equal(t, "./storage/logs", config.Log.Path)
	assert.Equal(t, "mysql", config.DB.Driver)
	assert.Equal(t, "root", config.DB.User)
	assert.Equal(t, "mypassword", config.DB.Password)
	assert.Equal(t, "127.0.0.1:3306", config.DB.Address)
	assert.Equal(t, "ecom", config.DB.Name)
	assert.Equal(t, 25, config.DB.MaxOpenConns)
	assert.Equal(t, 25, config.DB.MaxIdleConns)
	assert.Equal(t, 5, config.DB.ConnMaxLifetime)
}

func TestInitWithCustomEnvVars(t *testing.T) {
	withClearedConfigEnv(t)

	// Set custom environment variables
	envValues := map[string]string{
		"APP_NAME":      "CustomAuthService",
		"APP_ENV":       "production",
		"APP_VERSION":   "2.1.0",
		"APP_URL":       "https://api.example.com",
		"APP_PORT":      "9090",
		"APP_GRPC_PORT": "50053",
		"REST_PREFIX":   "/api/v2",
		"AUTH_SERVICE":  "auth.example.com:8083",
		"LOG_LEVEL":     "info",
		"LOG_PATH":      "/var/log/app",
		"DB_DRIVER":     "postgres",
		"DB_USER":       "dbuser",
		"DB_PASSWORD":   "dbpass",
		"DB_HOST":       "db.example.com",
		"DB_PORT":       "5432",
		"DB_NAME":       "meetdb",
	}

	for key, value := range envValues {
		_ = os.Setenv(key, value)
	}

	// Test with custom values
	config := Init()

	// Check custom values
	assert.Equal(t, "CustomAuthService", config.AppName)
	assert.Equal(t, "production", config.AppEnv)
	assert.Equal(t, "2.1.0", config.Version)
	assert.Equal(t, "https://api.example.com", config.URL)
	assert.Equal(t, int64(9090), config.Port)
	assert.Equal(t, int64(50053), config.GRPCPort)
	assert.Equal(t, "/api/v2", config.RestPrefix)
	assert.Equal(t, "auth.example.com:8083", config.AuthService)
	assert.Equal(t, "info", config.Log.Level)
	assert.Equal(t, "/var/log/app", config.Log.Path)
	assert.Equal(t, "postgres", config.DB.Driver)
	assert.Equal(t, "dbuser", config.DB.User)
	assert.Equal(t, "dbpass", config.DB.Password)
	assert.Equal(t, "db.example.com:5432", config.DB.Address)
	assert.Equal(t, "meetdb", config.DB.Name)
	assert.Equal(t, 25, config.DB.MaxOpenConns)
	assert.Equal(t, 25, config.DB.MaxIdleConns)
	assert.Equal(t, 5, config.DB.ConnMaxLifetime)
}

func TestGetEnv(t *testing.T) {
	// Test with existing env var
	_ = os.Setenv("TEST_VAR", "test_value")
	defer func() { _ = os.Unsetenv("TEST_VAR") }()

	result := utils.GetEnv("TEST_VAR", "fallback")
	assert.Equal(t, "test_value", result)

	// Test with non-existing env var
	result = utils.GetEnv("NON_EXISTING_VAR", "fallback")
	assert.Equal(t, "fallback", result)
}

func TestGetEnvAsInt(t *testing.T) {
	// Test with valid integer env var
	_ = os.Setenv("TEST_INT_VAR", "12345")
	defer func() { _ = os.Unsetenv("TEST_INT_VAR") }()

	result := utils.GetEnvAsInt("TEST_INT_VAR", 999)
	assert.Equal(t, int64(12345), result)

	// Test with invalid integer env var
	_ = os.Setenv("TEST_INVALID_INT_VAR", "not_a_number")
	defer func() { _ = os.Unsetenv("TEST_INVALID_INT_VAR") }()

	result = utils.GetEnvAsInt("TEST_INVALID_INT_VAR", 999)
	assert.Equal(t, int64(999), result)

	// Test with non-existing env var
	result = utils.GetEnvAsInt("NON_EXISTING_INT_VAR", 777)
	assert.Equal(t, int64(777), result)
}
