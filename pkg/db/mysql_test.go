package db

import (
	"testing"

	"github.com/salahfarzin/meet/configs"
	"github.com/stretchr/testify/assert"
)

func TestNewMySQLStorage_InvalidConfig(t *testing.T) {
	// Test with invalid database address
	cfg := &configs.Configs{
		DB: configs.DBDriver{
			User:     "testuser",
			Password: "testpass",
			Address:  "invalid:address",
			Name:     "testdb",
		},
	}

	db, err := NewMySQLStorage(cfg)
	assert.Error(t, err)
	assert.Nil(t, db)
}

func TestNewMySQLStorage_ConnectionTimeout(t *testing.T) {
	// Test with a valid config but unreachable database
	cfg := &configs.Configs{
		DB: configs.DBDriver{
			User:     "testuser",
			Password: "testpass",
			Address:  "127.0.0.1:9999", // Unreachable port
			Name:     "testdb",
		},
	}

	db, err := NewMySQLStorage(cfg)
	assert.Error(t, err)
	assert.Nil(t, db)
}
