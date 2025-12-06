package meets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestRepository_HasConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	tests := []struct {
		name        string
		organizerID string
		start       time.Time
		end         time.Time
		excludeUUID string
		expected    bool
		expectError bool
		setupMock   func()
	}{
		{
			name:        "no conflict",
			organizerID: "org1",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour),
			expected:    false,
			setupMock: func() {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE organizer_id = \\? AND start_time < \\? AND end_time > \\?").
					WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
		},
		{
			name:        "has conflict",
			organizerID: "org1",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour),
			expected:    true,
			setupMock: func() {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE organizer_id = \\? AND start_time < \\? AND end_time > \\?").
					WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
		},
		{
			name:        "with exclude UUID",
			organizerID: "org1",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour),
			excludeUUID: "exclude-uuid",
			expected:    false,
			setupMock: func() {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE organizer_id = \\? AND start_time < \\? AND end_time > \\? AND uuid != \\?").
					WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg(), "exclude-uuid").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
		},
		{
			name:        "query error",
			organizerID: "org1",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour),
			expectError: true,
			setupMock: func() {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM meets WHERE organizer_id = \\? AND start_time < \\? AND end_time > \\?").
					WithArgs("org1", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(errors.New("query error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			result, err := repo.HasConflict(context.Background(), tt.organizerID, tt.start, tt.end, tt.excludeUUID)
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

func TestRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	meet := &Meet{
		UUID:         "test-uuid",
		Title:        "Test Meet",
		OrganizerID:  "org1",
		Participants: []string{"p1", "p2"},
		Start:        time.Now(),
		End:          time.Now().Add(time.Hour),
		Description:  "Test description",
		Color:        "#ffffff",
		Type:         1,
		OldPrice:     100.0,
		Discount:     10.0,
		Price:        90.0,
	}

	mock.ExpectExec("INSERT INTO meets \\(uuid, title, organizer_id, participants, start_time, end_time, description, color\\) VALUES \\(\\?, \\?, \\?, \\?, \\?, \\?, \\?, \\?\\)").
		WithArgs(meet.UUID, meet.Title, meet.OrganizerID, `["p1","p2"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), meet)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	expectedMeet := &Meet{
		ID:           "1",
		UUID:         "test-uuid",
		Title:        "Test Meet",
		OrganizerID:  "org1",
		Participants: []string{"p1", "p2"},
		Start:        time.Now(),
		End:          time.Now().Add(time.Hour),
		Description:  "Test description",
		Color:        "#ffffff",
	}

	mock.ExpectQuery("SELECT id, uuid, title, organizer_id, participants, start_time, end_time, description, color FROM meets WHERE id = \\?").
		WithArgs("1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_id", "participants", "start_time", "end_time", "description", "color"}).
			AddRow(expectedMeet.ID, expectedMeet.UUID, expectedMeet.Title, expectedMeet.OrganizerID, `["p1","p2"]`, expectedMeet.Start, expectedMeet.End, expectedMeet.Description, expectedMeet.Color))

	result, err := repo.GetByID(context.Background(), "1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedMeet.ID, result.ID)
	assert.Equal(t, expectedMeet.Title, result.Title)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	meet := &Meet{
		UUID:         "test-uuid",
		Title:        "Updated Meet",
		OrganizerID:  "org1",
		Participants: []string{"p1", "p3"},
		Start:        time.Now(),
		End:          time.Now().Add(time.Hour),
		Description:  "Updated description",
		Color:        "#000000",
	}

	mock.ExpectExec("UPDATE meets SET title=\\?, organizer_id=\\?, participants=\\?, start_time=\\?, end_time=\\?, description=\\?, color=\\? WHERE uuid=\\?").
		WithArgs(meet.Title, meet.OrganizerID, `["p1","p3"]`, sqlmock.AnyArg(), sqlmock.AnyArg(), meet.Description, meet.Color, meet.UUID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Update(context.Background(), meet)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	mock.ExpectExec("DELETE FROM meets WHERE id = \\?").
		WithArgs("1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Delete(context.Background(), "1")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_QueryMeets(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	opts := &MeetQueryOptions{
		OrganizerID: "org1",
	}

	expectedMeets := []*Meet{
		{
			ID:           "1",
			UUID:         "uuid1",
			Title:        "Meet 1",
			OrganizerID:  "org1",
			Participants: []string{"p1"},
			Start:        time.Now(),
			End:          time.Now().Add(time.Hour),
			Description:  "Desc 1",
			Color:        "#fff",
		},
	}

	mock.ExpectQuery("SELECT id, uuid, title, organizer_id, participants, start_time, end_time, description, color FROM meets WHERE organizer_id = \\?").
		WithArgs("org1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "title", "organizer_id", "participants", "start_time", "end_time", "description", "color"}).
			AddRow(expectedMeets[0].ID, expectedMeets[0].UUID, expectedMeets[0].Title, expectedMeets[0].OrganizerID, `["p1"]`, expectedMeets[0].Start, expectedMeets[0].End, expectedMeets[0].Description, expectedMeets[0].Color))

	result, err := repo.QueryMeets(context.Background(), opts)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, expectedMeets[0].Title, result[0].Title)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GenerateAvailableSlots(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	from := time.Now()
	to := from.AddDate(0, 0, 7)

	expectedSlots := []*Meet{
		{
			Title: "Available Slot 1",
			Start: from.Add(time.Hour),
			End:   from.Add(2 * time.Hour),
		},
	}

	mock.ExpectQuery("SELECT title, start_time, end_time FROM meets WHERE organizer_id = \\? AND start_time BETWEEN \\? AND \\? ORDER BY start_time ASC").
		WithArgs("org1", from, to).
		WillReturnRows(sqlmock.NewRows([]string{"title", "start_time", "end_time"}).
			AddRow(expectedSlots[0].Title, expectedSlots[0].Start, expectedSlots[0].End))

	result, err := repo.GenerateAvailableSlots(context.Background(), "org1", from, to)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, expectedSlots[0].Title, result[0].Title)

	assert.NoError(t, mock.ExpectationsWereMet())
}
