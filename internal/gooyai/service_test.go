package gooyai

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/salahfarzin/meet/pkg/gooyai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	listCalls     func(f *gooyai.ListCallsFilter) (*gooyai.CallPage, error)
	getCall       func(id int64) (*gooyai.IncomingCall, error)
	listProviders func(f *gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error)
}

func (f *fakeClient) ListIncomingCalls(_ context.Context, filter *gooyai.ListCallsFilter) (*gooyai.CallPage, error) {
	return f.listCalls(filter)
}
func (f *fakeClient) GetIncomingCall(_ context.Context, id int64) (*gooyai.IncomingCall, error) {
	return f.getCall(id)
}
func (f *fakeClient) ListProviders(_ context.Context, filter *gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
	return f.listProviders(filter)
}

// fakeRepo is an in-memory Repository; err, when set, fails every call.
type fakeRepo struct {
	rows map[string]*Connection
	err  error
}

func (r *fakeRepo) Get(_ context.Context, organizerUUID string) (*Connection, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.rows[organizerUUID], nil
}
func (r *fakeRepo) Upsert(_ context.Context, c *Connection) error {
	if r.err != nil {
		return r.err
	}
	r.rows[c.OrganizerUUID] = c
	return nil
}
func (r *fakeRepo) Delete(_ context.Context, organizerUUID string) error {
	if r.err != nil {
		return r.err
	}
	delete(r.rows, organizerUUID)
	return nil
}

var testEncryptionKey = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

func testSealer(t *testing.T) *Sealer {
	t.Helper()
	s, err := NewSealer(testEncryptionKey)
	require.NoError(t, err)
	return s
}

// used records which key and environment the last client was built for.
type used struct{ key, env string }

// newTestService connects clinic-a (key "key-a", prod) and wires fc in.
func newTestService(t *testing.T, fc *fakeClient) (Service, *fakeRepo, *used) {
	t.Helper()
	sealer := testSealer(t)
	sealed, err := sealer.Seal("key-a")
	require.NoError(t, err)
	repo := &fakeRepo{rows: map[string]*Connection{
		"clinic-a": {OrganizerUUID: "clinic-a", APIKeySealed: sealed, KeyLast4: "", Environment: "prod"},
	}}
	u := &used{}
	svc := NewService(repo, sealer, func(key, env string) Client {
		u.key, u.env = key, env
		return fc
	})
	return svc, repo, u
}

func TestService_NotConnected(t *testing.T) {
	svc, _, _ := newTestService(t, &fakeClient{})
	ctx := context.Background()

	_, err := svc.ListIncomingCalls(ctx, "unknown", nil)
	assert.ErrorIs(t, err, ErrNotConnected)
	_, err = svc.GetIncomingCall(ctx, "unknown", 1)
	assert.ErrorIs(t, err, ErrNotConnected)
	_, err = svc.ListProviders(ctx, "unknown")
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestService_ClientErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("storage failure", func(t *testing.T) {
		svc, repo, _ := newTestService(t, &fakeClient{})
		repo.err = errors.New("db down")
		_, err := svc.ListIncomingCalls(ctx, "clinic-a", nil)
		assert.EqualError(t, err, "db down")
	})

	t.Run("key that won't decrypt (e.g. encryption key rotated)", func(t *testing.T) {
		svc, repo, _ := newTestService(t, &fakeClient{})
		repo.rows["clinic-a"].APIKeySealed = []byte("garbage-that-is-long-enough-to-have-a-nonce")
		_, err := svc.ListIncomingCalls(ctx, "clinic-a", nil)
		assert.ErrorContains(t, err, "open sealed gooyai key")
	})

	t.Run("encryption not configured", func(t *testing.T) {
		repo := &fakeRepo{rows: map[string]*Connection{"clinic-a": {OrganizerUUID: "clinic-a", APIKeySealed: []byte("x")}}}
		svc := NewService(repo, nil, nil)
		_, err := svc.ListIncomingCalls(ctx, "clinic-a", nil)
		assert.ErrorIs(t, err, ErrEncryptionNotConfigured)
	})
}

func TestService_ListIncomingCalls_UsesDecryptedKeyAndEnvironment(t *testing.T) {
	want := &gooyai.CallPage{Calls: []gooyai.IncomingCall{{ID: 1}}}
	var got *gooyai.ListCallsFilter
	svc, _, u := newTestService(t, &fakeClient{listCalls: func(f *gooyai.ListCallsFilter) (*gooyai.CallPage, error) {
		got = f
		return want, nil
	}})

	page, err := svc.ListIncomingCalls(context.Background(), "clinic-a", &gooyai.ListCallsFilter{FromNumber: "0912"})
	require.NoError(t, err)
	assert.Same(t, want, page)
	assert.Equal(t, used{key: "key-a", env: "prod"}, *u)
	assert.Equal(t, "0912", got.FromNumber)
}

func TestService_GetIncomingCall(t *testing.T) {
	svc, _, _ := newTestService(t, &fakeClient{getCall: func(id int64) (*gooyai.IncomingCall, error) {
		return &gooyai.IncomingCall{ID: id}, nil
	}})

	call, err := svc.GetIncomingCall(context.Background(), "clinic-a", 101)
	require.NoError(t, err)
	assert.Equal(t, int64(101), call.ID)
}

func TestService_ListProviders_WalksAllPages(t *testing.T) {
	var pages []int
	svc, _, _ := newTestService(t, &fakeClient{listProviders: func(f *gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
		pages = append(pages, f.Page)
		assert.Equal(t, gooyai.MaxLimit, f.Limit)
		return &gooyai.ProviderPage{
			Providers:  []gooyai.ServiceProvider{{ID: int64(f.Page)}},
			Pagination: gooyai.Pagination{TotalPages: 3},
		}, nil
	}})

	providers, err := svc.ListProviders(context.Background(), "clinic-a")
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, pages)
	assert.Len(t, providers, 3)
}

func TestService_ListProviders_StopsAtPageCap(t *testing.T) {
	calls := 0
	svc, _, _ := newTestService(t, &fakeClient{listProviders: func(*gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
		calls++
		return &gooyai.ProviderPage{Pagination: gooyai.Pagination{TotalPages: 1_000_000}}, nil
	}})

	_, err := svc.ListProviders(context.Background(), "clinic-a")
	require.NoError(t, err)
	assert.Equal(t, maxProviderPages, calls)
}

func TestService_ListProviders_Error(t *testing.T) {
	boom := errors.New("boom")
	svc, _, _ := newTestService(t, &fakeClient{listProviders: func(*gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
		return nil, boom
	}})

	_, err := svc.ListProviders(context.Background(), "clinic-a")
	assert.ErrorIs(t, err, boom)
}

func okProbe(probed *gooyai.ListProvidersFilter) *fakeClient {
	return &fakeClient{listProviders: func(f *gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
		*probed = *f
		return &gooyai.ProviderPage{}, nil
	}}
}

func TestService_Connect_ChecksThenStoresEncrypted(t *testing.T) {
	var probed gooyai.ListProvidersFilter
	svc, repo, u := newTestService(t, okProbe(&probed))

	conn, err := svc.Connect(context.Background(), "clinic-b", "  sk-live-123456789abcd  ", "")
	require.NoError(t, err)

	// Probed Gooyai with the trimmed key, sandbox by default, one row only.
	assert.Equal(t, used{key: "sk-live-123456789abcd", env: "sandbox"}, *u)
	assert.Equal(t, gooyai.PageQuery{Page: 1, Limit: 1}, probed.PageQuery)

	assert.Equal(t, "abcd", conn.KeyLast4)
	assert.Equal(t, "sandbox", conn.Environment)
	stored := repo.rows["clinic-b"]
	assert.NotContains(t, string(stored.APIKeySealed), "sk-live", "the key is never stored in the clear")
	plain, err := testSealer(t).Open(stored.APIKeySealed)
	require.NoError(t, err)
	assert.Equal(t, "sk-live-123456789abcd", plain)
}

func TestService_Connect_ReplacesExistingConnection(t *testing.T) {
	var probed gooyai.ListProvidersFilter
	svc, repo, _ := newTestService(t, okProbe(&probed))

	_, err := svc.Connect(context.Background(), "clinic-a", "new-key-0000-wxyz", "prod")
	require.NoError(t, err)
	assert.Equal(t, "wxyz", repo.rows["clinic-a"].KeyLast4)
	assert.Equal(t, "prod", repo.rows["clinic-a"].Environment)
}

func TestService_Connect_Rejections(t *testing.T) {
	ctx := context.Background()
	var probed gooyai.ListProvidersFilter

	tests := []struct {
		name    string
		key     string
		env     string
		probe   error
		wantErr error
	}{
		{"blank key", "   ", "sandbox", nil, ErrInvalidConnection},
		{"unknown environment", "key", "staging", nil, ErrInvalidConnection},
		{"key refused by Gooyai", "key", "prod", &gooyai.APIError{StatusCode: 401}, ErrKeyRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &fakeClient{listProviders: func(f *gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
				probed = *f
				return nil, tt.probe
			}}
			svc, repo, _ := newTestService(t, fc)

			_, err := svc.Connect(ctx, "clinic-b", tt.key, tt.env)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.NotContains(t, repo.rows, "clinic-b", "nothing is stored on failure")
		})
	}

	t.Run("Gooyai unreachable is passed through", func(t *testing.T) {
		down := &gooyai.APIError{StatusCode: 503}
		svc, _, _ := newTestService(t, &fakeClient{listProviders: func(*gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
			return nil, down
		}})
		_, err := svc.Connect(ctx, "clinic-b", "key", "prod")
		assert.ErrorIs(t, err, gooyai.ErrServer)
	})

	t.Run("no encryption key: refuses before contacting Gooyai", func(t *testing.T) {
		svc := NewService(&fakeRepo{rows: map[string]*Connection{}}, nil, func(string, string) Client {
			t.Fatal("Gooyai must not be contacted")
			return nil
		})
		_, err := svc.Connect(ctx, "clinic-b", "key", "prod")
		assert.ErrorIs(t, err, ErrEncryptionNotConfigured)
	})

	t.Run("storage failure", func(t *testing.T) {
		svc, repo, _ := newTestService(t, okProbe(&probed))
		repo.err = errors.New("db down")
		_, err := svc.Connect(ctx, "clinic-b", "key", "prod")
		assert.EqualError(t, err, "db down")
	})
}

func TestService_GetConnectionAndDisconnect(t *testing.T) {
	svc, repo, _ := newTestService(t, &fakeClient{})
	ctx := context.Background()

	conn, err := svc.GetConnection(ctx, "clinic-a")
	require.NoError(t, err)
	assert.Equal(t, "prod", conn.Environment)

	require.NoError(t, svc.Disconnect(ctx, "clinic-a"))
	assert.NotContains(t, repo.rows, "clinic-a")

	conn, err = svc.GetConnection(ctx, "clinic-a")
	require.NoError(t, err)
	assert.Nil(t, conn)
}

func TestLast4(t *testing.T) {
	assert.Equal(t, "abcd", last4("sk-live-1234abcd"))
	assert.Empty(t, last4("short-key"), "too short to hint at safely")
}

func TestNewClientFactory(t *testing.T) {
	var gotKey, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotPath = r.Header.Get("x-api-key"), r.URL.Path
		_, _ = w.Write([]byte(`{"status":"success","data":{"id":7}}`))
	}))
	defer srv.Close()

	c := NewClientFactory(srv.URL)("key-a", "prod")

	call, err := c.GetIncomingCall(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), call.ID)
	assert.Equal(t, "key-a", gotKey)
	assert.Equal(t, "/api/v1/partners/incoming-calls/7", gotPath)
}
