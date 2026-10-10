package gooyai

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Connection is a clinic's stored Gooyai connection. APIKeySealed is the
// encrypted key (see Sealer); the plaintext is never kept on this struct.
type Connection struct {
	OrganizerUUID string
	APIKeySealed  []byte
	KeyLast4      string
	Environment   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Repository stores one Connection per organizer.
type Repository interface {
	// Get returns nil, nil when the organizer has no connection.
	Get(ctx context.Context, organizerUUID string) (*Connection, error)
	Upsert(ctx context.Context, c *Connection) error
	Delete(ctx context.Context, organizerUUID string) error
}

type repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

func (repo *repository) Get(ctx context.Context, organizerUUID string) (*Connection, error) {
	row := repo.db.QueryRowContext(ctx,
		`SELECT organizer_uuid, api_key_ciphertext, key_last4, environment, created_at, updated_at
		 FROM gooyai_connections WHERE organizer_uuid = ?`, organizerUUID)

	var c Connection
	err := row.Scan(&c.OrganizerUUID, &c.APIKeySealed, &c.KeyLast4, &c.Environment, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil = not connected, distinct from a failure
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (repo *repository) Upsert(ctx context.Context, c *Connection) error {
	_, err := repo.db.ExecContext(ctx,
		`INSERT INTO gooyai_connections (organizer_uuid, api_key_ciphertext, key_last4, environment)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE api_key_ciphertext = VALUES(api_key_ciphertext),
		   key_last4 = VALUES(key_last4), environment = VALUES(environment)`,
		c.OrganizerUUID, c.APIKeySealed, c.KeyLast4, c.Environment)
	return err
}

func (repo *repository) Delete(ctx context.Context, organizerUUID string) error {
	_, err := repo.db.ExecContext(ctx, `DELETE FROM gooyai_connections WHERE organizer_uuid = ?`, organizerUUID)
	return err
}
