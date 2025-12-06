package meets

import (
	"context"
	"errors"
	"testing"
	"time"

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

func (m *MockRepoConflict) HasConflict(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
	return m.HasConflictResult, nil
}

func (m *MockRepoConflict) Create(ctx context.Context, meet *Meet) error          { return nil }
func (m *MockRepoConflict) GetByID(ctx context.Context, id string) (*Meet, error) { return nil, nil }
func (m *MockRepoConflict) Update(ctx context.Context, meet *Meet) error          { return nil }
func (m *MockRepoConflict) Delete(ctx context.Context, id string) error           { return nil }
func (m *MockRepoConflict) QueryMeets(ctx context.Context, options *MeetQueryOptions) ([]*Meet, error) {
	return nil, nil
}
func (m *MockRepoConflict) GenerateAvailableSlots(ctx context.Context, organizerID string, from, to time.Time) ([]*Meet, error) {
	return nil, nil
}

func newServiceWithConflict(conflict bool) Service {
	return &service{repo: &MockRepoConflict{HasConflictResult: conflict}}
}

func TestServiceCreateConflict(t *testing.T) {
	svc := newServiceWithConflict(true)
	meet := &Meet{
		OrganizerID: "org1",
		Start:       time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		End:         time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	_, err := svc.Create(context.Background(), meet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestServiceCreateNoConflict(t *testing.T) {
	svc := newServiceWithConflict(false)
	meet := &Meet{
		OrganizerID: "org1",
		Start:       time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		End:         time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	got, err := svc.Create(context.Background(), meet)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

func TestServiceUpdateConflict(t *testing.T) {
	svc := newServiceWithConflict(true)
	meet := &Meet{
		UUID:        "uuid1",
		OrganizerID: "org1",
		Start:       time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		End:         time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	_, err := svc.Update(context.Background(), meet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestServiceUpdateNoConflict(t *testing.T) {
	svc := newServiceWithConflict(false)
	meet := &Meet{
		UUID:        "uuid1",
		OrganizerID: "org1",
		Start:       time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		End:         time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	got, err := svc.Update(context.Background(), meet)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

type MockService struct{}

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

func (m *MockService) QueryMeets(ctx context.Context, opts *MeetQueryOptions) ([]*Meet, error) {
	if opts.OrganizerID == "error" {
		return nil, errors.New("query error")
	}
	return []*Meet{{ID: "1", Title: "Dentist"}}, nil
}

func (m *MockService) GetAvailability(ctx context.Context, organizerId string, from, to time.Time) (map[string]DateSlot, error) {
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
	md := metadata.New(map[string]string{
		"x-user-roles": "Programmer",
		"x-user-uuid":  "user1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	h := NewHandler(NewMockService())
	resp, err := h.GetAll(ctx, &pb.GetAllRequest{OrganizerId: "error"})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Internal server error")
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
