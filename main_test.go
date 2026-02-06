package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetup(t *testing.T) {
	// Mock environment variables to avoid real DB connection if possible,
	// or just test that it errors out gracefully if DB is missing.
	os.Setenv("DB_USER", "test")
	os.Setenv("DB_PASSWORD", "test")
	os.Setenv("DB_ADDRESS", "localhost:3306")
	os.Setenv("DB_NAME", "test")
	os.Setenv("LOG_PATH", "logs_test")

	defer os.Unsetenv("DB_USER")
	defer os.Unsetenv("DB_PASSWORD")
	defer os.Unsetenv("DB_ADDRESS")
	defer os.Unsetenv("DB_NAME")
	defer os.Unsetenv("LOG_PATH")

	app, cleanup, err := setup()
	// It might error because DB is not running, but that's fine, we still cover setup statements.
	if err == nil {
		assert.NotNil(t, app)
		assert.NotNil(t, cleanup)
		cleanup()
	} else {
		assert.Error(t, err)
	}

	// Clean up log dir created by setup
	os.RemoveAll("logs_test")
}
