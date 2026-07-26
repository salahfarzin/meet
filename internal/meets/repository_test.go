package meets

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryHasConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	tests := []struct {
		name          string
		organizerUuid string
		start         time.Time
		end           time.Time
		excludeUUID   string
		expected      bool
		expectError   bool
		setupMock     func()
	}{
		{
			name:          "no conflict",
			organizerUuid: "org1",
			start:         time.Now(),
			end:           time.Now().Add(time.Hour),
			expected:      false,
			setupMock: func() {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE organizer_uuid = \\? AND start_time < \\? AND end_time > \\?").
					WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
		},
		{
			name:          "has conflict",
			organizerUuid: "org1",
			start:         time.Now(),
			end:           time.Now().Add(time.Hour),
			expected:      true,
			setupMock: func() {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE organizer_uuid = \\? AND start_time < \\? AND end_time > \\?").
					WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
		},
		{
			name:          "with exclude UUID",
			organizerUuid: "org1",
			start:         time.Now(),
			end:           time.Now().Add(time.Hour),
			excludeUUID:   "exclude-uuid",
			expected:      false,
			setupMock: func() {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE organizer_uuid = \\? AND start_time < \\? AND end_time > \\? AND uuid != \\?").
					WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg(), "exclude-uuid").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
		},
		{
			name:          "query error",
			organizerUuid: "org1",
			start:         time.Now(),
			end:           time.Now().Add(time.Hour),
			expectError:   true,
			setupMock: func() {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE organizer_uuid = \\? AND start_time < \\? AND end_time > \\?").
					WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(errors.New("query error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			result, err := repo.HasConflict(context.Background(), tt.organizerUuid, tt.start, tt.end, tt.excludeUUID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	priceID := "price-uuid-1"
	meet := &Meet{
		UUID:             "test-uuid",
		Title:            "Test Meet",
		OrganizerUuid:    "org1",
		ParticipantUuids: []string{"p1", "p2"},
		Start:            time.Now(),
		End:              time.Now().Add(time.Hour),
		Description:      "Test description",
		Color:            "#ffffff",
		Type:             1,
		PriceUuid:        &priceID,
	}

	mock.ExpectExec("INSERT INTO meets \\(uuid, title, organizer_uuid, creator_uuid, participant_uuids, start_time, end_time, description, color, type, price_uuid, booked_at, settings, version, created_at\\) VALUES \\(\\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?\\)").
		WithArgs(meet.UUID, meet.Title, meet.OrganizerUuid, meet.CreatorUuid, `["p1","p2"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color, meet.Type, meet.PriceUuid, meet.BookedAt, meet.Settings, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), meet)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), meet.Version)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetOne(t *testing.T) {
	scanCols := []string{"id", "uuid", "title", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at", "settings", "version", "created_at"}

	tests := []struct {
		name    string
		byUUID  bool
		lookup  string
		queryRe string
	}{
		{name: "ByID", byUUID: false, lookup: "1", queryRe: "SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE id = \\?"},
		{name: "ByUUID", byUUID: true, lookup: "test-uuid", queryRe: "SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE uuid = \\?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			assert.NoError(t, err)
			defer db.Close()

			repo := NewRepository(db)

			priceID := "price-uuid-1"
			expectedMeet := &Meet{
				ID:               "1",
				UUID:             "test-uuid",
				Title:            "Test Meet",
				OrganizerUuid:    "org1",
				ParticipantUuids: []string{"p1", "p2"},
				Start:            time.Now(),
				End:              time.Now().Add(time.Hour),
				Description:      "Test description",
				Color:            "#ffffff",
				Type:             1,
				PriceUuid:        &priceID,
			}
			participantsJSON, err := json.Marshal(expectedMeet.ParticipantUuids)
			assert.NoError(t, err)

			mock.ExpectQuery(tt.queryRe).
				WithArgs(tt.lookup).
				WillReturnRows(sqlmock.NewRows(scanCols).
					AddRow(expectedMeet.ID, expectedMeet.UUID, expectedMeet.Title, expectedMeet.OrganizerUuid, expectedMeet.CreatorUuid, expectedMeet.PriceUuid, string(participantsJSON), expectedMeet.Start, expectedMeet.End, expectedMeet.Description, expectedMeet.Color, expectedMeet.Type, expectedMeet.BookedAt, expectedMeet.Settings, 1, expectedMeet.CreatedAt))

			var result *Meet
			if tt.byUUID {
				result, err = repo.GetByUUID(context.Background(), tt.lookup)
			} else {
				result, err = repo.GetByID(context.Background(), tt.lookup)
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, expectedMeet.Title, result.Title)
			if tt.byUUID {
				assert.Equal(t, expectedMeet.UUID, result.UUID)
			} else {
				assert.Equal(t, expectedMeet.ID, result.ID)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRepositoryGetOneInvalidParticipants(t *testing.T) {
	scanCols := []string{"id", "uuid", "title", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at", "settings", "version", "created_at"}

	tests := []struct {
		name    string
		byUUID  bool
		lookup  string
		queryRe string
	}{
		{name: "ByID", byUUID: false, lookup: "1", queryRe: "SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE id = \\?"},
		{name: "ByUUID", byUUID: true, lookup: "test-uuid", queryRe: "SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE uuid = \\?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			assert.NoError(t, err)
			defer db.Close()

			repo := NewRepository(db)

			mock.ExpectQuery(tt.queryRe).
				WithArgs(tt.lookup).
				WillReturnRows(sqlmock.NewRows(scanCols).
					AddRow("1", "test-uuid", "Test Meet", "org1", nil, nil, "invalid", time.Now(), time.Now().Add(time.Hour), "Test description", "#ffffff", 1, nil, nil, 1, time.Now()))

			var result *Meet
			if tt.byUUID {
				result, err = repo.GetByUUID(context.Background(), tt.lookup)
			} else {
				result, err = repo.GetByID(context.Background(), tt.lookup)
			}
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "failed to unmarshal participants")

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRepositoryUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	priceID := "price-uuid-updated"
	meet := &Meet{
		UUID:             "test-uuid",
		Title:            "Updated Meet",
		OrganizerUuid:    "org1",
		ParticipantUuids: []string{"p1", "p3"},
		Start:            time.Now(),
		End:              time.Now().Add(time.Hour),
		Description:      "Updated description",
		Color:            "#000000",
		Type:             2,
		PriceUuid:        &priceID,
	}

	mock.ExpectExec("UPDATE meets SET title=\\?, organizer_uuid=\\?, participant_uuids=\\?, start_time=\\?, end_time=\\?, description=\\?, color=\\?, type=\\?, price_uuid=\\?, booked_at=\\?, settings=\\?, version=version\\+1 WHERE uuid=\\? AND version=\\?").
		WithArgs(meet.Title, meet.OrganizerUuid, `["p1","p3"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color, meet.Type, meet.PriceUuid, meet.BookedAt, meet.Settings, meet.UUID, meet.Version).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Update(context.Background(), meet)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpdateVersionConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	meet := &Meet{
		UUID:    "test-uuid",
		Title:   "Updated Meet",
		Version: 3,
	}

	mock.ExpectExec("UPDATE meets SET").
		WithArgs(meet.Title, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), meet.UUID, meet.Version).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE uuid = \\?").
		WithArgs(meet.UUID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	err = repo.Update(context.Background(), meet)
	assert.ErrorIs(t, err, ErrVersionConflict)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpdateNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	meet := &Meet{
		UUID:    "missing-uuid",
		Title:   "Updated Meet",
		Version: 1,
	}

	mock.ExpectExec("UPDATE meets SET").
		WithArgs(meet.Title, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), meet.UUID, meet.Version).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE uuid = \\?").
		WithArgs(meet.UUID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err = repo.Update(context.Background(), meet)
	assert.EqualError(t, err, "meet not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectExec("DELETE FROM meets WHERE uuid = \\?").
		WithArgs("test-uuid").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Delete(context.Background(), "test-uuid")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryQueryMeets(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.Add(time.Hour)

	opts := &MeetQueryOptions{
		OrganizerUuid: "org1",
		From:          &from,
		To:            &to,
	}

	priceID := "price-uuid-1"
	expectedMeets := []*Meet{
		{
			ID:               "1",
			UUID:             "uuid1",
			Title:            "Meet 1",
			OrganizerUuid:    "org1",
			ParticipantUuids: []string{"p1"},
			Start:            time.Now(),
			End:              time.Now().Add(time.Hour),
			Description:      "Desc 1",
			Color:            "#fff",
			Type:             1,
			PriceUuid:        &priceID,
		},
	}

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = \\? AND end_time > \\? AND start_time < \\?").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow(expectedMeets[0].ID, expectedMeets[0].UUID, expectedMeets[0].Title, expectedMeets[0].OrganizerUuid, expectedMeets[0].CreatorUuid, expectedMeets[0].PriceUuid, `["p1"]`, expectedMeets[0].Start, expectedMeets[0].End, expectedMeets[0].Description, expectedMeets[0].Color, expectedMeets[0].Type, expectedMeets[0].BookedAt))

	result, _, err := repo.QueryMeets(context.Background(), opts)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, expectedMeets[0].Title, result[0].Title)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGenerateAvailableSlots(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.AddDate(0, 0, 7)

	expectedSlots := []*Meet{
		{
			UUID:  "test-uuid",
			Title: "Available Slot 1",
			Start: from.Add(time.Hour),
			End:   from.Add(2 * time.Hour),
		},
	}

	mock.ExpectQuery("SELECT uuid, title, start_time, end_time FROM meets WHERE organizer_uuid = \\? AND start_time BETWEEN \\? AND \\? AND booked_at IS NULL ORDER BY start_time ASC").
		WithArgs("org1", from, to).
		WillReturnRows(sqlmock.NewRows([]string{"uuid", "title", "start_time", "end_time"}).
			AddRow(expectedSlots[0].UUID, expectedSlots[0].Title, expectedSlots[0].Start, expectedSlots[0].End))

	result, err := repo.GenerateAvailableSlots(context.Background(), "org1", from, to, nil)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, expectedSlots[0].UUID, result[0].UUID)
	assert.Equal(t, expectedSlots[0].Title, result[0].Title)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGenerateAvailableSlotsWithPrice(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.AddDate(0, 0, 7)
	priceUUID := "price-123"

	expectedSlots := []*Meet{
		{
			UUID:  "test-uuid",
			Title: "Available Slot 1",
			Start: from.Add(time.Hour),
			End:   from.Add(2 * time.Hour),
		},
	}

	mock.ExpectQuery("SELECT uuid, title, start_time, end_time FROM meets WHERE organizer_uuid = \\? AND start_time BETWEEN \\? AND \\? AND booked_at IS NULL AND price_uuid = \\? ORDER BY start_time ASC").
		WithArgs("org1", from, to, priceUUID).
		WillReturnRows(sqlmock.NewRows([]string{"uuid", "title", "start_time", "end_time"}).
			AddRow(expectedSlots[0].UUID, expectedSlots[0].Title, expectedSlots[0].Start, expectedSlots[0].End))

	result, err := repo.GenerateAvailableSlots(context.Background(), "org1", from, to, &priceUUID)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, expectedSlots[0].UUID, result[0].UUID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryQueryMeetsAvailability(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.AddDate(0, 0, 7)
	onlyAvailable := true

	expectedSlots := []*Meet{
		{
			UUID:  "test-uuid",
			Title: "Available Slot 1",
			Start: from.Add(time.Hour),
			End:   from.Add(2 * time.Hour),
		},
	}

	mock.ExpectQuery("SELECT uuid, title, start_time, end_time FROM meets WHERE organizer_uuid = \\? AND start_time BETWEEN \\? AND \\? AND booked_at IS NULL ORDER BY start_time ASC").
		WithArgs("org1", from, to).
		WillReturnRows(sqlmock.NewRows([]string{"uuid", "title", "start_time", "end_time"}).
			AddRow(expectedSlots[0].UUID, expectedSlots[0].Title, expectedSlots[0].Start, expectedSlots[0].End))

	result, _, err := repo.QueryMeets(context.Background(), &MeetQueryOptions{
		OrganizerUuid: "org1",
		OnlyAvailable: &onlyAvailable,
		From:          &from,
		To:            &to,
	})

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, expectedSlots[0].UUID, result[0].UUID)
	assert.Equal(t, expectedSlots[0].Title, result[0].Title)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryCreateError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	meet := &Meet{
		UUID:             "test-uuid",
		Title:            "Test Meet",
		OrganizerUuid:    "org1",
		ParticipantUuids: []string{"p1", "p2"},
		Start:            time.Now(),
		End:              time.Now().Add(time.Hour),
		Description:      "Test description",
		Color:            "#ffffff",
	}

	mock.ExpectExec("INSERT INTO meets \\(uuid, title, organizer_uuid, creator_uuid, participant_uuids, start_time, end_time, description, color, type, price_uuid, booked_at, settings, version, created_at\\) VALUES \\(\\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?\\)").
		WithArgs(meet.UUID, meet.Title, meet.OrganizerUuid, meet.CreatorUuid, `["p1","p2"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errors.New("insert error"))

	err = repo.Create(context.Background(), meet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpdateError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	meet := &Meet{
		UUID:             "test-uuid",
		Title:            "Updated Meet",
		OrganizerUuid:    "org1",
		ParticipantUuids: []string{"p1", "p3"},
		Start:            time.Now(),
		End:              time.Now().Add(time.Hour),
		Description:      "Updated description",
		Color:            "#000000",
	}

	mock.ExpectExec("UPDATE meets SET title=\\?, organizer_uuid=\\?, participant_uuids=\\?, start_time=\\?, end_time=\\?, description=\\?, color=\\?, type=\\?, price_uuid=\\?, booked_at=\\?, settings=\\?, version=version\\+1 WHERE uuid=\\? AND version=\\?").
		WithArgs(meet.Title, meet.OrganizerUuid, `["p1","p3"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), meet.UUID, meet.Version).
		WillReturnError(errors.New("update error"))

	err = repo.Update(context.Background(), meet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryQueryMeetsNoOrganizerUuid(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	result, _, err := repo.QueryMeets(context.Background(), &MeetQueryOptions{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "OrganizerUuid is required")
}

func TestRepositoryQueryMeetsNilOptions(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	result, _, err := repo.QueryMeets(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "OrganizerUuid is required")
}

func TestRepositoryQueryMeetsQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.Add(time.Hour)

	opts := &MeetQueryOptions{
		OrganizerUuid: "org1",
		From:          &from,
		To:            &to,
	}

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = \\? AND end_time > \\? AND start_time < \\?").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errors.New("query error"))

	result, _, err := repo.QueryMeets(context.Background(), opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByIDNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE id = \\?").
		WithArgs("999").
		WillReturnError(sql.ErrNoRows)

	result, err := repo.GetByID(context.Background(), "999")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "meet not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByIDQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE id = \\?").
		WithArgs("1").
		WillReturnError(errors.New("database error"))

	result, err := repo.GetByID(context.Background(), "1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryProcessRowsScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.Add(time.Hour)

	opts := &MeetQueryOptions{
		OrganizerUuid: "org1",
		From:          &from,
		To:            &to,
	}

	// Return invalid data to trigger scan error
	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = \\? AND end_time > \\? AND start_time < \\?").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow("1", "uuid1", "Meet 1", "org1", nil, nil, `["p1"]`, "invalid-time", "invalid-time", "Desc", "#fff", 1, nil))

	result, _, err := repo.QueryMeets(context.Background(), opts)
	assert.Error(t, err)
	assert.Nil(t, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryProcessRowsUnmarshalError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.Add(time.Hour)

	opts := &MeetQueryOptions{
		OrganizerUuid: "org1",
		From:          &from,
		To:            &to,
	}

	// Return invalid JSON to trigger unmarshal error
	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = \\? AND end_time > \\? AND start_time < \\?").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow("1", "uuid1", "Meet 1", "org1", nil, nil, "invalid-json", time.Now(), time.Now().Add(time.Hour), "Desc", "#fff", 1, nil))

	result, _, err := repo.QueryMeets(context.Background(), opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to unmarshal participants")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGenerateAvailableSlotsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.AddDate(0, 0, 7)

	mock.ExpectQuery("SELECT uuid, title, start_time, end_time FROM meets WHERE organizer_uuid = \\? AND start_time BETWEEN \\? AND \\? AND booked_at IS NULL ORDER BY start_time ASC").
		WithArgs("org1", from, to).
		WillReturnError(errors.New("query error"))

	result, err := repo.GenerateAvailableSlots(context.Background(), "org1", from, to, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryMeetsScopesAndPaginates(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewRepository(db)

	mock.ExpectQuery("SELECT COUNT").
		WithArgs("clinic-1").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectQuery("SELECT .* FROM meets").
		WithArgs("clinic-1", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "uuid", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids",
			"type", "title", "start_time", "end_time", "color", "description", "booked_at", "settings", "version", "created_at",
		}).AddRow(1, "m1", "clinic-1", nil, nil, "[\"p1\"]", 1, "T", time.Now(), time.Now(), "", "", nil, nil, 1, time.Now()))

	rows, total, err := repo.QueryMeets(context.Background(), &MeetQueryOptions{
		OrganizerUuids: []string{"clinic-1"}, Page: 1, PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rows, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryMeetsPaginatedRowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewRepository(db)

	iterErr := errors.New("iteration error")

	mock.ExpectQuery("SELECT COUNT").
		WithArgs("clinic-1").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))

	// Return rows that produce an error after iteration via RowError.
	dataRows := sqlmock.NewRows([]string{
		"id", "uuid", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids",
		"type", "title", "start_time", "end_time", "color", "description", "booked_at", "settings", "version", "created_at",
	}).AddRow(1, "m1", "clinic-1", nil, nil, `["p1"]`, 1, "T", time.Now(), time.Now(), "", "", nil, nil, 1, time.Now()).
		RowError(0, iterErr)

	mock.ExpectQuery("SELECT .* FROM meets").
		WithArgs("clinic-1", 10, 0).
		WillReturnRows(dataRows)

	_, _, err = repo.QueryMeets(context.Background(), &MeetQueryOptions{
		OrganizerUuids: []string{"clinic-1"}, Page: 1, PageSize: 10,
	})
	require.Error(t, err)
	assert.Equal(t, iterErr, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestQueryMeetsPaginatedWithFiltersAndDefaults exercises the From/To and
// ParticipantUuids WHERE-clause branches, plus the Page<=0/PageSize<=0 defaulting.
func TestQueryMeetsPaginatedWithFiltersAndDefaults(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewRepository(db)

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectQuery("SELECT .* FROM meets").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "uuid", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids",
			"type", "title", "start_time", "end_time", "color", "description", "booked_at", "settings", "version", "created_at",
		}).AddRow(1, "m1", "clinic-1", nil, nil, `["p1"]`, 1, "T", time.Now(), time.Now(), "", "", nil, nil, 1, time.Now()))

	rows, total, err := repo.QueryMeets(context.Background(), &MeetQueryOptions{
		OrganizerUuids:   []string{"clinic-1"},
		From:             &from,
		To:               &to,
		ParticipantUuids: []string{"p1"},
		Page:             0, // defaults to 1
		PageSize:         0, // defaults to 50
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rows, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryMeetsPaginatedCountQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewRepository(db)

	mock.ExpectQuery("SELECT COUNT").WillReturnError(errors.New("count boom"))

	_, _, err = repo.QueryMeets(context.Background(), &MeetQueryOptions{
		OrganizerUuids: []string{"clinic-1"}, Page: 1, PageSize: 10,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count boom")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryMeetsPaginatedSelectQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewRepository(db)

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectQuery("SELECT .* FROM meets").WillReturnError(errors.New("select boom"))

	_, _, err = repo.QueryMeets(context.Background(), &MeetQueryOptions{
		OrganizerUuids: []string{"clinic-1"}, Page: 1, PageSize: 10,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "select boom")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestQueryMeetsPaginatedScanAndUnmarshalErrors covers scanPaginatedMeets' scan-error
// and unmarshal-error branches.
func TestQueryMeetsPaginatedScanAndUnmarshalErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewRepository(db)

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectQuery("SELECT .* FROM meets").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "uuid", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids",
			"type", "title", "start_time", "end_time", "color", "description", "booked_at", "settings", "version", "created_at",
		}).AddRow(1, "m1", "clinic-1", nil, nil, "invalid", 1, "T", time.Now(), time.Now(), "", "", nil, nil, 1, time.Now()))

	_, _, err = repo.QueryMeets(context.Background(), &MeetQueryOptions{
		OrganizerUuids: []string{"clinic-1"}, Page: 1, PageSize: 10,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal participants")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGenerateAvailableSlotsScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.AddDate(0, 0, 7)

	mock.ExpectQuery("SELECT uuid, title, start_time, end_time FROM meets WHERE organizer_uuid = \\? AND start_time BETWEEN \\? AND \\? AND booked_at IS NULL ORDER BY start_time ASC").
		WithArgs("org1", from, to).
		WillReturnRows(sqlmock.NewRows([]string{"uuid", "title", "start_time", "end_time"}).
			AddRow("uuid1", "Slot 1", "invalid-time", "invalid-time"))

	result, err := repo.GenerateAvailableSlots(context.Background(), "org1", from, to, nil)
	assert.Error(t, err)
	assert.Nil(t, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByUUIDNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE uuid = \\?").
		WithArgs("999").
		WillReturnError(sql.ErrNoRows)

	result, err := repo.GetByUUID(context.Background(), "999")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "meet not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByUUIDQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE uuid = \\?").
		WithArgs("test-uuid").
		WillReturnError(errors.New("database error"))

	result, err := repo.GetByUUID(context.Background(), "test-uuid")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByUUIDInvalidParticipants(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE uuid = \\?").
		WithArgs("test-uuid").
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at", "settings", "version", "created_at"}).
			AddRow("1", "test-uuid", "Test Meet", "org1", nil, nil, "invalid", time.Now(), time.Now().Add(time.Hour), "Test description", "#ffffff", 1, nil, nil, 1, time.Now()))

	result, err := repo.GetByUUID(context.Background(), "test-uuid")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to unmarshal participants")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryQueryMeetsAvailabilityDefaultDates(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	onlyAvailable := true

	mock.ExpectQuery("SELECT uuid, title, start_time, end_time FROM meets WHERE organizer_uuid = \\? AND start_time BETWEEN \\? AND \\? AND booked_at IS NULL ORDER BY start_time ASC").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"uuid", "title", "start_time", "end_time"}))

	result, _, err := repo.QueryMeets(context.Background(), &MeetQueryOptions{
		OrganizerUuid: "org1",
		OnlyAvailable: &onlyAvailable,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryHasConflictWithoutExclude(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE organizer_uuid = \\? AND start_time < \\? AND end_time > \\?").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	result, err := repo.HasConflict(context.Background(), "org1", time.Now(), time.Now().Add(time.Hour))
	assert.NoError(t, err)
	assert.False(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpdateWithBookedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	now := time.Now()
	meet := &Meet{
		UUID:     "test-uuid",
		Title:    "Updated",
		BookedAt: &now,
	}

	mock.ExpectExec("UPDATE meets SET").
		WithArgs(meet.Title, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), meet.BookedAt, sqlmock.AnyArg(), meet.UUID, meet.Version).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Update(context.Background(), meet)
	assert.NoError(t, err)
}

func TestRepositoryUpdateWithNilPrice(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	meet := &Meet{
		UUID:      "test-uuid",
		Title:     "Updated",
		PriceUuid: nil,
	}

	mock.ExpectExec("UPDATE meets SET").
		WithArgs(meet.Title, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), nil, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.UUID, meet.Version).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Update(context.Background(), meet)
	assert.NoError(t, err)
}

func TestRepositoryDeleteError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectExec("DELETE FROM meets WHERE uuid = ?").
		WithArgs("test-uuid").
		WillReturnError(errors.New("delete error"))

	err = repo.Delete(context.Background(), "test-uuid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryCreateLastInsertIdError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	meet := &Meet{UUID: "test"}

	mock.ExpectExec("INSERT INTO meets").
		WillReturnResult(sqlmock.NewErrorResult(errors.New("last insert id error")))

	err = repo.Create(context.Background(), meet)
	assert.NoError(t, err) // Create should still return nil if LastInsertId fails
}

func TestRepositoryQueryMeetsNoFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = \\?").
		WithArgs("org1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}))

	result, _, err := repo.QueryMeets(context.Background(), &MeetQueryOptions{OrganizerUuid: "org1"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGenerateAvailableSlotsEmptyPrice(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	price := ""

	mock.ExpectQuery("SELECT uuid, title, start_time, end_time FROM meets WHERE organizer_uuid = \\? AND start_time BETWEEN \\? AND \\? AND booked_at IS NULL ORDER BY start_time ASC").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"uuid", "title", "start_time", "end_time"}))

	result, err := repo.GenerateAvailableSlots(context.Background(), "org1", time.Now(), time.Now().Add(time.Hour), &price)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRepositoryCreateAndGetByUUIDPersistsSettingsAndCreatedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)
	settings := `{"approved_at":"2026-07-10T09:00:00Z","is_absent":false,"present_at":null}`
	meet := &Meet{
		UUID:             "uuid-1",
		Title:            "Session",
		OrganizerUuid:    "clinic-1",
		ParticipantUuids: []string{"patient-1"},
		Start:            time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC),
		End:              time.Date(2026, 7, 10, 9, 30, 0, 0, time.UTC),
		Settings:         &settings,
	}

	mock.ExpectExec("INSERT INTO meets").
		WithArgs(meet.UUID, meet.Title, meet.OrganizerUuid, meet.CreatorUuid, sqlmock.AnyArg(), meet.Start.UTC(), meet.End.UTC(),
			meet.Description, meet.Color, meet.Type, meet.PriceUuid, meet.BookedAt, settings, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), meet)
	assert.NoError(t, err)

	createdAt := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "uuid", "title", "organizer_uuid", "creator_uuid", "price_uuid", "participant_uuids",
		"start_time", "end_time", "description", "color", "type", "booked_at", "settings", "version", "created_at",
	}).AddRow(
		1, "uuid-1", "Session", "clinic-1", nil, nil, `["patient-1"]`,
		meet.Start.UTC(), meet.End.UTC(), "", "", 0, nil, settings, 1, createdAt,
	)
	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, version, created_at FROM meets WHERE uuid = \\?").
		WithArgs("uuid-1").
		WillReturnRows(rows)

	got, err := repo.GetByUUID(context.Background(), "uuid-1")
	assert.NoError(t, err)
	assert.Equal(t, &settings, got.Settings)
	assert.Equal(t, createdAt, got.CreatedAt)
}
