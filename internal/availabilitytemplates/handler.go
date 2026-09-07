package availabilitytemplates

import (
	"context"
	"slices"
	"time"

	"github.com/salahfarzin/logger"
	"github.com/salahfarzin/meet/internal/meets"
	pb "github.com/salahfarzin/meet/proto/availability_templates"
	"github.com/salahfarzin/meet/proto/common"
	"github.com/salahfarzin/utils/middlewares"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const errInternalServer = "Internal server error"
const dateOnlyLayout = "2006-01-02"

type handler struct {
	service      Service
	authDisabled bool
	pb.UnimplementedAvailabilityTemplateServiceServer
}

func NewHandler(service Service, authDisabled bool) *handler {
	return &handler{service: service, authDisabled: authDisabled}
}

func (h *handler) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	if req == nil || req.Template == nil {
		return nil, status.Error(codes.InvalidArgument, "template is required")
	}

	t, err := fromProto(&pb.AvailabilityTemplate{
		OrganizerUuid:  h.retrieveOrganizerUuid(ctx, req.Template.OrganizerUuid),
		Weekday:        req.Template.Weekday,
		StartTime:      req.Template.StartTime,
		EndTime:        req.Template.EndTime,
		PriceUuid:      req.Template.PriceUuid,
		EffectiveFrom:  req.Template.EffectiveFrom,
		EffectiveUntil: req.Template.EffectiveUntil,
	})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	created, err := h.service.Create(ctx, t)
	if err != nil {
		logger.FromContext(ctx).Error("failed to create availability template", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.CreateResponse{
		Status:   &common.ResponseStatus{Code: 0, Message: "success"},
		Template: toProto(created),
	}, nil
}

func (h *handler) Update(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	if req == nil || req.Template == nil || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid and template are required")
	}

	t, err := fromProto(req.Template)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	t.UUID = req.Uuid
	t.OrganizerUuid = h.retrieveOrganizerUuid(ctx, req.Template.OrganizerUuid)

	updated, err := h.service.Update(ctx, t)
	if err != nil {
		logger.FromContext(ctx).Error("failed to update availability template", zap.Error(err), zap.String("uuid", req.Uuid))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.UpdateResponse{Template: toProto(updated)}, nil
}

func (h *handler) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if req == nil || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required")
	}

	if err := h.service.Delete(ctx, req.Uuid); err != nil {
		logger.FromContext(ctx).Error("failed to delete availability template", zap.Error(err), zap.String("uuid", req.Uuid))
		return nil, status.Error(codes.Internal, errInternalServer)
	}

	return &pb.DeleteResponse{}, nil
}

func (h *handler) GetAll(ctx context.Context, req *pb.GetAllRequest) (*pb.GetAllResponse, error) {
	organizerUuid := req.GetOrganizerId()
	if organizerUuid == "" {
		organizerUuid = middlewares.GetUserFromContext(ctx).Uuid
	}

	templates, err := h.service.GetAll(ctx, organizerUuid)
	if err != nil {
		logger.FromContext(ctx).Error("failed to list availability templates", zap.Error(err))
		return nil, status.Error(codes.Internal, errInternalServer)
	}

	pbTemplates := make([]*pb.AvailabilityTemplate, 0, len(templates))
	for _, t := range templates {
		pbTemplates = append(pbTemplates, toProto(t))
	}
	return &pb.GetAllResponse{Templates: pbTemplates}, nil
}

// retrieveOrganizerUuid mirrors internal/meets/handler.go's rule: an explicit
// organizer_uuid is only honored for RoleSuperAdmin callers, everyone else is
// scoped to their own uuid. When auth is disabled (local dev / AUTH_DISABLED=true),
// every request carries the same fixed placeholder identity (see
// localAuthBypassMiddleware in cmd/api/rest.go), so falling back to it would
// stamp every locally-created template with that placeholder uuid instead of
// a real, useful organizer - honor the request's organizer_uuid instead.
func (h *handler) retrieveOrganizerUuid(ctx context.Context, organizerUuid string) string {
	if h.authDisabled && organizerUuid != "" {
		return organizerUuid
	}

	user := middlewares.GetUserFromContext(ctx)
	if slices.Contains(user.Roles, meets.RoleSuperAdmin) && organizerUuid != "" {
		return organizerUuid
	}
	return user.Uuid
}

func fromProto(t *pb.AvailabilityTemplate) (*Template, error) {
	effectiveFrom, err := time.Parse(dateOnlyLayout, t.EffectiveFrom)
	if err != nil {
		return nil, err
	}

	var effectiveUntil *time.Time
	if t.EffectiveUntil != nil && *t.EffectiveUntil != "" {
		v, err := time.Parse(dateOnlyLayout, *t.EffectiveUntil)
		if err != nil {
			return nil, err
		}
		effectiveUntil = &v
	}

	return &Template{
		OrganizerUuid:  t.OrganizerUuid,
		Weekday:        t.Weekday,
		StartTime:      normalizeTime(t.StartTime),
		EndTime:        normalizeTime(t.EndTime),
		PriceUuid:      t.PriceUuid,
		EffectiveFrom:  effectiveFrom,
		EffectiveUntil: effectiveUntil,
	}, nil
}

// normalizeTime accepts "HH:MM" or "HH:MM:SS" and always returns "HH:MM:SS",
// matching the TIME column's storage format.
func normalizeTime(hhmm string) string {
	if len(hhmm) == 5 {
		return hhmm + ":00"
	}
	return hhmm
}

func toProto(t *Template) *pb.AvailabilityTemplate {
	out := &pb.AvailabilityTemplate{
		Uuid:          t.UUID,
		OrganizerUuid: t.OrganizerUuid,
		Weekday:       t.Weekday,
		StartTime:     t.StartTime,
		EndTime:       t.EndTime,
		PriceUuid:     t.PriceUuid,
		EffectiveFrom: t.EffectiveFrom.Format(dateOnlyLayout),
		Active:        t.Active,
		CreatedAt:     t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     t.UpdatedAt.Format(time.RFC3339),
	}
	if t.EffectiveUntil != nil {
		s := t.EffectiveUntil.Format(dateOnlyLayout)
		out.EffectiveUntil = &s
	}
	return out
}
