package meets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type MockRepository struct {
	CreateFunc        func(ctx context.Context, meet *Meet) error
	GetByIDFunc       func(ctx context.Context, id string) (*Meet, error)
	GetByUUIDFunc     func(ctx context.Context, uuid string) (*Meet, error)
	UpdateFunc        func(ctx context.Context, meet *Meet) error
	DeleteFunc        func(ctx context.Context, uuid string) error
	QueryMeetsFunc    func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, error)
	HasConflictFunc   func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error)
	GenerateSlotsFunc func(ctx context.Context, organizerID string, from, to time.Time) ([]*Meet, error)
}

func (m *MockRepository) Create(ctx context.Context, meet *Meet) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, meet)
	}
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, id string) (*Meet, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return &Meet{ID: id, Title: "Test Meet"}, nil
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

func (m *MockRepository) QueryMeets(ctx context.Context, options *MeetQueryOptions) ([]*Meet, error) {
	if m.QueryMeetsFunc != nil {
		return m.QueryMeetsFunc(ctx, options)
	}
	return []*Meet{
		{ID: "1", Title: "Meet 1", Start: time.Now(), End: time.Now().Add(time.Hour)},
	}, nil
}

func (m *MockRepository) HasConflict(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
	if m.HasConflictFunc != nil {
		return m.HasConflictFunc(ctx, organizerId, start, end, excludeUUID...)
	}
	return false, nil
}

func (m *MockRepository) GenerateAvailableSlots(ctx context.Context, organizerID string, from, to time.Time) ([]*Meet, error) {
	if m.GenerateSlotsFunc != nil {
		return m.GenerateSlotsFunc(ctx, organizerID, from, to)
	}
	return []*Meet{}, nil
}

func TestServiceGetByID(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo)

	meet, err := svc.GetByID(context.Background(), "123")

	assert.NoError(t, err)
	assert.NotNil(t, meet)
	assert.Equal(t, "123", meet.ID)
	assert.Equal(t, "Test Meet", meet.Title)
}

func TestServiceGetByIDError(t *testing.T) {
	mockRepo := &MockRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*Meet, error) {
			return nil, errors.New("not found")
		},
	}
	svc := NewService(mockRepo)

	meet, err := svc.GetByID(context.Background(), "123")

	assert.Error(t, err)
	assert.Nil(t, meet)
	assert.Contains(t, err.Error(), "not found")
}

func TestServiceQueryMeets(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo)

	opts := &MeetQueryOptions{OrganizerUuid: "org1"}
	meets, err := svc.QueryMeets(context.Background(), opts)

	assert.NoError(t, err)
	assert.Len(t, meets, 1)
	assert.Equal(t, "Meet 1", meets[0].Title)
}

func TestServiceQueryMeetsError(t *testing.T) {
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, error) {
			return nil, errors.New("query error")
		},
	}
	svc := NewService(mockRepo)

	opts := &MeetQueryOptions{}
	meets, err := svc.QueryMeets(context.Background(), opts)

	assert.Error(t, err)
	assert.Nil(t, meets)
	assert.Contains(t, err.Error(), "query error")
}

func TestServiceParseStartAndEndTimesValid(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo)

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
	svc := NewService(mockRepo)

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
	svc := NewService(mockRepo)

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
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, error) {
			return []*Meet{
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
			}, nil
		},
	}
	svc := NewService(mockRepo)

	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)

	availability, err := svc.GetAvailability(context.Background(), "org1", from, to)

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
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, error) {
			return nil, errors.New("query error")
		},
	}
	svc := NewService(mockRepo)

	from := time.Now()
	to := from.Add(time.Hour)

	availability, err := svc.GetAvailability(context.Background(), "org1", from, to)

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
	svc := NewService(mockRepo)

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
	svc := NewService(mockRepo)

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

func TestService_Create_RepoError(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, nil
		},
		CreateFunc: func(ctx context.Context, meet *Meet) error {
			return errors.New("repo create error")
		},
	}
	svc := NewService(mockRepo)

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
	svc := NewService(mockRepo)

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
	svc := NewService(mockRepo)

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
	svc := NewService(mockRepo)

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
	svc := NewService(mockRepo)

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

func TestServiceUpdateHasConflictError(t *testing.T) {
	mockRepo := &MockRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, errors.New("conflict check error")
		},
	}
	svc := NewService(mockRepo)

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
	svc := NewService(mockRepo)

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
