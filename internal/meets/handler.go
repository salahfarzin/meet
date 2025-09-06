package meets

import (
	"context"
	"fmt"
	"net/http"

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
	user, ok := middlewares.GetUser(ctx)
	var userID string
	if ok && user != nil {
		userID = user.ID
		// Use userID as needed
	}

	if err := validateCreateRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Message)
	}

	startTime, endTime, err := h.service.ParseStartAndEndTimes(req.Meet.Start, req.Meet.End)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	meet, err := h.service.Create(ctx, &Meet{
		Title:        req.Meet.Title,
		OrganizerID:  userID,
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
	user, ok := middlewares.GetUser(ctx)
	var userID string
	if ok && user != nil {
		userID = user.ID
		// Use userID as needed
	}

	if err := validateUpdateRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Message)
	}

	startTime, endTime, err := h.service.ParseStartAndEndTimes(req.Meet.Start, req.Meet.End)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	meet, err := h.service.Update(ctx, &Meet{
		UUID:         req.Uuid,
		OrganizerID:  userID,
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
	meets, err := h.service.GetAllByOrganizerId(ctx, req.OrganizerId)
	if err != nil {
		// log the error for internal debugging
		logger.FromContext(ctx).Error("DB error", zap.Error(err))
		// return a generic error to the client
		return nil, status.Error(codes.Internal, "Internal server error")
	}

	pbMeets := make([]*pb.Meet, 0, len(meets))
	for _, a := range meets {
		var id int32
		// Convert string ID to int32, ignore error for now or handle as needed
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
