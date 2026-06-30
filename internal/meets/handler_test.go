package meets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/salahfarzin/meet/internal/identity"
	pb "github.com/salahfarzin/meet/proto/meets"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// --- Conflict logic tests ---
type MockRepoConflict struct {
	HasConflictResult bool
}

func (m *MockRepoConflict) HasConflict(ctx context.Context, organizerUuid string, start, end time.Time, excludeUUID ...string) (bool, error) {
	return m.HasConflictResult, nil
}

func (m *MockRepoConflict) Create(ctx context.Context, meet *Meet) error          { return nil }
func (m *MockRepoConflict) GetByID(ctx context.Context, id string) (*Meet, error) { return nil, nil }
func (m *MockRepoConflict) GetByUUID(ctx context.Context, uuid string) (*Meet, error) {
	return nil, nil
}
func (m *MockRepoConflict) Update(ctx context.Context, meet *Meet) error  { return nil }
func (m *MockRepoConflict) Delete(ctx context.Context, uuid string) error { return nil }
func (m *MockRepoConflict) QueryMeets(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
	return nil, 0, nil
}
func (m *MockRepoConflict) GenerateAvailableSlots(ctx context.Context, organizerID string, from, to time.Time, priceUUID *string) ([]*Meet, error) {
	return nil, nil
}

func newServiceWithConflict(conflict bool) Service {
	return &service{repo: &MockRepoConflict{HasConflictResult: conflict}}
}

func TestHandlerCreateConflict(t *testing.T) {
	svc := newServiceWithConflict(true)
	meet := &Meet{
		OrganizerUuid: "org1",
		Start:         time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		End:           time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	_, err := svc.Create(context.Background(), meet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestHandlerCreateNoConflict(t *testing.T) {
	svc := newServiceWithConflict(false)
	meet := &Meet{
		OrganizerUuid: "org1",
		Start:         time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		End:           time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	got, err := svc.Create(context.Background(), meet)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

func TestHandlerUpdateConflict(t *testing.T) {
	svc := newServiceWithConflict(true)
	meet := &Meet{
		UUID:          "uuid1",
		OrganizerUuid: "org1",
		Start:         time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		End:           time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	_, err := svc.Update(context.Background(), meet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestHandlerUpdateNoConflict(t *testing.T) {
	svc := newServiceWithConflict(false)
	meet := &Meet{
		UUID:          "uuid1",
		OrganizerUuid: "org1",
		Start:         time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		End:           time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	got, err := svc.Update(context.Background(), meet)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

type MockService struct {
	// ListSchedulingErr, when non-nil, is returned by ListScheduling so tests
	// can exercise the error path without embedding magic strings in UUIDs.
	ListSchedulingErr error
	// ListClinicsErr, when non-nil, is returned by ListClinics so tests can
	// simulate the identity service being unreachable for admin callers.
	ListClinicsErr error
}

func (m *MockService) Create(ctx context.Context, meet *Meet) (*Meet, error) {
	if meet.Title == "" {
		return nil, errors.New("title is required")
	}
	if meet.Start.IsZero() {
		return nil, errors.New("invalid start time format")
	}
	if meet.Title == "internal-error" {
		return nil, errors.New("some internal error")
	}
	if meet.Title == "conflict" {
		return nil, errors.New("appointment conflict for this organizer and period")
	}
	meet.UUID = "mock-uuid"
	return meet, nil
}

func (m *MockService) Update(ctx context.Context, meet *Meet) (*Meet, error) {
	if meet.UUID == "" {
		return nil, errors.New("UUID is required")
	}
	if meet.Title == "internal-error" {
		return nil, errors.New("some internal error")
	}
	if meet.Title == "conflict" {
		return nil, errors.New("appointment conflict for this organizer and period")
	}
	return meet, nil
}

func (m *MockService) GetByID(ctx context.Context, id string) (*Meet, error) {
	return &Meet{ID: id, Title: "Dentist"}, nil
}
func (m *MockService) GetByUUID(ctx context.Context, uuid string) (*Meet, error) {
	if uuid == "not-found" {
		return nil, errors.New("meet not found")
	}
	if uuid == "internal-error" {
		return nil, errors.New("internal error")
	}
	if uuid == "booked" {
		now := time.Now()
		return &Meet{UUID: uuid, Title: "Dentist", Start: time.Now(), End: time.Now().Add(time.Hour), BookedAt: &now}, nil
	}
	return &Meet{UUID: uuid, Title: "Dentist", Start: time.Now(), End: time.Now().Add(time.Hour)}, nil
}

func (m *MockService) Delete(ctx context.Context, uuid string) error {
	if uuid == "error" {
		return errors.New("delete error")
	}
	return nil
}

func (m *MockService) QueryMeets(ctx context.Context, opts *MeetQueryOptions) ([]*Meet, error) {
	if opts.OrganizerUuid == "error" {
		return nil, errors.New("query error")
	}

	// Return different results based on date filters for testing
	if opts.From != nil && opts.To != nil {
		// Date range specified
		return []*Meet{
			{ID: "1", Title: "Filtered Meet", Start: *opts.From, End: *opts.To},
		}, nil
	}

	if opts.From != nil {
		// Only from date specified
		return []*Meet{
			{ID: "2", Title: "From Date Meet", Start: *opts.From},
		}, nil
	}

	if opts.To != nil {
		// Only to date specified
		return []*Meet{
			{ID: "3", Title: "To Date Meet", End: *opts.To},
		}, nil
	}

	// No date filters
	now := time.Now()
	return []*Meet{{ID: "1", Title: "Dentist", BookedAt: &now}}, nil
}

func (m *MockService) GetAvailability(ctx context.Context, organizerId string, from, to time.Time, priceUUID *string) (map[string]DateSlot, error) {
	if organizerId == "error" {
		return nil, errors.New("availability error")
	}
	return map[string]DateSlot{
		"2023-01-01": {
			Title: "Test Meet",
			Times: []TimeSlot{
				{Start: "10:00", End: "11:00", Duration: "60m"},
			},
		},
	}, nil
}

func (m *MockService) ParseStartAndEndTimes(start, end string) (startTime, endTime time.Time, err error) {
	st, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid start time format")
	}
	et, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid end time format")
	}
	return st, et, nil
}

func (m *MockService) ListScheduling(ctx context.Context, in ListSchedulingInput) (ListSchedulingResult, error) {
	// Return the injected error when the test has set ListSchedulingErr.
	if m.ListSchedulingErr != nil {
		return ListSchedulingResult{}, m.ListSchedulingErr
	}

	now := time.Now()
	// Mirror the date-range logic from the old QueryMeets mock so existing tests pass.
	if in.From != nil && in.To != nil {
		return ListSchedulingResult{
			Meets:    []*Meet{{ID: "1", Title: "Filtered Meet", Start: *in.From, End: *in.To}},
			Total:    1,
			Page:     in.Page,
			PageSize: in.PageSize,
		}, nil
	}
	if in.From != nil {
		return ListSchedulingResult{
			Meets:    []*Meet{{ID: "2", Title: "From Date Meet", Start: *in.From}},
			Total:    1,
			Page:     in.Page,
			PageSize: in.PageSize,
		}, nil
	}
	if in.To != nil {
		return ListSchedulingResult{
			Meets:    []*Meet{{ID: "3", Title: "To Date Meet", End: *in.To}},
			Total:    1,
			Page:     in.Page,
			PageSize: in.PageSize,
		}, nil
	}

	// Default result.
	return ListSchedulingResult{
		Meets:    []*Meet{{ID: "1", Title: "Dentist", BookedAt: &now}},
		Total:    1,
		Page:     in.Page,
		PageSize: in.PageSize,
	}, nil
}

func (m *MockService) ListClinics(ctx context.Context) ([]identity.Clinic, error) {
	if m.ListClinicsErr != nil {
		return nil, m.ListClinicsErr
	}
	// Return two fake clinics so admin-scoped tests have a non-empty AllowedClinics.
	return []identity.Clinic{
		{UUID: "clinic1", Name: "Clinic One"},
		{UUID: "clinic2", Name: "Clinic Two"},
	}, nil
}

func NewMockService() *MockService {
	return &MockService{}
}

func TestCreateMeet(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Create(context.Background(), &pb.CreateRequest{
		Meet: &pb.Meet{
			Title: "Dentist",
			Start: "2023-01-01T10:00:00Z",
			End:   "2023-01-01T10:30:00Z",
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(0), resp.Status.Code)
	assert.Equal(t, "success", resp.Status.Message)
	assert.Equal(t, "Dentist", resp.Meet.Title)
}

func TestCreateMeetValidationError(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Create(context.Background(), &pb.CreateRequest{
		Meet: &pb.Meet{
			Start: "2023-01-01T10:00:00Z",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "title is required")
}

func TestCreateMeetValidationNilRequest(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Create(context.Background(), nil)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "data is required")
}

func TestCreateMeetValidationNilMeet(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Create(context.Background(), &pb.CreateRequest{})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "data is required")
}

func TestCreateMeetValidationEmptyStart(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Create(context.Background(), &pb.CreateRequest{
		Meet: &pb.Meet{
			Title: "Test",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "start time is required")
}

func TestCreateMeetValidationEmptyEnd(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Create(context.Background(), &pb.CreateRequest{
		Meet: &pb.Meet{
			Title: "Test",
			Start: "2023-01-01T10:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "end time is required")
}

func TestCreateMeetInvalidTimeFormat(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Create(context.Background(), &pb.CreateRequest{
		Meet: &pb.Meet{
			Title: "Dentist",
			Start: "not-a-time",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "invalid start time format")
}

func TestCreateMeetInternalError(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Create(context.Background(), &pb.CreateRequest{
		Meet: &pb.Meet{
			Title: "internal-error",
			Start: "2023-01-01T10:00:00Z",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Internal server error")
}

func TestCreateMeetConflict(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Create(context.Background(), &pb.CreateRequest{
		Meet: &pb.Meet{
			Title: "conflict",
			Start: "2023-01-01T10:00:00Z",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "appointment conflict")
}

func TestGetAllMeets(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{OrganizerId: "any"})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Meets, 1)
	assert.Equal(t, "Dentist", resp.Meets[0].Title)
}

func TestGetAllMeetsError(t *testing.T) {
	// Set ListSchedulingErr directly on the mock so the error path is exercised
	// without relying on magic sentinel values embedded in user UUIDs.
	md := metadata.New(map[string]string{
		"x-user-roles": "User",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	svc := NewMockService()
	svc.ListSchedulingErr = errors.New("query error")
	h := NewHandler(svc)
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Internal server error")
}

func TestGetAllMeetsWithDateRange(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{
		OrganizerId: "org1",
		From:        "2026-01-01T00:00:00Z",
		To:          "2026-01-31T23:59:59Z",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Meets, 1)
	assert.Equal(t, "Filtered Meet", resp.Meets[0].Title)
}

func TestGetAllMeetsWithFromDateOnly(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{
		OrganizerId: "org1",
		From:        "2026-01-01T00:00:00Z",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Meets, 1)
	assert.Equal(t, "From Date Meet", resp.Meets[0].Title)
}

func TestGetAllMeetsWithToDateOnly(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{
		OrganizerId: "org1",
		To:          "2026-12-31T23:59:59Z",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Meets, 1)
	assert.Equal(t, "To Date Meet", resp.Meets[0].Title)
}

func TestGetAllMeetsWithInvalidFromDate(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	// Invalid date format should be ignored and query should succeed without filter
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{
		OrganizerId: "org1",
		From:        "invalid-date",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Meets, 1)
	assert.Equal(t, "Dentist", resp.Meets[0].Title) // Falls back to unfiltered query
}

func TestGetAllMeetsWithInvalidToDate(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	// Invalid date format should be ignored and query should succeed without filter
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{
		OrganizerId: "org1",
		To:          "not-a-date",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Meets, 1)
	assert.Equal(t, "Dentist", resp.Meets[0].Title) // Falls back to unfiltered query
}

func TestGetAllMeetsWithPartiallyInvalidDates(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	// Valid from, invalid to - should use only the from filter
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{
		OrganizerId: "org1",
		From:        "2026-01-01T00:00:00Z",
		To:          "invalid",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Meets, 1)
	assert.Equal(t, "From Date Meet", resp.Meets[0].Title)
}

func TestUpdateMeet(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), &pb.UpdateRequest{
		Uuid: "test-uuid",
		Meet: &pb.Meet{
			Title: "Updated Dentist",
			Start: "2023-01-01T10:00:00Z",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Updated Dentist", resp.Meet.Title)
}

func TestUpdateMeetValidationError(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), &pb.UpdateRequest{
		Meet: &pb.Meet{
			Start: "2023-01-01T10:00:00Z",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "UUID is required")
}

func TestUpdateMeetValidationNilRequest(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), nil)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "data is required")
}

func TestUpdateMeetValidationNilMeet(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), &pb.UpdateRequest{Uuid: "test"})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "data is required")
}

func TestUpdateMeetValidationEmptyTitle(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), &pb.UpdateRequest{
		Uuid: "test",
		Meet: &pb.Meet{
			Start: "2023-01-01T10:00:00Z",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "title is required")
}

func TestUpdateMeetValidationEmptyStart(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), &pb.UpdateRequest{
		Uuid: "test",
		Meet: &pb.Meet{
			Title: "Test",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "start time is required")
}

func TestUpdateMeetValidationEmptyEnd(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), &pb.UpdateRequest{
		Uuid: "test",
		Meet: &pb.Meet{
			Title: "Test",
			Start: "2023-01-01T10:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "end time is required")
}

func TestUpdateMeetInvalidTimeFormat(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), &pb.UpdateRequest{
		Uuid: "test-uuid",
		Meet: &pb.Meet{
			Title: "Dentist",
			Start: "invalid",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "invalid start time format")
}

func TestUpdateMeetInternalError(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), &pb.UpdateRequest{
		Uuid: "test-uuid",
		Meet: &pb.Meet{
			Title: "internal-error",
			Start: "2023-01-01T10:00:00Z",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Internal server error")
}

func TestUpdateMeetConflict(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Update(context.Background(), &pb.UpdateRequest{
		Uuid: "test-uuid",
		Meet: &pb.Meet{
			Title: "conflict",
			Start: "2023-01-01T10:00:00Z",
			End:   "2023-01-01T11:00:00Z",
		},
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "appointment conflict")
}

func TestGetAvailability(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.GetAvailability(context.Background(), &pb.GetAvailabilityRequest{
		Uuid: "org1",
		From: "2023-01-01",
		To:   "2023-01-07",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Dates)
}

func TestGetAvailabilityDefaultDates(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.GetAvailability(context.Background(), &pb.GetAvailabilityRequest{
		Uuid: "org1",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Dates)
}

func TestGetAvailabilityInvalidRequest(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.GetAvailability(context.Background(), &pb.GetAvailabilityRequest{})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "uuid is required")
}

func TestGetAvailabilityError(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	resp, err := h.GetAvailability(ctx, &pb.GetAvailabilityRequest{
		Uuid: "error",
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "failed to fetch availability")
}

func TestGetMeetTypes(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.GetMeetTypes(context.Background(), &pb.GetMeetTypesRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Types, 5) // All meet types
	assert.Contains(t, resp.Types, pb.MeetType_VIDEO_CALL)
}

func TestGetOneMeet(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.GetOne(context.Background(), &pb.GetOneRequest{
		Uuid: "test-uuid",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-uuid", resp.Meet.Uuid)
	assert.Equal(t, "Dentist", resp.Meet.Title)
}

func TestGetOneMeetNotFound(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.GetOne(context.Background(), &pb.GetOneRequest{
		Uuid: "not-found",
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "meet not found")
}

func TestGetOneMeetInternalError(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.GetOne(context.Background(), &pb.GetOneRequest{
		Uuid: "internal-error",
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Internal server error")
}

func TestGetOneMeetNoUUID(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.GetOne(context.Background(), &pb.GetOneRequest{})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestDeleteMeet(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Delete(context.Background(), &pb.DeleteRequest{
		Uuid: "test-uuid",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestDeleteMeetError(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Delete(context.Background(), &pb.DeleteRequest{
		Uuid: "error",
	})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Internal server error")
}

func TestDeleteMeetNoUUID(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.Delete(context.Background(), &pb.DeleteRequest{})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetOneMeetBookedAt(t *testing.T) {
	h := NewHandler(NewMockService())
	resp, err := h.GetOne(context.Background(), &pb.GetOneRequest{
		Uuid: "booked",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Meet.BookedAt)
}

func TestGetAllMeetsNonProgrammer(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "User",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	// As a non-programmer, requesting any organizer ID should be overridden by own UUID
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{OrganizerId: "other-org"})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestGetAllReturnsPaginatedEnriched verifies that GetAll maps request filters into
// ListSchedulingInput and returns enriched meets with Total/Page/PageSize populated.
func TestGetAllReturnsPaginatedEnriched(t *testing.T) {
	// Admin context so AllowedClinics comes from ListClinics (["clinic1","clinic2"]).
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "admin-user",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	svc := NewMockService()
	h := NewHandler(svc)

	resp, err := h.GetAll(ctx, &pb.GetAllRequest{
		PageSize:   50,
		NationalId: "123456789",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Total must be set (non-zero indicates ListScheduling was called, not old QueryMeets).
	assert.Equal(t, int32(1), resp.Total)
	assert.NotEmpty(t, resp.Meets)
	assert.NotEqual(t, "", resp.Meets[0].Uuid+resp.Meets[0].Title) // some field is set
}

// TestGetAllNonAdminClinicScope verifies that a non-admin caller's AllowedClinics
// is scoped to their own UUID, and requesting a clinic outside that scope returns
// PermissionDenied.
func TestGetAllNonAdminClinicScope(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "User",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())

	// Requesting a clinic the user doesn't own → PermissionDenied.
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{Clinic: "other-clinic"})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

// TestGetAllAdminDefaultsPage verifies page defaults (1/50) are applied.
func TestGetAllAdminDefaultsPage(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Admin",
		"x-user-uuid":  "admin1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())

	resp, err := h.GetAll(ctx, &pb.GetAllRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// default page=1, page_size=50
	assert.Equal(t, int32(1), resp.Page)
	assert.Equal(t, int32(50), resp.PageSize)
}

// enrichedMockService is a minimal Service implementation that returns a single
// Meet with all four identity-enriched fields populated. It is used by
// TestGetAllSerializesEnrichedIdentity to verify the proto mapping in GetAll.
type enrichedMockService struct {
	MockService
}

func (e *enrichedMockService) ListScheduling(_ context.Context, in ListSchedulingInput) (ListSchedulingResult, error) {
	now := time.Now()
	return ListSchedulingResult{
		Meets: []*Meet{
			{
				UUID:             "uuid-enriched",
				OrganizerUuid:    "org1",
				Title:            "Enriched Meet",
				Start:            now,
				End:              now.Add(time.Hour),
				FirstName:        "Ada",
				LastName:         "Lovelace",
				NationalCode:     "1234567890",
				Mobile:           "09123456789",
				ParticipantUuids: []string{"p1"},
			},
		},
		Total:    1,
		Page:     in.Page,
		PageSize: in.PageSize,
	}, nil
}

// enrichedMockServiceWithClinic extends enrichedMockService to also populate ClinicName,
// used by TestGetAllSerializesClinicName to verify the ClinicName proto mapping.
type enrichedMockServiceWithClinic struct {
	MockService
}

func (e *enrichedMockServiceWithClinic) ListScheduling(_ context.Context, in ListSchedulingInput) (ListSchedulingResult, error) {
	now := time.Now()
	return ListSchedulingResult{
		Meets: []*Meet{
			{
				UUID:             "uuid-clinic",
				OrganizerUuid:    "clinic-1",
				Title:            "Clinic Meet",
				Start:            now,
				End:              now.Add(time.Hour),
				FirstName:        "Ada",
				LastName:         "Lovelace",
				NationalCode:     "1234567890",
				Mobile:           "09123456789",
				ClinicName:       "Tehran Clinic",
				ParticipantUuids: []string{"p1"},
			},
		},
		Total:    1,
		Page:     in.Page,
		PageSize: in.PageSize,
	}, nil
}

// TestGetAllSerializesEnrichedIdentity asserts that when the service returns a
// Meet with enriched identity fields (FirstName/LastName/NationalCode/Mobile),
// those values survive the proto-mapping loop and appear in the gRPC response.
// This is a regression guard for the defect where the fields were populated in
// the Go struct but silently dropped before being written to &pb.Meet{}.
func TestGetAllSerializesEnrichedIdentity(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "User",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	svc := &enrichedMockService{}
	h := NewHandler(svc)

	resp, err := h.GetAll(ctx, &pb.GetAllRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Meets, 1)

	m := resp.Meets[0]
	assert.Equal(t, "Ada", m.FirstName, "FirstName must be forwarded to the proto response")
	assert.Equal(t, "Lovelace", m.LastName, "LastName must be forwarded to the proto response")
	assert.Equal(t, "1234567890", m.NationalCode, "NationalCode must be forwarded to the proto response")
	assert.Equal(t, "09123456789", m.Mobile, "Mobile must be forwarded to the proto response")
}

// TestGetAllSerializesClinicName asserts that when the service returns a Meet with
// ClinicName populated, that value survives the proto-mapping loop in GetAll and
// appears in the gRPC response as clinic_name.
func TestGetAllSerializesClinicName(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "User",
		"x-user-uuid":  "clinic-1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	svc := &enrichedMockServiceWithClinic{}
	h := NewHandler(svc)

	resp, err := h.GetAll(ctx, &pb.GetAllRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Meets, 1)

	m := resp.Meets[0]
	assert.Equal(t, "Tehran Clinic", m.ClinicName, "ClinicName must be forwarded to the proto response")
	// Ensure patient fields still survive alongside clinic name.
	assert.Equal(t, "Ada", m.FirstName, "FirstName must not be dropped when ClinicName is set")
}

// TestGetAllAdminListClinicsError verifies that when ListClinics returns an error for
// an admin caller, GetAll degrades gracefully: it must NOT return codes.Internal.
// Instead it falls back to scoping AllowedClinics to the admin's own UUID and
// returns the meets successfully.
func TestGetAllAdminListClinicsError(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-user-roles": "Admin",
		"x-user-uuid":  "admin-uuid-1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	svc := NewMockService()
	svc.ListClinicsErr = errors.New("identity service unreachable")
	h := NewHandler(svc)

	resp, err := h.GetAll(ctx, &pb.GetAllRequest{})
	// Must succeed — the calendar must remain available when identity is down.
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// The fallback scope (user's own UUID) means ListScheduling is called with
	// AllowedClinics = ["admin-uuid-1"]. MockService.ListScheduling returns a
	// default single meet, so resp.Meets must be non-empty.
	assert.NotEmpty(t, resp.Meets)
}
