package specialties

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/salahfarzin/logger"
	"github.com/salahfarzin/meet/internal/meets"
	"github.com/salahfarzin/meet/proto/common"
	pb "github.com/salahfarzin/meet/proto/specialties"
	"github.com/salahfarzin/utils/middlewares"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const errInternalServer = "Internal server error"
const errAdminOnly = "only admins can manage specialties"

type handler struct {
	service      Service
	authDisabled bool
	pb.UnimplementedSpecialtyServiceServer
}

func NewHandler(service Service, authDisabled bool) *handler {
	return &handler{service: service, authDisabled: authDisabled}
}

// GetAll is open to every authenticated caller: trappists need the catalogue
// to fill their profile and booking clients (rawej) need it to show labels.
// Only admins may also see deactivated entries.
func (h *handler) GetAll(ctx context.Context, req *pb.GetAllRequest) (*pb.GetAllResponse, error) {
	includeInactive := req.GetIncludeInactive() && h.isAdmin(ctx)

	list, err := h.service.GetAll(ctx, Kind(req.GetKind()), includeInactive)
	if err != nil {
		if errors.Is(err, ErrInvalidKind) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		logger.FromContext(ctx).Error("failed to list specialties", zap.Error(err))
		return nil, status.Error(codes.Internal, errInternalServer)
	}

	out := make([]*pb.Specialty, 0, len(list))
	for _, s := range list {
		out = append(out, toProto(s))
	}
	return &pb.GetAllResponse{Specialties: out}, nil
}

func (h *handler) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	if !h.isAdmin(ctx) {
		return nil, status.Error(codes.PermissionDenied, errAdminOnly)
	}
	if req == nil || req.Specialty == nil {
		return nil, status.Error(codes.InvalidArgument, "specialty is required")
	}

	created, err := h.service.Create(ctx, fromProto(req.Specialty))
	if err != nil {
		logger.FromContext(ctx).Error("failed to create specialty", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.CreateResponse{
		Status:    &common.ResponseStatus{Code: 0, Message: "success"},
		Specialty: toProto(created),
	}, nil
}

func (h *handler) Update(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	if !h.isAdmin(ctx) {
		return nil, status.Error(codes.PermissionDenied, errAdminOnly)
	}
	if req == nil || req.Specialty == nil || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid and specialty are required")
	}

	s := fromProto(req.Specialty)
	s.UUID = req.Uuid

	updated, err := h.service.Update(ctx, s)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		logger.FromContext(ctx).Error("failed to update specialty", zap.Error(err), zap.String("uuid", req.Uuid))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.UpdateResponse{Specialty: toProto(updated)}, nil
}

func (h *handler) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if !h.isAdmin(ctx) {
		return nil, status.Error(codes.PermissionDenied, errAdminOnly)
	}
	if req == nil || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required")
	}

	if err := h.service.Delete(ctx, req.Uuid); err != nil {
		logger.FromContext(ctx).Error("failed to delete specialty", zap.Error(err), zap.String("uuid", req.Uuid))
		return nil, status.Error(codes.Internal, errInternalServer)
	}

	return &pb.DeleteResponse{}, nil
}

// isAdmin: the catalogue is shared by every trappist, so only tenant admins
// and superadmins edit it. With auth disabled (local dev) every caller is a
// fully-privileged placeholder, see localAuthBypassMiddleware.
func (h *handler) isAdmin(ctx context.Context) bool {
	if h.authDisabled {
		return true
	}
	roles := middlewares.GetUserFromContext(ctx).Roles
	return slices.Contains(roles, meets.RoleSuperAdmin) || slices.Contains(roles, meets.RoleAdmin)
}

func fromProto(s *pb.Specialty) *Specialty {
	return &Specialty{
		Kind:      Kind(s.Kind),
		Slug:      s.Slug,
		Names:     s.Names,
		SortOrder: s.SortOrder,
		Active:    s.Active,
	}
}

func toProto(s *Specialty) *pb.Specialty {
	return &pb.Specialty{
		Uuid:      s.UUID,
		Kind:      string(s.Kind),
		Slug:      s.Slug,
		Names:     s.Names,
		SortOrder: s.SortOrder,
		Active:    s.Active,
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
		UpdatedAt: s.UpdatedAt.Format(time.RFC3339),
	}
}
