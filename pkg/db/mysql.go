package db

import (
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func ConnectWithConfig(cfg *mysql.Config) error {
	dsn := cfg.FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open mysql: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping mysql: %w", err)
	}

	DB = db
	return nil
}

// Close closes the MySQL connection
func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
