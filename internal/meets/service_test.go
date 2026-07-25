package meets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/salahfarzin/meet/internal/identity"
	"github.com/stretchr/testify/assert"
)

// mockIdentityClient is a test double for identity.Client.
type mockIdentityClient struct {
	searchFunc      func(ctx context.Context, f identity.IdentityFilter) ([]string, error)
	getByUUIDsFunc  func(ctx context.Context, uuids []string) (map[string]identity.Identity, error)
	listClinicsFunc func(ctx context.Context) ([]identity.Clinic, error)
}

func (m *mockIdentityClient) Search(ctx context.Context, f identity.IdentityFilter) ([]string, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, f)
	}
	return nil, nil
}

func (m *mockIdentityClient) GetByUUIDs(ctx context.Context, uuids []string) (map[string]identity.Identity, error) {
	if m.getByUUIDsFunc != nil {
		return m.getByUUIDsFunc(ctx, uuids)
	}
	return map[string]identity.Identity{}, nil
}

func (m *mockIdentityClient) ListClinics(ctx context.Context) ([]identity.Clinic, error) {
	if m.listClinicsFunc != nil {
		return m.listClinicsFunc(ctx)
	}
	return nil, nil
}

type MockRepository struct {
	CreateFunc        func(ctx context.Context, meet *Meet) error
	GetByIDFunc       func(ctx context.Context, id string) (*Meet, error)
	GetByUUIDFunc     func(ctx context.Context, uuid string) (*Meet, error)
	UpdateFunc        func(ctx context.Context, meet *Meet) error
	DeleteFunc        func(ctx context.Context, uuid string) error
	QueryMeetsFunc    func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error)
	HasConflictFunc   func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error)
	GenerateSlotsFunc func(ctx context.Context, organizerID string, from, to time.Time, priceUUID *string) ([]*Meet, error)
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

func (m *MockRepository) QueryMeets(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
	if m.QueryMeetsFunc != nil {
		return m.QueryMeetsFunc(ctx, options)
	}
	meets := []*Meet{
		{ID: "1", Title: "Meet 1", Start: time.Now(), End: time.Now().Add(time.Hour)},
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

func TestServiceGetByID(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

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
	svc := NewService(mockRepo, nil)

	meet, err := svc.GetByID(context.Background(), "123")

	assert.Error(t, err)
	assert.Nil(t, meet)
	assert.Contains(t, err.Error(), "not found")
}

func TestServiceQueryMeets(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := NewService(mockRepo, nil)

	opts := &MeetQueryOptions{OrganizerUuid: "org1"}
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

func TestListSchedulingEmptyAllowedClinics(t *testing.T) {
	mockRepo := &MockRepository{}
	idClient := &mockIdentityClient{}
	svc := NewService(mockRepo, idClient)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{},
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Empty(t, result.Meets)
	assert.Equal(t, 0, result.Total)
}

func TestListSchedulingIdentitySearchEmptyResult(t *testing.T) {
	mockRepo := &MockRepository{}
	idClient := &mockIdentityClient{
		searchFunc: func(ctx context.Context, f identity.IdentityFilter) ([]string, error) {
			return []string{}, nil
		},
	}
	svc := NewService(mockRepo, idClient)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		FirstName:      "Ada",
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Empty(t, result.Meets)
	assert.Equal(t, 0, result.Total)
}

func TestListSchedulingIdentitySearchError(t *testing.T) {
	mockRepo := &MockRepository{}
	idClient := &mockIdentityClient{
		searchFunc: func(ctx context.Context, f identity.IdentityFilter) ([]string, error) {
			return nil, errors.New("search failed")
		},
	}
	svc := NewService(mockRepo, idClient)

	_, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		FirstName:      "Ada",
		Page:           1,
		PageSize:       10,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "search failed")
}

func TestListSchedulingEnrichesRows(t *testing.T) {
	now := time.Now()
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			return []*Meet{
				{UUID: "m1", OrganizerUuid: "clinic-1", ParticipantUuids: []string{"p1"}, Start: now, End: now.Add(time.Hour)},
			}, 1, nil
		},
	}
	idClient := &mockIdentityClient{
		getByUUIDsFunc: func(ctx context.Context, uuids []string) (map[string]identity.Identity, error) {
			// After the clinic-name enrichment change, organizer UUID is included in the same batch.
			assert.ElementsMatch(t, []string{"p1", "clinic-1"}, uuids)
			return map[string]identity.Identity{
				"p1":       {UUID: "p1", FirstName: "Ada", LastName: "Lovelace", NationalCode: "123", Mobile: "0900"},
				"clinic-1": {UUID: "clinic-1", FirstName: "Test", LastName: "Clinic"},
			}, nil
		},
	}
	svc := NewService(mockRepo, idClient)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Len(t, result.Meets, 1)
	assert.Equal(t, "Ada", result.Meets[0].FirstName)
	assert.Equal(t, "Lovelace", result.Meets[0].LastName)
	assert.Equal(t, "123", result.Meets[0].NationalCode)
	assert.Equal(t, "0900", result.Meets[0].Mobile)
}

// TestListSchedulingGetByUUIDsError verifies that when GetByUUIDs fails, ListScheduling
// still returns the meets un-enriched (no identity fields) with a nil error and
// the correct Total — the calendar must stay available even when identity is down.
func TestListSchedulingGetByUUIDsError(t *testing.T) {
	now := time.Now()
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			return []*Meet{
				{UUID: "m1", OrganizerUuid: "clinic-1", ParticipantUuids: []string{"p1"}, Start: now, End: now.Add(time.Hour)},
			}, 1, nil
		},
	}
	idClient := &mockIdentityClient{
		getByUUIDsFunc: func(ctx context.Context, uuids []string) (map[string]identity.Identity, error) {
			return nil, errors.New("identity unavailable")
		},
	}
	svc := NewService(mockRepo, idClient)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		Page:           1,
		PageSize:       10,
	})
	// Must succeed even though identity is unreachable.
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Len(t, result.Meets, 1)
	// Identity fields must be empty (un-enriched).
	assert.Equal(t, "", result.Meets[0].FirstName)
	assert.Equal(t, "", result.Meets[0].LastName)
	assert.Equal(t, "", result.Meets[0].NationalCode)
	assert.Equal(t, "", result.Meets[0].Mobile)
	assert.Equal(t, "", result.Meets[0].ClinicName)
	// Core meet fields must still be present.
	assert.Equal(t, "m1", result.Meets[0].UUID)
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

// TestListSchedulingEnrichesClinicName verifies that the organizer UUID is included
// in the same GetByUUIDs batch as patient UUIDs and that ClinicName is set on the result.
func TestListSchedulingEnrichesClinicName(t *testing.T) {
	now := time.Now()
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			return []*Meet{
				{
					UUID:             "m1",
					OrganizerUuid:    "clinic-1",
					ParticipantUuids: []string{"p1"},
					Start:            now,
					End:              now.Add(time.Hour),
				},
			}, 1, nil
		},
	}
	idClient := &mockIdentityClient{
		getByUUIDsFunc: func(ctx context.Context, uuids []string) (map[string]identity.Identity, error) {
			// Both patient and organizer UUIDs must be present in the single batch call.
			assert.ElementsMatch(t, []string{"p1", "clinic-1"}, uuids)
			return map[string]identity.Identity{
				"p1":       {UUID: "p1", FirstName: "Ada", LastName: "Lovelace", NationalCode: "123", Mobile: "0900"},
				"clinic-1": {UUID: "clinic-1", FirstName: "Tehran", LastName: "Clinic"},
			}, nil
		},
	}
	svc := NewService(mockRepo, idClient)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Len(t, result.Meets, 1)
	// Patient fields still set.
	assert.Equal(t, "Ada", result.Meets[0].FirstName)
	assert.Equal(t, "Lovelace", result.Meets[0].LastName)
	// Clinic name composed from organizer identity.
	assert.Equal(t, "Tehran Clinic", result.Meets[0].ClinicName)
}

// TestListSchedulingClinicNamePrefersNameFieldOverFirstLastName verifies that when
// the organizer identity has its `Name` field set (the real shape for a clinic/center
// record, which has no first_name/last_name), ClinicName uses Name rather than the
// empty First+Last composition.
func TestListSchedulingClinicNamePrefersNameFieldOverFirstLastName(t *testing.T) {
	now := time.Now()
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			return []*Meet{
				{
					UUID:          "m1",
					OrganizerUuid: "clinic-1",
					Start:         now,
					End:           now.Add(time.Hour),
				},
			}, 1, nil
		},
	}
	idClient := &mockIdentityClient{
		getByUUIDsFunc: func(ctx context.Context, uuids []string) (map[string]identity.Identity, error) {
			return map[string]identity.Identity{
				"clinic-1": {UUID: "clinic-1", Name: "Tehran Clinic"},
			}, nil
		},
	}
	svc := NewService(mockRepo, idClient)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-1"},
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Len(t, result.Meets, 1)
	assert.Equal(t, "Tehran Clinic", result.Meets[0].ClinicName)
}

// TestListSchedulingClinicNameOrganizerNotInIdentity verifies that if the identity
// service does not return an entry for the organizer UUID, ClinicName is left empty
// rather than causing an error.
func TestListSchedulingClinicNameOrganizerNotInIdentity(t *testing.T) {
	now := time.Now()
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			return []*Meet{
				{
					UUID:             "m2",
					OrganizerUuid:    "clinic-unknown",
					ParticipantUuids: []string{"p2"},
					Start:            now,
					End:              now.Add(time.Hour),
				},
			}, 1, nil
		},
	}
	idClient := &mockIdentityClient{
		getByUUIDsFunc: func(ctx context.Context, uuids []string) (map[string]identity.Identity, error) {
			// Only patient entry is returned; organizer is absent.
			return map[string]identity.Identity{
				"p2": {UUID: "p2", FirstName: "Grace", LastName: "Hopper"},
			}, nil
		},
	}
	svc := NewService(mockRepo, idClient)

	result, err := svc.ListScheduling(context.Background(), &ListSchedulingInput{
		AllowedClinics: []string{"clinic-unknown"},
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Len(t, result.Meets, 1)
	assert.Equal(t, "", result.Meets[0].ClinicName)
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
	idClient := &mockIdentityClient{}
	svc := NewService(mockRepo, idClient)

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

// TestListSchedulingParticipantUuidBypassesIdentitySearch verifies that when
// ParticipantUuid is set, ListScheduling uses it directly as an exact-match filter
// and never calls the identity Search pre-filter (identity client is nil here, so
// any attempt to call it would panic).
func TestListSchedulingParticipantUuidBypassesIdentitySearch(t *testing.T) {
	mockRepo := &MockRepository{
		QueryMeetsFunc: func(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
			assert.Equal(t, []string{"patient-1"}, options.ParticipantUuids)
			// Clinic scoping still applies alongside the ParticipantUuid bypass.
			assert.Equal(t, []string{"clinic-1"}, options.OrganizerUuids)
			return []*Meet{{UUID: "m1", OrganizerUuid: "clinic-1", ParticipantUuids: []string{"patient-1"}}}, 1, nil
		},
	}
	svc := NewService(mockRepo, nil) // no identity client — must not be needed for this path

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
