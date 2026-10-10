package gooyai

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/salahfarzin/logger"
	"github.com/salahfarzin/meet/internal/meets"
	"github.com/salahfarzin/meet/pkg/gooyai"
	pb "github.com/salahfarzin/meet/proto/gooyai"
	"github.com/salahfarzin/utils/middlewares"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	defaultPageSize = 20
	dateLayout      = "2006-01-02"
	// localTenant is the organizer uuid localAuthBypassMiddleware injects
	// when AUTH_DISABLED=true.
	localTenant = "local"

	directionIncoming = "incoming"
	directionOutgoing = "outgoing"

	errInternalServer = "Internal server error"
)

type handler struct {
	service      Service
	authDisabled bool
	pb.UnimplementedGooyaiServiceServer
}

func NewHandler(service Service, authDisabled bool) *handler {
	return &handler{service: service, authDisabled: authDisabled}
}

// ListCalls lists the clinic's calls. Only incoming calls exist in Gooyai's
// Partner API so far; direction is the extension point for outgoing ones.
func (h *handler) ListCalls(ctx context.Context, req *pb.ListCallsRequest) (*pb.ListCallsResponse, error) {
	if err := checkDirection(req.GetDirection()); err != nil {
		return nil, err
	}
	tenant, err := h.tenant(ctx, req.GetOrganizerId())
	if err != nil {
		return nil, err
	}
	filter, err := callsFilter(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	page, err := h.service.ListIncomingCalls(ctx, tenant, &filter)
	if err != nil {
		return nil, toStatus(ctx, err, "list incoming calls")
	}

	calls := make([]*pb.Call, 0, len(page.Calls))
	for i := range page.Calls {
		calls = append(calls, callToProto(&page.Calls[i]))
	}
	return &pb.ListCallsResponse{
		Calls:      calls,
		Total:      int32(page.Pagination.Total),      //nolint:gosec // bounded by Gooyai's paging
		Page:       int32(page.Pagination.Page),       //nolint:gosec // bounded by Gooyai's paging
		PageSize:   int32(page.Pagination.Limit),      //nolint:gosec // at most gooyai.MaxLimit
		TotalPages: int32(page.Pagination.TotalPages), //nolint:gosec // bounded by Gooyai's paging
	}, nil
}

func (h *handler) GetCall(ctx context.Context, req *pb.GetCallRequest) (*pb.Call, error) {
	if err := checkDirection(req.GetDirection()); err != nil {
		return nil, err
	}
	tenant, err := h.tenant(ctx, req.GetOrganizerId())
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.GetId(), 10, 64)
	if err != nil || id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be a positive integer")
	}

	call, err := h.service.GetIncomingCall(ctx, tenant, id)
	if err != nil {
		return nil, toStatus(ctx, err, "get incoming call")
	}
	return callToProto(call), nil
}

func (h *handler) ListProviders(ctx context.Context, req *pb.ListProvidersRequest) (*pb.ListProvidersResponse, error) {
	tenant, err := h.tenant(ctx, req.GetOrganizerId())
	if err != nil {
		return nil, err
	}

	providers, err := h.service.ListProviders(ctx, tenant)
	if err != nil {
		return nil, toStatus(ctx, err, "list providers")
	}

	out := make([]*pb.Provider, 0, len(providers))
	for i := range providers {
		out = append(out, providerToProto(&providers[i]))
	}
	return &pb.ListProvidersResponse{Providers: out}, nil
}

func (h *handler) GetConnection(ctx context.Context, req *pb.GetConnectionRequest) (*pb.Connection, error) {
	tenant, err := h.tenant(ctx, req.GetOrganizerId())
	if err != nil {
		return nil, err
	}
	conn, err := h.service.GetConnection(ctx, tenant)
	if err != nil {
		return nil, toStatus(ctx, err, "get connection")
	}
	return connectionToProto(conn), nil
}

func (h *handler) SaveConnection(ctx context.Context, req *pb.SaveConnectionRequest) (*pb.Connection, error) {
	tenant, err := h.tenant(ctx, req.GetOrganizerId())
	if err != nil {
		return nil, err
	}
	conn, err := h.service.Connect(ctx, tenant, &ConnectInput{
		APIKey:      req.GetApiKey(),
		Environment: req.GetEnvironment(),
		ProxyURL:    req.GetProxyUrl(),
	})
	if err != nil {
		return nil, toStatus(ctx, err, "save connection")
	}
	return connectionToProto(conn), nil
}

func (h *handler) DeleteConnection(ctx context.Context, req *pb.DeleteConnectionRequest) (*pb.DeleteConnectionResponse, error) {
	tenant, err := h.tenant(ctx, req.GetOrganizerId())
	if err != nil {
		return nil, err
	}
	if err := h.service.Disconnect(ctx, tenant); err != nil {
		return nil, toStatus(ctx, err, "delete connection")
	}
	return &pb.DeleteConnectionResponse{}, nil
}

// tenant resolves whose Gooyai key to use, mirroring meets'
// retrieveOrganizerUuid: the caller's own uuid, unless a superadmin asks for
// another clinic. The tenant never comes from the request otherwise.
func (h *handler) tenant(ctx context.Context, requested string) (string, error) {
	if h.authDisabled {
		if requested != "" {
			return requested, nil
		}
		return localTenant, nil
	}
	user := middlewares.GetUserFromContext(ctx)
	if requested != "" && slices.Contains(user.Roles, meets.RoleSuperAdmin) {
		return requested, nil
	}
	if user.Uuid == "" {
		return "", status.Error(codes.PermissionDenied, "caller identity is missing")
	}
	return user.Uuid, nil
}

// callsFilter validates the request and builds Gooyai's filter. Validating
// here keeps Gooyai's Persian 400 messages from reaching the dashboard.
func callsFilter(req *pb.ListCallsRequest) (gooyai.ListCallsFilter, error) {
	f := gooyai.ListCallsFilter{
		PageQuery: gooyai.PageQuery{
			Page:  max(int(req.GetPage()), 1),
			Limit: int(req.GetPageSize()),
		},
		FromNumber: req.GetPhone(),
		StartDate:  req.GetFrom(),
		EndDate:    req.GetTo(),
	}
	if req.ShouldRecall != nil {
		recall := req.GetShouldRecall()
		f.ShouldRecallUser = &recall
	}
	if f.Limit <= 0 {
		f.Limit = defaultPageSize
	}
	if f.Limit > gooyai.MaxLimit {
		return f, errors.New("page_size must be at most 100")
	}
	return f, validateRange(f.StartDate, f.EndDate)
}

// validateRange checks from/to are YYYY-MM-DD and in order. Either may be empty.
func validateRange(from, to string) error {
	var fromDate, toDate time.Time
	var err error
	if from != "" {
		if fromDate, err = time.Parse(dateLayout, from); err != nil {
			return errors.New("from must be a YYYY-MM-DD date")
		}
	}
	if to != "" {
		if toDate, err = time.Parse(dateLayout, to); err != nil {
			return errors.New("to must be a YYYY-MM-DD date")
		}
	}
	if from != "" && to != "" && toDate.Before(fromDate) {
		return errors.New("from must not be after to")
	}
	return nil
}

// toStatus maps service errors to gRPC codes. Gooyai rejecting the clinic's
// key is FailedPrecondition, not Unauthenticated: the dashboard treats a 401
// as its own session expiring and would log the user out.
func toStatus(ctx context.Context, err error, op string) error {
	switch {
	case errors.Is(err, ErrNotConnected):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, ErrEncryptionNotConfigured), errors.Is(err, ErrUsersServiceNotConfigured):
		logger.FromContext(ctx).Error("phone assistant integration is not configured", zap.String("op", op), zap.Error(err))
		return status.Error(codes.FailedPrecondition, "the phone assistant integration is not set up on the server")
	case errors.Is(err, ErrKeyRejected), errors.Is(err, ErrInvalidConnection), errors.Is(err, ErrInvalidProxy):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, gooyai.ErrUnauthorized):
		logger.FromContext(ctx).Warn("gooyai rejected the clinic's API key", zap.String("op", op))
		return status.Error(codes.FailedPrecondition, "the phone assistant provider rejected this clinic's API key")
	case errors.Is(err, gooyai.ErrNotFound):
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, gooyai.ErrBadRequest):
		logger.FromContext(ctx).Warn("gooyai rejected the request", zap.String("op", op), zap.Error(err))
		return status.Error(codes.InvalidArgument, "the phone assistant provider rejected the request")
	case errors.Is(err, gooyai.ErrRateLimited):
		return status.Error(codes.ResourceExhausted, "the phone assistant provider is rate limiting, try again shortly")
	case gooyai.IsUnavailable(err):
		logger.FromContext(ctx).Error("gooyai request failed", zap.String("op", op), zap.Error(err))
		return status.Error(codes.Unavailable, "the phone assistant provider is unavailable")
	}
	// Ours, not Gooyai's: the users service, a key that won't decrypt, a decode error.
	logger.FromContext(ctx).Error("gooyai operation failed", zap.String("op", op), zap.Error(err))
	return status.Error(codes.Internal, errInternalServer)
}

// checkDirection accepts "" and "incoming". "outgoing" is a known direction
// Gooyai doesn't expose yet, so it's UNIMPLEMENTED rather than invalid.
func checkDirection(direction string) error {
	switch direction {
	case "", directionIncoming:
		return nil
	case directionOutgoing:
		return status.Error(codes.Unimplemented, "outgoing calls are not available yet")
	}
	return status.Error(codes.InvalidArgument, `direction must be "incoming" or "outgoing"`)
}

func callToProto(c *gooyai.IncomingCall) *pb.Call {
	out := &pb.Call{
		Direction:        directionIncoming,
		Id:               strconv.FormatInt(c.ID, 10),
		FromNumber:       c.FromNumber,
		ToNumber:         c.ToNumber,
		StartAt:          c.StartAt,
		EndAt:            c.EndAt,
		DurationSeconds:  int32(c.DurationSeconds), //nolint:gosec // a call's length in seconds
		RecordingUrl:     c.RecordingURL,
		CallForward:      c.CallForward,
		ShouldRecallUser: c.ShouldRecallUser,
	}
	if msgs, ok := c.ChatMessages(); ok {
		for _, m := range msgs {
			out.Transcript = append(out.Transcript, &pb.TranscriptMessage{Role: m.Role, Content: m.Content})
		}
	}
	// Analytics was decoded from JSON, so NewStruct can't fail on value types;
	// on any error the field is simply left empty.
	if s, err := structpb.NewStruct(c.Analytics); err == nil && len(c.Analytics) > 0 {
		out.Analytics = s
	}
	if c.Model != nil {
		out.AssistantName = c.Model.Name
	}
	return out
}

// connectionToProto never carries the key: only whether one is stored, its
// environment and last 4 characters — nor the proxy's password.
func connectionToProto(c *Connection) *pb.Connection {
	if c == nil {
		return &pb.Connection{}
	}
	return &pb.Connection{
		Connected:   true,
		Environment: c.Environment,
		KeyLast4:    c.KeyLast4,
		UpdatedAt:   c.UpdatedAt.UTC().Format(time.RFC3339),
		ProxyUrl:    c.ProxyRedacted,
	}
}

func providerToProto(p *gooyai.ServiceProvider) *pb.Provider {
	out := &pb.Provider{
		Id:        strconv.FormatInt(p.ID, 10),
		FirstName: p.FirstName,
		LastName:  p.LastName,
		Specialty: p.Specialty,
		Phone:     p.Phone,
		IsActive:  p.IsActive,
	}
	if p.ProfileImageURL != nil {
		out.ProfileImageUrl = *p.ProfileImageURL
	}
	return out
}
