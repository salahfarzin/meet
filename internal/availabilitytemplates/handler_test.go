package availabilitytemplates

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/salahfarzin/meet/proto/availability_templates"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockService struct {
	createFn func(ctx context.Context, t *Template) (*Template, error)
	updateFn func(ctx context.Context, t *Template) (*Template, error)
	deleteFn func(ctx context.Context, uuid string) error
	getAllFn func(ctx context.Context, organizerUuid string) ([]*Template, error)
}

func (m *mockService) Create(ctx context.Context, t *Template) (*Template, error) {
	return m.createFn(ctx, t)
}
func (m *mockService) Update(ctx context.Context, t *Template) (*Template, error) {
	return m.updateFn(ctx, t)
}
func (m *mockService) GetByUUID(ctx context.Context, uuid string) (*Template, error) {
	return nil, nil
}
func (m *mockService) Delete(ctx context.Context, uuid string) error {
	return m.deleteFn(ctx, uuid)
}
func (m *mockService) GetAll(ctx context.Context, organizerUuid string) ([]*Template, error) {
	return m.getAllFn(ctx, organizerUuid)
}
func (m *mockService) Materialize(ctx context.Context, organizerUuid string, from, to time.Time) error {
	return nil
}
func (m *mockService) RecordSkip(ctx context.Context, templateUuid string, occurrenceDate time.Time) error {
	return nil
}

func ctxWithUser(uuid string, roles ...string) context.Context {
	md := metadata.New(map[string]string{
		"x-user-uuid":  uuid,
		"x-user-roles": joinRoles(roles),
	})
	return metadata.NewIncomingContext(context.Background(), md)
}

func joinRoles(roles []string) string {
	out := ""
	for i, r := range roles {
		if i > 0 {
			out += ","
		}
		out += r
	}
	return out
}

func validTemplateProto() *pb.AvailabilityTemplate {
	return &pb.AvailabilityTemplate{
		OrganizerUuid: "org-1",
		Weekday:       1,
		StartTime:     "09:00",
		EndTime:       "10:00",
		EffectiveFrom: "2026-01-01",
	}
}

func TestHandlerCreateNilRequest(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	_, err := h.Create(context.Background(), nil)
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlerCreateNilTemplate(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	_, err := h.Create(context.Background(), &pb.CreateRequest{})
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlerCreateInvalidTime(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	req := &pb.CreateRequest{Template: &pb.AvailabilityTemplate{
		OrganizerUuid: "org-1",
		EffectiveFrom: "not-a-date",
	}}
	_, err := h.Create(context.Background(), req)
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlerCreateInvalidEffectiveUntil(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	bad := "not-a-date"
	req := &pb.CreateRequest{Template: validTemplateProto()}
	req.Template.EffectiveUntil = &bad
	_, err := h.Create(context.Background(), req)
	assert.Error(t, err)
}

func TestHandlerCreateServiceError(t *testing.T) {
	svc := &mockService{createFn: func(ctx context.Context, t *Template) (*Template, error) {
		return nil, errors.New("boom")
	}}
	h := NewHandler(svc, false)
	_, err := h.Create(ctxWithUser("org-1"), &pb.CreateRequest{Template: validTemplateProto()})
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlerCreateSuccess(t *testing.T) {
	until := "2026-02-01"
	priceUuid := "price-1"
	svc := &mockService{createFn: func(ctx context.Context, t *Template) (*Template, error) {
		t.UUID = "created-uuid"
		t.Active = true
		t.CreatedAt = time.Now()
		t.UpdatedAt = time.Now()
		return t, nil
	}}
	h := NewHandler(svc, false)
	req := &pb.CreateRequest{Template: validTemplateProto()}
	req.Template.EffectiveUntil = &until
	req.Template.PriceUuid = &priceUuid
	req.Template.StartTime = "09:00:00"
	resp, err := h.Create(ctxWithUser("org-1"), req)
	assert.NoError(t, err)
	assert.Equal(t, "created-uuid", resp.Template.Uuid)
	assert.NotNil(t, resp.Template.EffectiveUntil)
}

func TestHandlerUpdateNilRequest(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	_, err := h.Update(context.Background(), nil)
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlerUpdateMissingUuid(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	_, err := h.Update(context.Background(), &pb.UpdateRequest{Template: validTemplateProto()})
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlerUpdateInvalidTime(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	req := &pb.UpdateRequest{Uuid: "u1", Template: &pb.AvailabilityTemplate{EffectiveFrom: "bad"}}
	_, err := h.Update(context.Background(), req)
	assert.Error(t, err)
}

func TestHandlerUpdateServiceError(t *testing.T) {
	svc := &mockService{updateFn: func(ctx context.Context, t *Template) (*Template, error) {
		return nil, errors.New("boom")
	}}
	h := NewHandler(svc, false)
	_, err := h.Update(ctxWithUser("org-1"), &pb.UpdateRequest{Uuid: "u1", Template: validTemplateProto()})
	assert.Error(t, err)
}

func TestHandlerUpdateSuccess(t *testing.T) {
	svc := &mockService{updateFn: func(ctx context.Context, t *Template) (*Template, error) {
		return t, nil
	}}
	h := NewHandler(svc, false)
	resp, err := h.Update(ctxWithUser("org-1"), &pb.UpdateRequest{Uuid: "u1", Template: validTemplateProto()})
	assert.NoError(t, err)
	assert.Equal(t, "u1", resp.Template.Uuid)
}

func TestHandlerDeleteMissingUuid(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	_, err := h.Delete(context.Background(), &pb.DeleteRequest{})
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlerDeleteNilRequest(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	_, err := h.Delete(context.Background(), nil)
	assert.Error(t, err)
}

func TestHandlerDeleteServiceError(t *testing.T) {
	svc := &mockService{deleteFn: func(ctx context.Context, uuid string) error {
		return errors.New("boom")
	}}
	h := NewHandler(svc, false)
	_, err := h.Delete(context.Background(), &pb.DeleteRequest{Uuid: "u1"})
	assert.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestHandlerDeleteSuccess(t *testing.T) {
	svc := &mockService{deleteFn: func(ctx context.Context, uuid string) error {
		return nil
	}}
	h := NewHandler(svc, false)
	_, err := h.Delete(context.Background(), &pb.DeleteRequest{Uuid: "u1"})
	assert.NoError(t, err)
}

func TestHandlerGetAllWithOrganizerId(t *testing.T) {
	svc := &mockService{getAllFn: func(ctx context.Context, organizerUuid string) ([]*Template, error) {
		assert.Equal(t, "org-explicit", organizerUuid)
		return []*Template{{UUID: "t1", CreatedAt: time.Now(), UpdatedAt: time.Now()}}, nil
	}}
	h := NewHandler(svc, false)
	resp, err := h.GetAll(context.Background(), &pb.GetAllRequest{OrganizerId: "org-explicit"})
	assert.NoError(t, err)
	assert.Len(t, resp.Templates, 1)
}

func TestHandlerGetAllFallsBackToContextUser(t *testing.T) {
	svc := &mockService{getAllFn: func(ctx context.Context, organizerUuid string) ([]*Template, error) {
		assert.Equal(t, "ctx-user", organizerUuid)
		return nil, nil
	}}
	h := NewHandler(svc, false)
	_, err := h.GetAll(ctxWithUser("ctx-user"), &pb.GetAllRequest{})
	assert.NoError(t, err)
}

func TestHandlerGetAllServiceError(t *testing.T) {
	svc := &mockService{getAllFn: func(ctx context.Context, organizerUuid string) ([]*Template, error) {
		return nil, errors.New("boom")
	}}
	h := NewHandler(svc, false)
	_, err := h.GetAll(context.Background(), &pb.GetAllRequest{})
	assert.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestRetrieveOrganizerUuidAuthDisabledHonorsRequested(t *testing.T) {
	h := NewHandler(&mockService{}, true)
	got := h.retrieveOrganizerUuid(context.Background(), "requested-uuid")
	assert.Equal(t, "requested-uuid", got)
}

func TestRetrieveOrganizerUuidSuperAdminOverride(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	ctx := ctxWithUser("admin-uuid", "Programmer")
	got := h.retrieveOrganizerUuid(ctx, "requested-uuid")
	assert.Equal(t, "requested-uuid", got)
}

func TestRetrieveOrganizerUuidDefaultsToCallerUuid(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	ctx := ctxWithUser("caller-uuid")
	got := h.retrieveOrganizerUuid(ctx, "requested-uuid")
	assert.Equal(t, "caller-uuid", got)
}

func TestNormalizeTime(t *testing.T) {
	assert.Equal(t, "09:00:00", normalizeTime("09:00"))
	assert.Equal(t, "09:00:00", normalizeTime("09:00:00"))
}
