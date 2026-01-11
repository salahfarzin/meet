package meets

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/salahfarzin/logger"
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
	Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error)
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
		Title:            req.Meet.Title,
		OrganizerUuid:    retrieveOrganizerUuid(ctx, req.Meet.OrganizerUuid),
		PriceUuid:        req.Meet.PriceUuid,
		ParticipantUuids: req.Meet.ParticipantUuids,
		Start:            startTime,
		End:              endTime,
		Description:      req.Meet.Description,
		Color:            req.Meet.Color,
		Type:             int32(req.Meet.Type),
	})
	if err != nil {
		if err.Error() == "appointment conflict for this organizer and period" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		logger.FromContext(ctx).Error("failed to create meet", zap.Error(err))
		return nil, status.Error(codes.Internal, "Internal server error")
	}

	return &pb.CreateResponse{
		Status: &common.ResponseStatus{Code: 0, Message: "success"},
		Meet: &pb.Meet{
			Uuid:             meet.UUID,
			OrganizerUuid:    meet.OrganizerUuid,
			PriceUuid:        meet.PriceUuid,
			ParticipantUuids: meet.ParticipantUuids,
			Title:            meet.Title,
			Start:            meet.Start.Format(time.RFC3339),
			End:              meet.End.Format(time.RFC3339),
			Color:            meet.Color,
			Description:      meet.Description,
			Type:             pb.MeetType(meet.Type),
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
		UUID:             req.Uuid,
		OrganizerUuid:    retrieveOrganizerUuid(ctx, req.Meet.OrganizerUuid),
		PriceUuid:        req.Meet.PriceUuid,
		ParticipantUuids: req.Meet.ParticipantUuids,
		Title:            req.Meet.Title,
		Start:            startTime,
		End:              endTime,
		Color:            req.Meet.Color,
		Description:      req.Meet.Description,
		Type:             int32(req.Meet.Type),
	})
	if err != nil {
		if err.Error() == "appointment conflict for this organizer and period" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		// Log the internal error with trace context
		logger.FromContext(ctx).Error("failed to update meet", zap.Error(err), zap.String("uuid", req.Uuid))
		return nil, status.Error(codes.Internal, "Internal server error")
	}

	return &pb.UpdateResponse{
		Meet: &pb.Meet{
			Uuid:             meet.UUID,
			OrganizerUuid:    meet.OrganizerUuid,
			PriceUuid:        meet.PriceUuid,
			ParticipantUuids: meet.ParticipantUuids,
			Title:            meet.Title,
			Start:            meet.Start.Format(time.RFC3339),
			End:              meet.End.Format(time.RFC3339),
			Description:      meet.Description,
			Color:            meet.Color,
			Type:             pb.MeetType(meet.Type),
		},
	}, nil
}

// GetOne implements proto.MeetServiceServer.
func (h *handler) GetOne(ctx context.Context, req *pb.GetOneRequest) (*pb.GetOneResponse, error) {
	if req == nil || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "UUID is required")
	}

	meet, err := h.service.GetByUUID(ctx, req.Uuid)
	if err != nil {
		if err.Error() == "meet not found" {
			return nil, status.Error(codes.NotFound, "meet not found")
		}
		logger.FromContext(ctx).Error("failed to fetch meet", zap.Error(err), zap.String("uuid", req.Uuid))
		return nil, status.Error(codes.Internal, "Internal server error")
	}

	return &pb.GetOneResponse{
		Meet: &pb.Meet{
			Uuid:             meet.UUID,
			OrganizerUuid:    meet.OrganizerUuid,
			PriceUuid:        meet.PriceUuid,
			ParticipantUuids: meet.ParticipantUuids,
			Title:            meet.Title,
			Description:      meet.Description,
			Start:            meet.Start.Format(time.RFC3339),
			End:              meet.End.Format(time.RFC3339),
			Color:            meet.Color,
			Type:             pb.MeetType(meet.Type),
			BookedAt: func() *string {
				if meet.BookedAt == nil {
					return nil
				}
				s := meet.BookedAt.Format(time.RFC3339)
				return &s
			}(),
		},
	}, nil
}

// Delete implements proto.MeetServiceServer.
func (h *handler) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if req == nil || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "UUID is required")
	}

	err := h.service.Delete(ctx, req.Uuid)
	if err != nil {
		logger.FromContext(ctx).Error("failed to delete meet", zap.Error(err), zap.String("uuid", req.Uuid))
		return nil, status.Error(codes.Internal, "Internal server error")
	}

	return &pb.DeleteResponse{}, nil
}

func (h *handler) GetAll(ctx context.Context, req *pb.GetAllRequest) (*pb.GetAllResponse, error) {
	opts := &MeetQueryOptions{OrganizerUuid: retrieveOrganizerUuid(ctx, req.OrganizerId)}

	// Parse optional date range filters for performance optimization
	if req.From != "" {
		fromTime, err := time.Parse(time.RFC3339, req.From)
		if err != nil {
			logger.FromContext(ctx).Warn("Invalid from date format", zap.String("from", req.From), zap.Error(err))
		} else {
			opts.From = &fromTime
		}
	}

	if req.To != "" {
		toTime, err := time.Parse(time.RFC3339, req.To)
		if err != nil {
			logger.FromContext(ctx).Warn("Invalid to date format", zap.String("to", req.To), zap.Error(err))
		} else {
			opts.To = &toTime
		}
	}

	meetsList, err := h.service.QueryMeets(ctx, opts)
	if err != nil {
		logger.FromContext(ctx).Error("failed to query meets", zap.Error(err))
		return nil, status.Error(codes.Internal, "Internal server error")
	}
	pbMeets := make([]*pb.Meet, 0, len(meetsList))
	for _, a := range meetsList {
		pbMeets = append(pbMeets, &pb.Meet{
			Uuid:             a.UUID,
			OrganizerUuid:    a.OrganizerUuid,
			PriceUuid:        a.PriceUuid,
			ParticipantUuids: a.ParticipantUuids,
			Title:            a.Title,
			Description:      a.Description,
			Start:            a.Start.Format(time.RFC3339),
			End:              a.End.Format(time.RFC3339),
			Color:            a.Color,
			Type:             pb.MeetType(a.Type),
			BookedAt: func() *string {
				if a.BookedAt == nil {
					return nil
				}
				s := a.BookedAt.Format(time.RFC3339)
				return &s
			}(),
		})
	}
	return &pb.GetAllResponse{Meets: pbMeets}, nil
}

// GetAvailability returns next 7 days of availability for a organizer user
func (h *handler) GetAvailability(ctx context.Context, req *pb.GetAvailabilityRequest) (*pb.GetAvailabilityResponse, error) {
	if req == nil || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required")
	}

	organizerID := req.Uuid

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

	priceUUID := req.PriceUuid
	datesMap, err := h.service.GetAvailability(ctx, organizerID, from, to, priceUUID)
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
				Uuid:     slot.Uuid,
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

// GetMeetTypes returns all possible meet types for an organizer
func (h *handler) GetMeetTypes(ctx context.Context, req *pb.GetMeetTypesRequest) (*pb.GetMeetTypesResponse, error) {
	types := []pb.MeetType{
		pb.MeetType_MEET_TYPE_UNSPECIFIED,
		pb.MeetType_IMMEDIATE_PHONE_CALL,
		pb.MeetType_CHAT,
		pb.MeetType_PHONE_CALL,
		pb.MeetType_VIDEO_CALL,
	}
	return &pb.GetMeetTypesResponse{Types: types}, nil
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

func retrieveOrganizerUuid(ctx context.Context, organizerUserID string) string {
	user := middlewares.GetUserFromContext(ctx)

	organizerID := ""
	if slices.Contains(user.Roles, "Programmer") {
		organizerID = organizerUserID
	}

	if organizerID == "" {
		organizerID = user.Uuid
	}

	return organizerID
}
