package gooyai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/salahfarzin/meet/internal/meets"
	"github.com/salahfarzin/meet/pkg/gooyai"
	pb "github.com/salahfarzin/meet/proto/gooyai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockService struct {
	tenant        string
	filter        *gooyai.ListCallsFilter
	listCalls     func() (*gooyai.CallPage, error)
	getCall       func(id int64) (*gooyai.IncomingCall, error)
	listProviders func() ([]gooyai.ServiceProvider, error)
	getConn       func() (*Connection, error)
	connect       func(apiKey, env string) (*Connection, error)
	disconnect    func() error
}

func (m *mockService) ListIncomingCalls(_ context.Context, tenant string, f *gooyai.ListCallsFilter) (*gooyai.CallPage, error) {
	m.tenant, m.filter = tenant, f
	return m.listCalls()
}
func (m *mockService) GetIncomingCall(_ context.Context, tenant string, id int64) (*gooyai.IncomingCall, error) {
	m.tenant = tenant
	return m.getCall(id)
}
func (m *mockService) ListProviders(_ context.Context, tenant string) ([]gooyai.ServiceProvider, error) {
	m.tenant = tenant
	return m.listProviders()
}
func (m *mockService) GetConnection(_ context.Context, tenant string) (*Connection, error) {
	m.tenant = tenant
	return m.getConn()
}
func (m *mockService) Connect(_ context.Context, tenant, apiKey, env string) (*Connection, error) {
	m.tenant = tenant
	return m.connect(apiKey, env)
}
func (m *mockService) Disconnect(_ context.Context, tenant string) error {
	m.tenant = tenant
	return m.disconnect()
}

func ctxAs(uuid, roles string) context.Context {
	md := metadata.New(map[string]string{"x-user-uuid": uuid, "x-user-roles": roles})
	return metadata.NewIncomingContext(context.Background(), md)
}

var (
	clinicCtx     = ctxAs("clinic-a", meets.RoleAdmin)
	superAdminCtx = ctxAs("root", meets.RoleSuperAdmin)
)

func assertCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	st, ok := status.FromError(err)
	require.True(t, ok, "expected a gRPC status, got %v", err)
	assert.Equal(t, code, st.Code())
}

func sampleCall() gooyai.IncomingCall {
	return gooyai.IncomingCall{
		ID:               101,
		FromNumber:       "+989121111111",
		ToNumber:         "+989121000001",
		StartAt:          "2026-06-23T08:15:00.000Z",
		EndAt:            "2026-06-23T08:17:00.000Z",
		DurationSeconds:  120,
		RecordingURL:     "https://example.com/call-101.mp3",
		ShouldRecallUser: true,
		Analytics:        map[string]any{"sentiment": "neutral", "resolved": true},
		Chats:            json.RawMessage(`[{"role":"assistant","content":"hello"}]`),
		Model:            &gooyai.AIModel{ID: 1, Name: "Support Bot"},
	}
}

func okCalls() (*gooyai.CallPage, error) {
	return &gooyai.CallPage{
		Calls:      []gooyai.IncomingCall{sampleCall()},
		Pagination: gooyai.Pagination{Total: 42, Page: 2, Limit: 20, TotalPages: 3},
	}, nil
}

func TestListCalls_MapsResponse(t *testing.T) {
	svc := &mockService{listCalls: okCalls}

	resp, err := NewHandler(svc, false).ListCalls(clinicCtx, &pb.ListCallsRequest{Page: 2})
	require.NoError(t, err)

	assert.Equal(t, "clinic-a", svc.tenant)
	assert.Equal(t, int32(42), resp.Total)
	assert.Equal(t, int32(2), resp.Page)
	assert.Equal(t, int32(20), resp.PageSize)
	assert.Equal(t, int32(3), resp.TotalPages)
	require.Len(t, resp.Calls, 1)

	c := resp.Calls[0]
	assert.Equal(t, "101", c.Id)
	assert.Equal(t, "incoming", c.Direction)
	assert.Equal(t, "+989121111111", c.FromNumber)
	assert.Equal(t, int32(120), c.DurationSeconds)
	assert.True(t, c.ShouldRecallUser)
	assert.Equal(t, "Support Bot", c.AssistantName)
	require.Len(t, c.Transcript, 1)
	assert.Equal(t, "hello", c.Transcript[0].Content)
	assert.Equal(t, "neutral", c.Analytics.Fields["sentiment"].GetStringValue())
}

func TestListCalls_BuildsFilter(t *testing.T) {
	recall := false
	svc := &mockService{listCalls: okCalls}

	_, err := NewHandler(svc, false).ListCalls(clinicCtx, &pb.ListCallsRequest{
		PageSize: 50, From: "2026-06-01", To: "2026-06-30", Phone: "0912", ShouldRecall: &recall,
	})
	require.NoError(t, err)

	f := svc.filter
	assert.Equal(t, 1, f.Page, "page defaults to 1")
	assert.Equal(t, 50, f.Limit)
	assert.Equal(t, "0912", f.FromNumber)
	assert.Equal(t, "2026-06-01", f.StartDate)
	assert.Equal(t, "2026-06-30", f.EndDate)
	require.NotNil(t, f.ShouldRecallUser)
	assert.False(t, *f.ShouldRecallUser)
}

func TestListCalls_DefaultsAndUnsetRecall(t *testing.T) {
	svc := &mockService{listCalls: okCalls}

	_, err := NewHandler(svc, false).ListCalls(clinicCtx, &pb.ListCallsRequest{})
	require.NoError(t, err)
	assert.Equal(t, defaultPageSize, svc.filter.Limit)
	assert.Nil(t, svc.filter.ShouldRecallUser, "unset means don't filter")
}

func TestListCalls_InvalidFilters(t *testing.T) {
	tests := map[string]*pb.ListCallsRequest{
		"page size too large": {PageSize: 101},
		"bad from":            {From: "1405/04/01"},
		"bad to":              {To: "tomorrow"},
		"reversed range":      {From: "2026-06-30", To: "2026-06-01"},
	}
	for name, req := range tests {
		t.Run(name, func(t *testing.T) {
			svc := &mockService{listCalls: okCalls}
			_, err := NewHandler(svc, false).ListCalls(clinicCtx, req)
			assertCode(t, err, codes.InvalidArgument)
		})
	}
}

func TestListCalls_ServiceError(t *testing.T) {
	svc := &mockService{listCalls: func() (*gooyai.CallPage, error) { return nil, ErrNotConnected }}

	_, err := NewHandler(svc, false).ListCalls(clinicCtx, &pb.ListCallsRequest{})
	assertCode(t, err, codes.FailedPrecondition)
}

func TestDirection(t *testing.T) {
	tests := []struct {
		direction string
		want      codes.Code
	}{
		{"", codes.OK},
		{"incoming", codes.OK},
		{"outgoing", codes.Unimplemented},
		{"sideways", codes.InvalidArgument},
	}
	for _, tt := range tests {
		t.Run("direction="+tt.direction, func(t *testing.T) {
			svc := &mockService{
				listCalls: okCalls,
				getCall: func(int64) (*gooyai.IncomingCall, error) {
					c := sampleCall()
					return &c, nil
				},
			}
			h := NewHandler(svc, false)

			_, listErr := h.ListCalls(clinicCtx, &pb.ListCallsRequest{Direction: tt.direction})
			_, getErr := h.GetCall(clinicCtx, &pb.GetCallRequest{Id: "101", Direction: tt.direction})

			assert.Equal(t, tt.want, status.Code(listErr))
			assert.Equal(t, tt.want, status.Code(getErr))
			if tt.want != codes.OK {
				assert.Empty(t, svc.tenant, "a rejected direction never reaches Gooyai")
			}
		})
	}
}

func TestTenantResolution(t *testing.T) {
	tests := []struct {
		name         string
		ctx          context.Context
		authDisabled bool
		requested    string
		want         string
	}{
		{"caller's own clinic", clinicCtx, false, "", "clinic-a"},
		{"non-superadmin can't pick another clinic", clinicCtx, false, "clinic-b", "clinic-a"},
		{"superadmin may pick a clinic", superAdminCtx, false, "clinic-b", "clinic-b"},
		{"superadmin without a pick uses own uuid", superAdminCtx, false, "", "root"},
		{"auth disabled uses the local tenant", context.Background(), true, "", localTenant},
		{"auth disabled may pick a clinic", context.Background(), true, "clinic-b", "clinic-b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockService{listProviders: func() ([]gooyai.ServiceProvider, error) { return nil, nil }}
			_, err := NewHandler(svc, tt.authDisabled).ListProviders(tt.ctx, &pb.ListProvidersRequest{OrganizerId: tt.requested})
			require.NoError(t, err)
			assert.Equal(t, tt.want, svc.tenant)
		})
	}
}

func TestMissingIdentityIsDenied(t *testing.T) {
	h := NewHandler(&mockService{}, false)
	ctx := context.Background()

	_, err := h.ListCalls(ctx, &pb.ListCallsRequest{})
	assertCode(t, err, codes.PermissionDenied)
	_, err = h.GetCall(ctx, &pb.GetCallRequest{Id: "1"})
	assertCode(t, err, codes.PermissionDenied)
	_, err = h.ListProviders(ctx, &pb.ListProvidersRequest{})
	assertCode(t, err, codes.PermissionDenied)
	_, err = h.GetConnection(ctx, &pb.GetConnectionRequest{})
	assertCode(t, err, codes.PermissionDenied)
	_, err = h.SaveConnection(ctx, &pb.SaveConnectionRequest{ApiKey: "k"})
	assertCode(t, err, codes.PermissionDenied)
	_, err = h.DeleteConnection(ctx, &pb.DeleteConnectionRequest{})
	assertCode(t, err, codes.PermissionDenied)
}

func TestGetConnection(t *testing.T) {
	updated := time.Date(2026, 10, 10, 12, 0, 0, 0, time.FixedZone("Tehran", 3*3600+1800))

	t.Run("connected: status and hint, never the key", func(t *testing.T) {
		svc := &mockService{getConn: func() (*Connection, error) {
			return &Connection{OrganizerUUID: "clinic-a", APIKeySealed: []byte("sealed"), KeyLast4: "ab12", Environment: "prod", UpdatedAt: updated}, nil
		}}

		conn, err := NewHandler(svc, false).GetConnection(clinicCtx, &pb.GetConnectionRequest{})
		require.NoError(t, err)
		assert.Equal(t, "clinic-a", svc.tenant)
		assert.Equal(t, &pb.Connection{Connected: true, Environment: "prod", KeyLast4: "ab12", UpdatedAt: "2026-10-10T08:30:00Z"}, conn)
	})

	t.Run("not connected", func(t *testing.T) {
		svc := &mockService{getConn: func() (*Connection, error) { return nil, nil }}

		conn, err := NewHandler(svc, false).GetConnection(clinicCtx, &pb.GetConnectionRequest{})
		require.NoError(t, err)
		assert.False(t, conn.Connected)
		assert.Empty(t, conn.KeyLast4)
	})

	t.Run("storage error", func(t *testing.T) {
		svc := &mockService{getConn: func() (*Connection, error) { return nil, errors.New("db down") }}

		_, err := NewHandler(svc, false).GetConnection(clinicCtx, &pb.GetConnectionRequest{})
		assertCode(t, err, codes.Internal)
	})
}

func TestSaveConnection(t *testing.T) {
	t.Run("passes key and environment, returns the new status", func(t *testing.T) {
		var gotKey, gotEnv string
		svc := &mockService{connect: func(apiKey, env string) (*Connection, error) {
			gotKey, gotEnv = apiKey, env
			return &Connection{KeyLast4: "ab12", Environment: env}, nil
		}}

		conn, err := NewHandler(svc, false).SaveConnection(clinicCtx, &pb.SaveConnectionRequest{ApiKey: "secret-key-ab12", Environment: "sandbox"})
		require.NoError(t, err)
		assert.Equal(t, "secret-key-ab12", gotKey)
		assert.Equal(t, "sandbox", gotEnv)
		assert.True(t, conn.Connected)
		assert.Equal(t, "ab12", conn.KeyLast4)
	})

	t.Run("a rejected key is the caller's to fix", func(t *testing.T) {
		svc := &mockService{connect: func(string, string) (*Connection, error) { return nil, ErrKeyRejected }}

		_, err := NewHandler(svc, false).SaveConnection(clinicCtx, &pb.SaveConnectionRequest{ApiKey: "bad"})
		assertCode(t, err, codes.InvalidArgument)
	})
}

func TestDeleteConnection(t *testing.T) {
	svc := &mockService{disconnect: func() error { return nil }}

	_, err := NewHandler(svc, false).DeleteConnection(clinicCtx, &pb.DeleteConnectionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "clinic-a", svc.tenant)

	svc.disconnect = func() error { return errors.New("db down") }
	_, err = NewHandler(svc, false).DeleteConnection(clinicCtx, &pb.DeleteConnectionRequest{})
	assertCode(t, err, codes.Internal)
}

func TestGetCall(t *testing.T) {
	var gotID int64
	svc := &mockService{getCall: func(id int64) (*gooyai.IncomingCall, error) {
		gotID = id
		c := sampleCall()
		return &c, nil
	}}

	call, err := NewHandler(svc, false).GetCall(clinicCtx, &pb.GetCallRequest{Id: "101"})
	require.NoError(t, err)
	assert.Equal(t, int64(101), gotID)
	assert.Equal(t, "101", call.Id, "returned bare, not wrapped")
}

func TestGetCall_InvalidID(t *testing.T) {
	for _, id := range []string{"", "abc", "0", "-5"} {
		_, err := NewHandler(&mockService{}, false).GetCall(clinicCtx, &pb.GetCallRequest{Id: id})
		assertCode(t, err, codes.InvalidArgument)
	}
}

func TestGetCall_NotFound(t *testing.T) {
	svc := &mockService{getCall: func(int64) (*gooyai.IncomingCall, error) {
		return nil, &gooyai.APIError{StatusCode: 404}
	}}

	_, err := NewHandler(svc, false).GetCall(clinicCtx, &pb.GetCallRequest{Id: "9"})
	assertCode(t, err, codes.NotFound)
}

func TestListProviders(t *testing.T) {
	img := "https://example.com/p.png"
	svc := &mockService{listProviders: func() ([]gooyai.ServiceProvider, error) {
		return []gooyai.ServiceProvider{
			{ID: 1, FirstName: "Aaran", LastName: "Ahmadi", Specialty: "GP", Phone: "+989121000001", IsActive: true, ProfileImageURL: &img},
			{ID: 2, FirstName: "Sara"},
		}, nil
	}}

	resp, err := NewHandler(svc, false).ListProviders(clinicCtx, &pb.ListProvidersRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Providers, 2)
	assert.Equal(t, &pb.Provider{Id: "1", FirstName: "Aaran", LastName: "Ahmadi", Specialty: "GP", Phone: "+989121000001", IsActive: true, ProfileImageUrl: img}, resp.Providers[0])
	assert.Empty(t, resp.Providers[1].ProfileImageUrl)
}

func TestListProviders_ServiceError(t *testing.T) {
	svc := &mockService{listProviders: func() ([]gooyai.ServiceProvider, error) {
		return nil, &gooyai.APIError{StatusCode: 429}
	}}

	_, err := NewHandler(svc, false).ListProviders(clinicCtx, &pb.ListProvidersRequest{})
	assertCode(t, err, codes.ResourceExhausted)
}

func TestToStatus(t *testing.T) {
	tests := []struct {
		err  error
		want codes.Code
	}{
		{ErrNotConnected, codes.FailedPrecondition},
		{fmt.Errorf("wrapped: %w", ErrNotConnected), codes.FailedPrecondition},
		// Never Unauthenticated: the dashboard would log the user out.
		{&gooyai.APIError{StatusCode: 401}, codes.FailedPrecondition},
		{&gooyai.APIError{StatusCode: 404}, codes.NotFound},
		{&gooyai.APIError{StatusCode: 400, Message: "اعتبارسنجی"}, codes.InvalidArgument},
		{&gooyai.APIError{StatusCode: 429}, codes.ResourceExhausted},
		{&gooyai.APIError{StatusCode: 503}, codes.Unavailable},
		{ErrEncryptionNotConfigured, codes.FailedPrecondition},
		{ErrKeyRejected, codes.InvalidArgument},
		{ErrInvalidConnection, codes.InvalidArgument},
		// Not Gooyai's fault (e.g. the database): never blamed on Gooyai.
		{errors.New("db: connection refused"), codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			err := toStatus(context.Background(), tt.err, "op")
			assertCode(t, err, tt.want)
			assert.NotContains(t, status.Convert(err).Message(), "اعتبارسنجی", "Gooyai's Persian text is never surfaced")
		})
	}
}

func TestCallToProto_SparseCall(t *testing.T) {
	c := callToProto(&gooyai.IncomingCall{ID: 5, Chats: json.RawMessage(`{"unexpected":true}`)})

	assert.Equal(t, "5", c.Id)
	assert.Empty(t, c.Transcript, "an unknown transcript shape is dropped, not an error")
	assert.Nil(t, c.Analytics)
	assert.Empty(t, c.AssistantName)
}
