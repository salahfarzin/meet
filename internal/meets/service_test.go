package meets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockRepository struct {
	CreateFunc                  func(ctx context.Context, meet *Meet) error
	GetByUUIDFunc               func(ctx context.Context, uuid string) (*Meet, error)
	UpdateFunc                  func(ctx context.Context, meet *Meet) error
	DeleteFunc                  func(ctx context.Context, uuid string) error
	QueryMeetsFunc              func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error)
	QueryMeetsCursorFunc        func(ctx context.Context, options *MeetQueryOptions) (meets []*Meet, total int, nextCursor string, hasMore bool, err error)
	HasConflictFunc             func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error)
	GenerateSlotsFunc           func(ctx context.Context, organizerID string, from, to time.Time, priceUUID *string) ([]*Meet, error)
	FindParticipantBookingsFunc func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error)
}

func (m *MockRepository) Create(ctx context.Context, meet *Meet) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, meet)
	}
	return nil
}

func (m *MockRepository) GetByUUID(ctx context.Context, uuid string) (*Meet, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, uuid)
	}
	return &Meet{UUID: uuid, Title: "Test Meet"}, nil
}

func (m *MockRepository) Update(ctx context.Context, meet *Meet) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, meet)
	}
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, uuid string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, uuid)
	}
	return nil
}

func (m *MockRepository) QueryMeetsCursor(ctx context.Context, options *MeetQueryOptions) (meets []*Meet, total int, nextCursor string, hasMore bool, err error) {
	if m.QueryMeetsCursorFunc != nil {
		return m.QueryMeetsCursorFunc(ctx, options)
	}
	return []*Meet{}, 0, "", false, nil
}

func (m *MockRepository) QueryMeets(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
	if m.QueryMeetsFunc != nil {
		return m.QueryMeetsFunc(ctx, options)
	}
	meets := []*Meet{
		{UUID: "1", Title: "Meet 1", Start: time.Now(), End: time.Now().Add(time.Hour)},
	}
	return meets, len(meets), nil
}

func (m *MockRepository) HasConflict(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
	if m.HasConflictFunc != nil {
		return m.HasConflictFunc(ctx, organizerId, start, end, excludeUUID...)
	}
	return false, nil
}

func (m *MockRepository) GenerateAvailableSlots(ctx context.Context, organizerID string, from, to time.Time, priceUUID *string) ([]*Meet, error) {
	if m.GenerateSlotsFunc != nil {
		return m.GenerateSlotsFunc(ctx, organizerID, from, to, priceUUID)
	}
	return []*Meet{}, nil
}

func (m *MockRepository) FindParticipantBookings(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
	if m.FindParticipantBookingsFunc != nil {
		return m.FindParticipantBookingsFunc(ctx, participantUuids, from, to, excludeUUID)
	}
	return nil, nil
}

func TestServiceQueryMeets(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

	opts := &MeetQueryOptions{OrganizerUuids: []string{"org1"}}
	meets, err := svc.QueryMeets(context.Background(), opts)

	assert.NoError(t, err)
	assert.Len(t, meets, 1)
	assert.Equal(t, "Meet 1", meets[0].Title)
}

func TestServiceQueryMeetsError(t *testing.T) {
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			return nil, 0, errors.New("query error")
		},
	}
	svc := NewService(mockRepo, nil)

	opts := &MeetQueryOptions{}
	meets, err := svc.QueryMeets(context.Background(), opts)

	assert.Error(t, err)
	assert.Nil(t, meets)
	assert.Contains(t, err.Error(), "query error")
}

func TestServiceParseStartAndEndTimesValid(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

	startStr := "2023-01-01T10:00:00Z"
	endStr := "2023-01-01T11:00:00Z"

	start, end, err := svc.ParseStartAndEndTimes(startStr, endStr)

	assert.NoError(t, err)
	assert.Equal(t, 2023, start.Year())
	assert.Equal(t, time.January, start.Month())
	assert.Equal(t, 1, start.Day())
	assert.Equal(t, 10, start.Hour())
	assert.Equal(t, 11, end.Hour())
}

func TestServiceParseStartAndEndTimesInvalidStart(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

	startStr := "invalid"
	endStr := "2023-01-01T11:00:00Z"

	start, end, err := svc.ParseStartAndEndTimes(startStr, endStr)

	assert.Error(t, err)
	assert.True(t, start.IsZero())
	assert.True(t, end.IsZero())
	assert.Contains(t, err.Error(), "invalid start time format")
}

func TestServiceParseStartAndEndTimesInvalidEnd(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

	startStr := "2023-01-01T10:00:00Z"
	endStr := "invalid"

	start, end, err := svc.ParseStartAndEndTimes(startStr, endStr)

	assert.Error(t, err)
	assert.True(t, start.IsZero())
	assert.True(t, end.IsZero())
	assert.Contains(t, err.Error(), "invalid end time format")
}

func TestServiceGetAvailability(t *testing.T) {
	now := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			meets := []*Meet{
				{
					Title: "Meeting 1",
					Start: now.Add(time.Hour),
					End:   now.Add(2 * time.Hour),
				},
				{
					Title: "Meeting 2",
					Start: now.Add(3 * time.Hour),
					End:   now.Add(4 * time.Hour),
				},
			}
			return meets, len(meets), nil
		},
	}
	svc := NewService(mockRepo, nil)

	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)

	availability, err := svc.GetAvailability(context.Background(), "org1", from, to, nil)

	assert.NoError(t, err)
	assert.NotEmpty(t, availability)

	// Check that dates are formatted correctly
	dateStr := now.Add(time.Hour).Format("2006-01-02")
	ds, exists := availability[dateStr]
	assert.True(t, exists)
	assert.Equal(t, "Meeting 1", ds.Title)
	assert.Len(t, ds.Times, 2) // Should have both meetings on the same day
}

func TestServiceGetAvailabilityError(t *testing.T) {
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			return nil, 0, errors.New("query error")
		},
	}
	svc := NewService(mockRepo, nil)

	from := time.Now()
	to := from.Add(time.Hour)

	availability, err := svc.GetAvailability(context.Background(), "org1", from, to, nil)

	assert.Error(t, err)
	assert.Nil(t, availability)
	assert.Contains(t, err.Error(), "query error")
}

func TestServiceCreateSuccess(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, nil
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		Title:         "Test Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	result, err := svc.Create(context.Background(), meet)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.UUID)
	assert.Equal(t, "Test Meet", result.Title)
}

func TestServiceCreateConflict(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return true, nil
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		Title:         "Test Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	result, err := svc.Create(context.Background(), meet)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "appointment conflict")
}

func TestServiceCreateRepoError(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, nil
		},
		CreateFunc: func(ctx context.Context, meet *Meet) error {
			return errors.New("repo create error")
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		Title:         "Test Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	result, err := svc.Create(context.Background(), meet)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "repo create error")
}

func TestServiceUpdateSuccess(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, nil
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		UUID:          "test-uuid",
		Title:         "Updated Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	result, err := svc.Update(context.Background(), meet)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Updated Meet", result.Title)
}

func TestServiceUpdateNoUUID(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		Title:         "Updated Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	result, err := svc.Update(context.Background(), meet)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "UUID is required")
}

func TestServiceUpdateConflict(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return true, nil
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		UUID:          "test-uuid",
		Title:         "Updated Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	result, err := svc.Update(context.Background(), meet)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "appointment conflict")
}

func TestServiceCreateHasConflictError(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, errors.New("conflict check error")
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		Title:         "Test Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	result, err := svc.Create(context.Background(), meet)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "conflict check error")
}

func TestServiceUpdatePropagatesVersionConflict(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, nil
		},
		UpdateFunc: func(ctx context.Context, meet *Meet) error {
			return ErrVersionConflict
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		UUID:          "test-uuid",
		Title:         "Updated Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
		Version:       1,
	}

	result, err := svc.Update(context.Background(), meet)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrVersionConflict)
}

func TestServiceUpdateHasConflictError(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, errors.New("conflict check error")
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		UUID:          "test-uuid",
		Title:         "Updated Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	result, err := svc.Update(context.Background(), meet)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "conflict check error")
}

func TestServiceUpdateRepoError(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, nil
		},
		UpdateFunc: func(ctx context.Context, meet *Meet) error {
			return errors.New("repo update error")
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		UUID:          "test-uuid",
		Title:         "Updated Meet",
		OrganizerUuid: "org1",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	result, err := svc.Update(context.Background(), meet)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "repo update error")
}

func TestServiceGetByUUID(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

	meet, err := svc.GetByUUID(context.Background(), "uuid-123")

	assert.NoError(t, err)
	assert.NotNil(t, meet)
	assert.Equal(t, "uuid-123", meet.UUID)
	assert.Equal(t, "Test Meet", meet.Title)
}

func TestServiceGetByUUIDError(t *testing.T) {
	mockRepo := &MockRepository{
		GetByUUIDFunc: func(ctx context.Context, uuid string) (*Meet, error) {
			return nil, errors.New("not found")
		},
	}
	svc := NewService(mockRepo, nil)

	meet, err := svc.GetByUUID(context.Background(), "uuid-123")

	assert.Error(t, err)
	assert.Nil(t, meet)
	assert.Contains(t, err.Error(), "not found")
}

func TestServiceDelete(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

	err := svc.Delete(context.Background(), "uuid-123")

	assert.NoError(t, err)
}

type mockTemplateMaterializer struct {
	materializeFn func(ctx context.Context, organizerUuid string, from, to time.Time) error
	recordSkipFn  func(ctx context.Context, templateUuid string, occurrenceDate time.Time) error
}

func (m *mockTemplateMaterializer) Materialize(ctx context.Context, organizerUuid string, from, to time.Time) error {
	if m.materializeFn != nil {
		return m.materializeFn(ctx, organizerUuid, from, to)
	}
	return nil
}

func (m *mockTemplateMaterializer) RecordSkip(ctx context.Context, templateUuid string, occurrenceDate time.Time) error {
	if m.recordSkipFn != nil {
		return m.recordSkipFn(ctx, templateUuid, occurrenceDate)
	}
	return nil
}

func TestServiceDeleteRecordsTemplateSkip(t *testing.T) {
	templateUuid := "tmpl-1"
	deleted := ""
	recordedTemplate := ""
	mockRepo := &MockRepository{
		GetByUUIDFunc: func(ctx context.Context, uuid string) (*Meet, error) {
			return &Meet{UUID: uuid, TemplateUuid: &templateUuid, Start: time.Now()}, nil
		},
		DeleteFunc: func(ctx context.Context, uuid string) error {
			deleted = uuid
			return nil
		},
	}
	templates := &mockTemplateMaterializer{
		recordSkipFn: func(ctx context.Context, tmplUuid string, occurrenceDate time.Time) error {
			recordedTemplate = tmplUuid
			return nil
		},
	}
	svc := NewService(mockRepo, templates)

	err := svc.Delete(context.Background(), "meet-uuid")

	assert.NoError(t, err)
	assert.Equal(t, "meet-uuid", deleted)
	assert.Equal(t, templateUuid, recordedTemplate)
}

func TestServiceDeleteRecordSkipErrorStillDeletes(t *testing.T) {
	templateUuid := "tmpl-1"
	mockRepo := &MockRepository{
		GetByUUIDFunc: func(ctx context.Context, uuid string) (*Meet, error) {
			return &Meet{UUID: uuid, TemplateUuid: &templateUuid, Start: time.Now()}, nil
		},
	}
	templates := &mockTemplateMaterializer{
		recordSkipFn: func(ctx context.Context, tmplUuid string, occurrenceDate time.Time) error {
			return errors.New("record skip failed")
		},
	}
	svc := NewService(mockRepo, templates)

	err := svc.Delete(context.Background(), "meet-uuid")

	assert.NoError(t, err)
}

func TestServiceDeleteNoTemplateUuidSkipsRecording(t *testing.T) {
	mockRepo := &MockRepository{
		GetByUUIDFunc: func(ctx context.Context, uuid string) (*Meet, error) {
			return &Meet{UUID: uuid}, nil
		},
	}
	templates := &mockTemplateMaterializer{
		recordSkipFn: func(ctx context.Context, tmplUuid string, occurrenceDate time.Time) error {
			t.Fatal("RecordSkip should not be called when meet has no template")
			return nil
		},
	}
	svc := NewService(mockRepo, templates)

	err := svc.Delete(context.Background(), "meet-uuid")

	assert.NoError(t, err)
}

func TestServiceDeleteError(t *testing.T) {
	mockRepo := &MockRepository{
		DeleteFunc: func(ctx context.Context, uuid string) error {
			return errors.New("delete error")
		},
	}
	svc := NewService(mockRepo, nil)

	err := svc.Delete(context.Background(), "uuid-123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete error")
}

// ---- ListScheduling tests ----

func TestListSchedulingRepoQueryError(t *testing.T) {
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			return nil, 0, errors.New("db unreachable")
		},
	}
	svc := NewService(mockRepo, nil)

	_, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		Page:           1,
		PageSize:       10,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db unreachable")
}

func TestListSchedulingUsesCursorPathWhenRequested(t *testing.T) {
	var capturedOpts *MeetQueryOptions
	mockRepo := &MockRepository{
		QueryMeetsCursorFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, string, bool, error) {
			capturedOpts = options
			return []*Meet{{UUID: "m1", Title: "Cursor Meet"}}, 5, "next-token", true, nil
		},
	}
	svc := NewService(mockRepo, nil)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		UseCursor:      true,
		Cursor:         "prev-token",
		PageSize:       10,
	})
	require.NoError(t, err)
	assert.Equal(t, "Cursor Meet", result.Meets[0].Title)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, "next-token", result.NextCursor)
	assert.True(t, result.HasMore)
	require.NotNil(t, capturedOpts)
	assert.True(t, capturedOpts.UseCursor)
	assert.Equal(t, "prev-token", capturedOpts.Cursor)
}

func TestListSchedulingCursorPathRepoError(t *testing.T) {
	mockRepo := &MockRepository{
		QueryMeetsCursorFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, string, bool, error) {
			return nil, 0, "", false, errors.New("db unreachable")
		},
	}
	svc := NewService(mockRepo, nil)

	_, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		UseCursor:      true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db unreachable")
}

func TestListSchedulingOffsetPathUnaffectedByCursorFields(t *testing.T) {
	// UseCursor false (default) must still go through QueryMeetsFunc, not
	// QueryMeetsCursorFunc, even if Cursor happens to be non-empty.
	offsetCalled := false
	cursorCalled := false
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			offsetCalled = true
			return []*Meet{{UUID: "m1", Title: "Offset Meet"}}, 1, nil
		},
		QueryMeetsCursorFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, string, bool, error) {
			cursorCalled = true
			return nil, 0, "", false, nil
		},
	}
	svc := NewService(mockRepo, nil)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		Page:           1,
		PageSize:       10,
	})
	require.NoError(t, err)
	assert.True(t, offsetCalled)
	assert.False(t, cursorCalled)
	assert.Equal(t, "Offset Meet", result.Meets[0].Title)
	assert.Empty(t, result.NextCursor)
	assert.False(t, result.HasMore)
}

func TestListSchedulingEmptyAllowedClinics(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{},
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Empty(t, result.Meets)
	assert.Equal(t, 0, result.Total)
}

// TestListSchedulingClinicNotInAllowedReturnsEmpty verifies that passing a Clinic
// that is NOT in AllowedClinics returns an empty result without calling the repo.
func TestListSchedulingClinicNotInAllowedReturnsEmpty(t *testing.T) {
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			t.Fatal("repo must NOT be called when Clinic is not in AllowedClinics")
			return nil, 0, nil
		},
	}
	svc := NewService(mockRepo, nil)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		Clinic:         "clinic-other",
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Empty(t, result.Meets)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.PageSize)
}

// TestListSchedulingClinicInAllowedScopes verifies that a Clinic that IS in
// AllowedClinics causes the repo to be called with OrganizerUuids == [Clinic].
func TestListSchedulingClinicInAllowedScopes(t *testing.T) {
	var capturedOpts *MeetQueryOptions
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			capturedOpts = options
			return []*Meet{}, 0, nil
		},
	}
	svc := NewService(mockRepo, nil)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1", "clinic-2"},
		Clinic:         "clinic-2",
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, capturedOpts)
	assert.Equal(t, []string{"clinic-2"}, capturedOpts.OrganizerUuids)
}

func TestListSchedulingSingleClinicFilter(t *testing.T) {
	now := time.Now()
	var capturedOpts *MeetQueryOptions
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			capturedOpts = options
			return []*Meet{}, 0, nil
		},
	}
	svc := NewService(mockRepo, nil)

	from := now
	to := now.Add(24 * time.Hour)
	_, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		Clinic:         "clinic-1",
		From:           &from,
		To:             &to,
		Page:           2,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.NotNil(t, capturedOpts)
	assert.Equal(t, []string{"clinic-1"}, capturedOpts.OrganizerUuids)
	assert.Equal(t, 2, capturedOpts.Page)
	assert.Equal(t, 20, capturedOpts.PageSize)
}

// TestListSchedulingParticipantUuidExactMatch verifies that when ParticipantUuid
// is set, ListScheduling uses it directly as an exact-match filter.
func TestListSchedulingParticipantUuidExactMatch(t *testing.T) {
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			assert.Equal(t, []string{"patient-1"}, options.ParticipantUuids)
			// Clinic scoping still applies alongside the ParticipantUuid filter.
			assert.Equal(t, []string{"clinic-1"}, options.OrganizerUuids)
			return []*Meet{{UUID: "m1", OrganizerUuid: "clinic-1", ParticipantUuids: []string{"patient-1"}}}, 1, nil
		},
	}
	svc := NewService(mockRepo, nil)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics:  []string{"clinic-1"},
		ParticipantUuid: "patient-1",
		Page:            1,
		PageSize:        20,
	})

	assert.NoError(t, err)
	assert.Len(t, result.Meets, 1)
	assert.Equal(t, "m1", result.Meets[0].UUID)
}

// TestServiceCreateSkipsCancelledUpcomingBooking is an end-to-end regression test
// for the bug where CheckMinHoursViolated (now checkMinHoursBetweenBookings) never
// excluded cancelled participant bookings: a participant who cancelled one meet
// must still be able to book another nearby one.
func TestServiceCreateSkipsCancelledUpcomingBooking(t *testing.T) {
	cancelled := `{"participants":[{"uuid":"p1","status":"cancelled"}]}`
	settings := `{"minHoursBetweenBookings":2,"preventMultipleUpcomingBookings":true}`
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, nil
		},
		FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
			return []*Meet{{UUID: "old-cancelled", ParticipantUuids: []string{"p1"}, Settings: &cancelled}}, nil
		},
	}
	svc := NewService(mockRepo, nil)

	meet := &Meet{
		Title:            "New Meet",
		OrganizerUuid:    "org1",
		ParticipantUuids: []string{"p1"},
		Start:            time.Now().Add(24 * time.Hour),
		End:              time.Now().Add(25 * time.Hour),
		Settings:         &settings,
	}

	result, err := svc.Create(context.Background(), meet)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}
