package availabilitytemplates

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func priceUUID(s string) *string { return &s }

func TestRepositoryCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	tmpl := &Template{
		UUID:          "tmpl-1",
		OrganizerUuid: "org1",
		Weekday:       1,
		StartTime:     "09:00:00",
		EndTime:       "13:00:00",
		EffectiveFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Active:        true,
	}

	mock.ExpectExec("INSERT INTO availability_templates \\(uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active\\) VALUES \\(\\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?\\)").
		WithArgs(tmpl.UUID, tmpl.OrganizerUuid, tmpl.Weekday, tmpl.StartTime, tmpl.EndTime, tmpl.PriceUuid, tmpl.EffectiveFrom, tmpl.EffectiveUntil, tmpl.Active).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), tmpl)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	tmpl := &Template{
		UUID:          "tmpl-1",
		Weekday:       2,
		StartTime:     "10:00:00",
		EndTime:       "14:00:00",
		EffectiveFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Active:        true,
	}

	mock.ExpectExec("UPDATE availability_templates SET weekday=\\?, start_time=\\?, end_time=\\?, price_uuid=\\?, effective_from=\\?, effective_until=\\?, active=\\? WHERE uuid=\\?").
		WithArgs(tmpl.Weekday, tmpl.StartTime, tmpl.EndTime, tmpl.PriceUuid, tmpl.EffectiveFrom, tmpl.EffectiveUntil, tmpl.Active, tmpl.UUID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Update(context.Background(), tmpl)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectExec("UPDATE availability_templates SET active = 0 WHERE uuid = \\?").
		WithArgs("tmpl-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Delete(context.Background(), "tmpl-1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByUUID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	cols := []string{"uuid", "organizer_uuid", "weekday", "start_time", "end_time", "price_uuid", "effective_from", "effective_until", "active", "created_at", "updated_at"}

	t.Run("found", func(t *testing.T) {
		mock.ExpectQuery("SELECT uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active, created_at, updated_at FROM availability_templates WHERE uuid = \\?").
			WithArgs("tmpl-1").
			WillReturnRows(sqlmock.NewRows(cols).AddRow(
				"tmpl-1", "org1", int32(1), "09:00:00", "13:00:00", nil,
				time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), nil, true, time.Now(), time.Now(),
			))

		got, err := repo.GetByUUID(context.Background(), "tmpl-1")
		require.NoError(t, err)
		assert.Equal(t, "org1", got.OrganizerUuid)
		assert.Equal(t, int32(1), got.Weekday)
		assert.Nil(t, got.EffectiveUntil)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active, created_at, updated_at FROM availability_templates WHERE uuid = \\?").
			WithArgs("missing").
			WillReturnError(sql.ErrNoRows)

		got, err := repo.GetByUUID(context.Background(), "missing")
		assert.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "availability template not found")
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryListActiveByOrganizer(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	cols := []string{"uuid", "organizer_uuid", "weekday", "start_time", "end_time", "price_uuid", "effective_from", "effective_until", "active", "created_at", "updated_at"}
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active, created_at, updated_at\\s+FROM availability_templates\\s+WHERE organizer_uuid = \\? AND active = 1 AND effective_from <= \\? AND \\(effective_until IS NULL OR effective_until >= \\?\\)").
		WithArgs("org1", to, from).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"tmpl-1", "org1", int32(1), "09:00:00", "13:00:00", priceUUID("price-1"),
			from, nil, true, time.Now(), time.Now(),
		))

	result, err := repo.ListActiveByOrganizer(context.Background(), "org1", from, to)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "tmpl-1", result[0].UUID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryOccurrences(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	t.Run("HasOccurrence true", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT\\(1\\) FROM availability_template_occurrences WHERE template_uuid = \\? AND occurrence_date = \\?").
			WithArgs("tmpl-1", "2026-08-03").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		has, err := repo.HasOccurrence(context.Background(), "tmpl-1", "2026-08-03")
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("HasOccurrence query error", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT\\(1\\) FROM availability_template_occurrences WHERE template_uuid = \\? AND occurrence_date = \\?").
			WithArgs("tmpl-1", "2026-08-03").
			WillReturnError(errors.New("db error"))

		_, err := repo.HasOccurrence(context.Background(), "tmpl-1", "2026-08-03")
		assert.Error(t, err)
	})

	t.Run("RecordOccurrence", func(t *testing.T) {
		meetUUID := "meet-1"
		mock.ExpectExec("INSERT INTO availability_template_occurrences \\(template_uuid, occurrence_date, status, meet_uuid\\) VALUES \\(\\?, \\?, \\?, \\?\\)\\s+ON DUPLICATE KEY UPDATE status = VALUES\\(status\\), meet_uuid = VALUES\\(meet_uuid\\)").
			WithArgs("tmpl-1", "2026-08-03", string(OccurrenceMaterialized), &meetUUID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.RecordOccurrence(context.Background(), "tmpl-1", "2026-08-03", OccurrenceMaterialized, &meetUUID)
		assert.NoError(t, err)
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}
