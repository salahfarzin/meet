package meets

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
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

	mock.ExpectExec("INSERT INTO meets \\(uuid, title, organizer_uuid, participant_uuids, start_time, end_time, description, color, type, price_uuid, booked_at\\) VALUES \\(\\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?\\)").
		WithArgs(meet.UUID, meet.Title, meet.OrganizerUuid, `["p1","p2"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color, meet.Type, meet.PriceUuid, meet.BookedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), meet)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByID(t *testing.T) {
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

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE id = \\?").
		WithArgs("1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow(expectedMeet.ID, expectedMeet.UUID, expectedMeet.Title, expectedMeet.OrganizerUuid, expectedMeet.PriceUuid, `["p1","p2"]`, expectedMeet.Start, expectedMeet.End, expectedMeet.Description, expectedMeet.Color, expectedMeet.Type, expectedMeet.BookedAt))

	result, err := repo.GetByID(context.Background(), "1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedMeet.ID, result.ID)
	assert.Equal(t, expectedMeet.Title, result.Title)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByUUID(t *testing.T) {
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

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE uuid = \\?").
		WithArgs("test-uuid").
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow(expectedMeet.ID, expectedMeet.UUID, expectedMeet.Title, expectedMeet.OrganizerUuid, expectedMeet.PriceUuid, `["p1","p2"]`, expectedMeet.Start, expectedMeet.End, expectedMeet.Description, expectedMeet.Color, expectedMeet.Type, expectedMeet.BookedAt))

	result, err := repo.GetByUUID(context.Background(), "test-uuid")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedMeet.UUID, result.UUID)
	assert.Equal(t, expectedMeet.Title, result.Title)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByIDInvalidParticipants(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE id = \\?").
		WithArgs("1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow("1", "test-uuid", "Test Meet", "org1", nil, "invalid", time.Now(), time.Now().Add(time.Hour), "Test description", "#ffffff", 1, nil))

	result, err := repo.GetByID(context.Background(), "1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to unmarshal participants")

	assert.NoError(t, mock.ExpectationsWereMet())
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

	mock.ExpectExec("UPDATE meets SET title=\\?, organizer_uuid=\\?, participant_uuids=\\?, start_time=\\?, end_time=\\?, description=\\?, color=\\?, type=\\?, price_uuid=\\?, booked_at=\\? WHERE uuid=\\?").
		WithArgs(meet.Title, meet.OrganizerUuid, `["p1","p3"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color, meet.Type, meet.PriceUuid, meet.BookedAt, meet.UUID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Update(context.Background(), meet)
	assert.NoError(t, err)

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

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = \\? AND end_time > \\? AND start_time < \\?").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow(expectedMeets[0].ID, expectedMeets[0].UUID, expectedMeets[0].Title, expectedMeets[0].OrganizerUuid, expectedMeets[0].PriceUuid, `["p1"]`, expectedMeets[0].Start, expectedMeets[0].End, expectedMeets[0].Description, expectedMeets[0].Color, expectedMeets[0].Type, expectedMeets[0].BookedAt))

	result, err := repo.QueryMeets(context.Background(), opts)
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

	result, err := repo.QueryMeets(context.Background(), &MeetQueryOptions{
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

	mock.ExpectExec("INSERT INTO meets \\(uuid, title, organizer_uuid, participant_uuids, start_time, end_time, description, color, type, price_uuid, booked_at\\) VALUES \\(\\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?\\)").
		WithArgs(meet.UUID, meet.Title, meet.OrganizerUuid, `["p1","p2"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
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

	mock.ExpectExec("UPDATE meets SET title=\\?, organizer_uuid=\\?, participant_uuids=\\?, start_time=\\?, end_time=\\?, description=\\?, color=\\?, type=\\?, price_uuid=\\?, booked_at=\\? WHERE uuid=\\?").
		WithArgs(meet.Title, meet.OrganizerUuid, `["p1","p3"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), meet.UUID).
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

	result, err := repo.QueryMeets(context.Background(), &MeetQueryOptions{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "OrganizerUuid is required")
}

func TestRepositoryQueryMeetsNilOptions(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	result, err := repo.QueryMeets(context.Background(), nil)
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

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = \\? AND end_time > \\? AND start_time < \\?").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errors.New("query error"))

	result, err := repo.QueryMeets(context.Background(), opts)
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

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE id = \\?").
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

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE id = \\?").
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
	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = \\? AND end_time > \\? AND start_time < \\?").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow("1", "uuid1", "Meet 1", "org1", nil, `["p1"]`, "invalid-time", "invalid-time", "Desc", "#fff", 1, nil))

	result, err := repo.QueryMeets(context.Background(), opts)
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
	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = \\? AND end_time > \\? AND start_time < \\?").
		WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow("1", "uuid1", "Meet 1", "org1", nil, "invalid-json", time.Now(), time.Now().Add(time.Hour), "Desc", "#fff", 1, nil))

	result, err := repo.QueryMeets(context.Background(), opts)
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

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE uuid = \\?").
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

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE uuid = \\?").
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

	mock.ExpectQuery("SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE uuid = \\?").
		WithArgs("test-uuid").
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_uuid", "price_uuid", "participant_uuids", "start_time", "end_time", "description", "color", "type", "booked_at"}).
			AddRow("1", "test-uuid", "Test Meet", "org1", nil, "invalid", time.Now(), time.Now().Add(time.Hour), "Test description", "#ffffff", 1, nil))

	result, err := repo.GetByUUID(context.Background(), "test-uuid")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to unmarshal participants")

	assert.NoError(t, mock.ExpectationsWereMet())
}
