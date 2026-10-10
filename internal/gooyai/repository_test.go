package gooyai

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockRepo(t *testing.T) (Repository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewRepository(db), mock
}

func TestRepository_Get(t *testing.T) {
	repo, mock := newMockRepo(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	query := regexp.QuoteMeta(`FROM gooyai_connections WHERE organizer_uuid = ?`)

	mock.ExpectQuery(query).WithArgs("clinic-a").WillReturnRows(
		sqlmock.NewRows([]string{"organizer_uuid", "api_key_ciphertext", "key_last4", "environment", "created_at", "updated_at"}).
			AddRow("clinic-a", []byte("sealed"), "ab12", "prod", now, now))

	conn, err := repo.Get(context.Background(), "clinic-a")
	require.NoError(t, err)
	assert.Equal(t, &Connection{OrganizerUUID: "clinic-a", APIKeySealed: []byte("sealed"), KeyLast4: "ab12", Environment: "prod", CreatedAt: now, UpdatedAt: now}, conn)

	mock.ExpectQuery(query).WithArgs("nobody").WillReturnRows(
		sqlmock.NewRows([]string{"organizer_uuid", "api_key_ciphertext", "key_last4", "environment", "created_at", "updated_at"}))
	conn, err = repo.Get(context.Background(), "nobody")
	require.NoError(t, err)
	assert.Nil(t, conn, "no row = not connected, not an error")

	mock.ExpectQuery(query).WithArgs("clinic-a").WillReturnError(errors.New("db down"))
	_, err = repo.Get(context.Background(), "clinic-a")
	assert.EqualError(t, err, "db down")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Upsert(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO gooyai_connections (organizer_uuid, api_key_ciphertext, key_last4, environment)`)).
		WithArgs("clinic-a", []byte("sealed"), "ab12", "sandbox").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Upsert(context.Background(), &Connection{OrganizerUUID: "clinic-a", APIKeySealed: []byte("sealed"), KeyLast4: "ab12", Environment: "sandbox"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM gooyai_connections WHERE organizer_uuid = ?`)).
		WithArgs("clinic-a").
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.Delete(context.Background(), "clinic-a"))
	require.NoError(t, mock.ExpectationsWereMet())
}
