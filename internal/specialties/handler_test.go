package specialties

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/salahfarzin/meet/internal/meets"
	pb "github.com/salahfarzin/meet/proto/specialties"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockService struct {
	createFn func(ctx context.Context, s *Specialty) (*Specialty, error)
	updateFn func(ctx context.Context, s *Specialty) (*Specialty, error)
	deleteFn func(ctx context.Context, uuid string) error
	getAllFn func(ctx context.Context, kind Kind, includeInactive bool) ([]*Specialty, error)
}

func (m *mockService) Create(ctx context.Context, s *Specialty) (*Specialty, error) {
	return m.createFn(ctx, s)
}
func (m *mockService) Update(ctx context.Context, s *Specialty) (*Specialty, error) {
	return m.updateFn(ctx, s)
}
func (m *mockService) Delete(ctx context.Context, uuid string) error {
	return m.deleteFn(ctx, uuid)
}
func (m *mockService) GetAll(ctx context.Context, kind Kind, includeInactive bool) ([]*Specialty, error) {
	return m.getAllFn(ctx, kind, includeInactive)
}

func ctxWithRoles(roles string) context.Context {
	md := metadata.New(map[string]string{"x-user-uuid": "u1", "x-user-roles": roles})
	return metadata.NewIncomingContext(context.Background(), md)
}

var (
	adminCtx      = ctxWithRoles(meets.RoleAdmin)
	superAdminCtx = ctxWithRoles(meets.RoleSuperAdmin)
	trappistCtx   = ctxWithRoles("Trappist")
)

func sample() *Specialty {
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	return &Specialty{UUID: "s1", Kind: KindSpecialty, Slug: "anxiety", Names: map[string]string{"en": "Anxiety"}, SortOrder: 1, Active: true, CreatedAt: now, UpdatedAt: now}
}

func assertCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, code, st.Code())
}

func TestHandlerGetAll(t *testing.T) {
	var gotInactive bool
	svc := &mockService{getAllFn: func(_ context.Context, kind Kind, includeInactive bool) ([]*Specialty, error) {
		gotInactive = includeInactive
		return []*Specialty{sample()}, nil
	}}

	t.Run("non-admins never see inactive entries", func(t *testing.T) {
		resp, err := NewHandler(svc, false).GetAll(trappistCtx, &pb.GetAllRequest{IncludeInactive: true})
		require.NoError(t, err)
		assert.False(t, gotInactive)
		require.Len(t, resp.Specialties, 1)
		assert.Equal(t, "anxiety", resp.Specialties[0].Slug)
		assert.Equal(t, "2026-09-27T10:00:00Z", resp.Specialties[0].CreatedAt)
	})

	t.Run("admins may include inactive", func(t *testing.T) {
		_, err := NewHandler(svc, false).GetAll(adminCtx, &pb.GetAllRequest{IncludeInactive: true})
		require.NoError(t, err)
		assert.True(t, gotInactive)
	})

	t.Run("invalid kind", func(t *testing.T) {
		bad := &mockService{getAllFn: func(context.Context, Kind, bool) ([]*Specialty, error) { return nil, ErrInvalidKind }}
		_, err := NewHandler(bad, false).GetAll(trappistCtx, &pb.GetAllRequest{Kind: "x"})
		assertCode(t, err, codes.InvalidArgument)
	})

	t.Run("internal error", func(t *testing.T) {
		bad := &mockService{getAllFn: func(context.Context, Kind, bool) ([]*Specialty, error) { return nil, errors.New("db") }}
		_, err := NewHandler(bad, false).GetAll(trappistCtx, &pb.GetAllRequest{})
		assertCode(t, err, codes.Internal)
	})
}

func TestHandlerCreate(t *testing.T) {
	svc := &mockService{createFn: func(_ context.Context, s *Specialty) (*Specialty, error) {
		out := sample()
		out.Slug = s.Slug
		return out, nil
	}}
	req := &pb.CreateRequest{Specialty: &pb.Specialty{Kind: "specialty", Slug: "depression", Names: map[string]string{"en": "Depression"}}}

	t.Run("admin", func(t *testing.T) {
		resp, err := NewHandler(svc, false).Create(adminCtx, req)
		require.NoError(t, err)
		assert.Equal(t, "depression", resp.Specialty.Slug)
		assert.Equal(t, "success", resp.Status.Message)
	})

	t.Run("superadmin", func(t *testing.T) {
		_, err := NewHandler(svc, false).Create(superAdminCtx, req)
		assert.NoError(t, err)
	})

	t.Run("auth disabled", func(t *testing.T) {
		_, err := NewHandler(svc, true).Create(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("trappist is denied", func(t *testing.T) {
		_, err := NewHandler(svc, false).Create(trappistCtx, req)
		assertCode(t, err, codes.PermissionDenied)
	})

	t.Run("missing body", func(t *testing.T) {
		_, err := NewHandler(svc, false).Create(adminCtx, &pb.CreateRequest{})
		assertCode(t, err, codes.InvalidArgument)
	})

	t.Run("service error", func(t *testing.T) {
		bad := &mockService{createFn: func(context.Context, *Specialty) (*Specialty, error) { return nil, errors.New("slug taken") }}
		_, err := NewHandler(bad, false).Create(adminCtx, req)
		assertCode(t, err, codes.InvalidArgument)
	})
}

func TestHandlerUpdate(t *testing.T) {
	var got *Specialty
	svc := &mockService{updateFn: func(_ context.Context, s *Specialty) (*Specialty, error) {
		got = s
		return sample(), nil
	}}
	req := &pb.UpdateRequest{Uuid: "s1", Specialty: &pb.Specialty{Kind: "specialty", Slug: "anxiety", Names: map[string]string{"en": "Anxiety"}, Active: false}}

	t.Run("admin", func(t *testing.T) {
		_, err := NewHandler(svc, false).Update(adminCtx, req)
		require.NoError(t, err)
		assert.Equal(t, "s1", got.UUID)
		assert.False(t, got.Active)
	})

	t.Run("trappist is denied", func(t *testing.T) {
		_, err := NewHandler(svc, false).Update(trappistCtx, req)
		assertCode(t, err, codes.PermissionDenied)
	})

	t.Run("missing uuid", func(t *testing.T) {
		_, err := NewHandler(svc, false).Update(adminCtx, &pb.UpdateRequest{Specialty: req.Specialty})
		assertCode(t, err, codes.InvalidArgument)
	})

	t.Run("not found", func(t *testing.T) {
		bad := &mockService{updateFn: func(context.Context, *Specialty) (*Specialty, error) { return nil, ErrNotFound }}
		_, err := NewHandler(bad, false).Update(adminCtx, req)
		assertCode(t, err, codes.NotFound)
	})

	t.Run("validation error", func(t *testing.T) {
		bad := &mockService{updateFn: func(context.Context, *Specialty) (*Specialty, error) { return nil, ErrInvalidKind }}
		_, err := NewHandler(bad, false).Update(adminCtx, req)
		assertCode(t, err, codes.InvalidArgument)
	})
}

func TestHandlerDelete(t *testing.T) {
	svc := &mockService{deleteFn: func(context.Context, string) error { return nil }}

	t.Run("admin", func(t *testing.T) {
		_, err := NewHandler(svc, false).Delete(adminCtx, &pb.DeleteRequest{Uuid: "s1"})
		assert.NoError(t, err)
	})

	t.Run("trappist is denied", func(t *testing.T) {
		_, err := NewHandler(svc, false).Delete(trappistCtx, &pb.DeleteRequest{Uuid: "s1"})
		assertCode(t, err, codes.PermissionDenied)
	})

	t.Run("missing uuid", func(t *testing.T) {
		_, err := NewHandler(svc, false).Delete(adminCtx, &pb.DeleteRequest{})
		assertCode(t, err, codes.InvalidArgument)
	})

	t.Run("service error", func(t *testing.T) {
		bad := &mockService{deleteFn: func(context.Context, string) error { return errors.New("db") }}
		_, err := NewHandler(bad, false).Delete(adminCtx, &pb.DeleteRequest{Uuid: "s1"})
		assertCode(t, err, codes.Internal)
	})
}
