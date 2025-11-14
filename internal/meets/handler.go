package meets

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/salahfarzin/meet/pkg/logger"
	"github.com/salahfarzin/meet/pkg/middlewares"
	"github.com/salahfarzin/meet/proto/common"
	pb "github.com/salahfarzin/meet/proto/meets"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler interface {
	Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error)
	Update(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error)
	GetOne(ctx context.Context, req *pb.GetOneRequest) (*pb.GetOneResponse, error)
}

type handler struct {
	service Service
	pb.UnimplementedMeetServiceServer
}

func NewHandler(service Service) *handler {
	return &handler{service: service}
}

// Create implements proto.MeetServiceServer.
func (h *handler) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Message)
	}

	startTime, endTime, err := h.service.ParseStartAndEndTimes(req.Meet.Start, req.Meet.End)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	meet, err := h.service.Create(ctx, &Meet{
		Title:        req.Meet.Title,
		OrganizerID:  retrieveOrganizerID(ctx, req.Meet.OrganizerId),
		Participants: req.Meet.Participants,
		Start:        startTime,
		End:          endTime,
		Description:  req.Meet.Description,
		Color:        req.Meet.Color,
	})
	if err != nil {
		logger.FromContext(ctx).Error("service create error", zap.Error(err))
		if err.Error() == "appointment conflict for this organizer and period" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "Internal server error")
	}

	return &pb.CreateResponse{
		Status: &common.ResponseStatus{Code: 0, Message: "success"},
		Meet: &pb.Meet{
			Uuid:         meet.UUID,
			OrganizerId:  meet.OrganizerID,
			Participants: meet.Participants,
			Title:        meet.Title,
			Start:        meet.Start.String(),
			End:          meet.End.String(),
			Color:        meet.Color,
			Description:  meet.Description,
		},
	}, nil
}

// Update implements proto.MeetServiceServer.
func (h *handler) Update(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	if err := validateUpdateRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Message)
	}

	startTime, endTime, err := h.service.ParseStartAndEndTimes(req.Meet.Start, req.Meet.End)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	meet, err := h.service.Update(ctx, &Meet{
		UUID:         req.Uuid,
		OrganizerID:  retrieveOrganizerID(ctx, req.Meet.OrganizerId),
		Participants: req.Meet.Participants,
		Title:        req.Meet.Title,
		Start:        startTime,
		End:          endTime,
		Color:        req.Meet.Color,
		Description:  req.Meet.Description,
	})
	if err != nil {
		logger.FromContext(ctx).Error("service update error", zap.Error(err))
		if err.Error() == "appointment conflict for this organizer and period" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "Internal server error")
	}

	return &pb.UpdateResponse{
		Meet: &pb.Meet{
			Uuid:         meet.UUID,
			OrganizerId:  meet.OrganizerID,
			Participants: meet.Participants,
			Title:        meet.Title,
			Start:        meet.Start.String(),
			End:          meet.End.String(),
			Description:  meet.Description,
		},
	}, nil
}

func (h *handler) GetAll(ctx context.Context, req *pb.GetAllRequest) (*pb.GetAllResponse, error) {
	opts := &MeetQueryOptions{OrganizerID: retrieveOrganizerID(ctx, req.OrganizerId)}
	meetsList, err := h.service.QueryMeets(ctx, opts)
	if err != nil {
		logger.FromContext(ctx).Error("DB error", zap.Error(err))
		return nil, status.Error(codes.Internal, "Internal server error")
	}
	pbMeets := make([]*pb.Meet, 0, len(meetsList))
	for _, a := range meetsList {
		var id int32
		fmt.Sscanf(a.ID, "%d", &id)
		pbMeets = append(pbMeets, &pb.Meet{
			Uuid:        a.UUID,
			Title:       a.Title,
			Description: a.Description,
			Start:       a.Start.String(),
			End:         a.End.String(),
			Color:       a.Color,
		})
	}
	return &pb.GetAllResponse{Meets: pbMeets}, nil
}

// GetAvailability returns next 7 days of availability for a organizer user
func (h *handler) GetAvailability(ctx context.Context, req *pb.GetAvailabilityRequest) (*pb.GetAvailabilityResponse, error) {
	if req == nil || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required")
	}

	organizerID := retrieveOrganizerID(ctx, req.Uuid)

	var from, to time.Time
	now := time.Now().UTC()
	if req.From == "" {
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	} else {
		from, _ = time.Parse("2006-01-02", req.From)
	}
	if req.To == "" {
		to = from.AddDate(0, 0, 6)
	} else {
		to, _ = time.Parse("2006-01-02", req.To)
	}

	datesMap, err := h.service.GetAvailability(ctx, organizerID, from, to)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to fetch availability")
	}
	// Collect and sort date keys
	dateKeys := make([]string, 0, len(datesMap))
	for date := range datesMap {
		dateKeys = append(dateKeys, date)
	}
	sort.Strings(dateKeys)

	dates := make([]*pb.DateSlot, 0, len(dateKeys))
	for _, date := range dateKeys {
		ds := datesMap[date]
		slots := make([]*pb.TimeSlot, 0)
		for _, slot := range ds.Times {
			slots = append(slots, &pb.TimeSlot{
				Start:    slot.Start,
				End:      slot.End,
				Duration: slot.Duration,
			})
		}
		t, _ := time.Parse("2006-01-02", date)
		dayName := t.Format("Mon")
		label := fmt.Sprintf("%s %s", dayName, t.Format("Jan 02, 2006"))
		dates = append(dates, &pb.DateSlot{
			Label: label,
			Value: date,
			Title: ds.Title,
			Times: slots,
		})
	}
	return &pb.GetAvailabilityResponse{Dates: dates}, nil
}

// validateCreateRequest checks required fields and returns a gRPC error if invalid
func validateCreateRequest(req *pb.CreateRequest) *common.ResponseStatus {
	if req == nil || req.Meet == nil {
		return &common.ResponseStatus{Code: http.StatusBadRequest, Message: "data is required"}
	}
	if req.Meet.Title == "" {
		return &common.ResponseStatus{Code: http.StatusBadRequest, Message: "title is required"}
	}
	if req.Meet.Start == "" {
		return &common.ResponseStatus{Code: http.StatusBadRequest, Message: "start time is required"}
	}
	if req.Meet.End == "" {
		return &common.ResponseStatus{Code: http.StatusBadRequest, Message: "end time is required"}
	}
	return nil
}

func validateUpdateRequest(req *pb.UpdateRequest) *common.ResponseStatus {
	if req == nil || req.Meet == nil {
		return &common.ResponseStatus{Code: http.StatusBadRequest, Message: "data is required"}
	}
	if req.Uuid == "" {
		return &common.ResponseStatus{Code: http.StatusBadRequest, Message: "UUID is required"}
	}
	if req.Meet.Title == "" {
		return &common.ResponseStatus{Code: http.StatusBadRequest, Message: "title is required"}
	}
	if req.Meet.Start == "" {
		return &common.ResponseStatus{Code: http.StatusBadRequest, Message: "start time is required"}
	}
	if req.Meet.End == "" {
		return &common.ResponseStatus{Code: http.StatusBadRequest, Message: "end time is required"}
	}
	return nil
}

func retrieveOrganizerID(ctx context.Context, organizerID string) string {
	user := middlewares.GetUserFromContext(ctx)

	if slices.Contains(user.Roles, "Programmer") {
		organizerID = organizerID
	}

	if organizerID == "" {
		organizerID = user.Uuid
	}

	return organizerID
}
