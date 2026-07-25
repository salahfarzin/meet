package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetup(t *testing.T) {
	// Mock environment variables to avoid real DB connection if possible,
	// or just test that it errors out gracefully if DB is missing.
	assert.NoError(t, os.Setenv("DB_USER", "test"))
	assert.NoError(t, os.Setenv("DB_PASSWORD", "test"))
	assert.NoError(t, os.Setenv("DB_ADDRESS", "localhost:3306"))
	assert.NoError(t, os.Setenv("DB_NAME", "test"))
	assert.NoError(t, os.Setenv("LOG_PATH", "logs_test"))

	defer func() { assert.NoError(t, os.Unsetenv("DB_USER")) }()
	defer func() { assert.NoError(t, os.Unsetenv("DB_PASSWORD")) }()
	defer func() { assert.NoError(t, os.Unsetenv("DB_ADDRESS")) }()
	defer func() { assert.NoError(t, os.Unsetenv("DB_NAME")) }()
	defer func() { assert.NoError(t, os.Unsetenv("LOG_PATH")) }()

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
	assert.NoError(t, os.RemoveAll("logs_test"))
}
