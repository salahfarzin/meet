package gooyai

import (
	"context"
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

// newTestService wires fc in for clinic-a and records which key was used.
func newTestService(fc *fakeClient) (svc Service, usedKey *string) {
	usedKey = new(string)
	svc = NewService(map[string]string{"clinic-a": "key-a", "clinic-empty": ""}, func(key string) Client {
		*usedKey = key
		return fc
	})
	return svc, usedKey
}

func TestService_NotConnected(t *testing.T) {
	svc, _ := newTestService(&fakeClient{})
	ctx := context.Background()

	for _, tenant := range []string{"unknown", "clinic-empty"} {
		_, err := svc.ListIncomingCalls(ctx, tenant, nil)
		assert.ErrorIs(t, err, ErrNotConnected)
		_, err = svc.GetIncomingCall(ctx, tenant, 1)
		assert.ErrorIs(t, err, ErrNotConnected)
		_, err = svc.ListProviders(ctx, tenant)
		assert.ErrorIs(t, err, ErrNotConnected)
	}
}

func TestService_ListIncomingCalls_UsesTenantKey(t *testing.T) {
	want := &gooyai.CallPage{Calls: []gooyai.IncomingCall{{ID: 1}}}
	var got *gooyai.ListCallsFilter
	svc, usedKey := newTestService(&fakeClient{listCalls: func(f *gooyai.ListCallsFilter) (*gooyai.CallPage, error) {
		got = f
		return want, nil
	}})

	page, err := svc.ListIncomingCalls(context.Background(), "clinic-a", &gooyai.ListCallsFilter{FromNumber: "0912"})
	require.NoError(t, err)
	assert.Same(t, want, page)
	assert.Equal(t, "key-a", *usedKey)
	assert.Equal(t, "0912", got.FromNumber)
}

func TestService_GetIncomingCall(t *testing.T) {
	svc, _ := newTestService(&fakeClient{getCall: func(id int64) (*gooyai.IncomingCall, error) {
		return &gooyai.IncomingCall{ID: id}, nil
	}})

	call, err := svc.GetIncomingCall(context.Background(), "clinic-a", 101)
	require.NoError(t, err)
	assert.Equal(t, int64(101), call.ID)
}

func TestService_ListProviders_WalksAllPages(t *testing.T) {
	var pages []int
	svc, _ := newTestService(&fakeClient{listProviders: func(f *gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
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
	svc, _ := newTestService(&fakeClient{listProviders: func(*gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
		calls++
		return &gooyai.ProviderPage{Pagination: gooyai.Pagination{TotalPages: 1_000_000}}, nil
	}})

	_, err := svc.ListProviders(context.Background(), "clinic-a")
	require.NoError(t, err)
	assert.Equal(t, maxProviderPages, calls)
}

func TestService_ListProviders_Error(t *testing.T) {
	boom := errors.New("boom")
	svc, _ := newTestService(&fakeClient{listProviders: func(*gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
		return nil, boom
	}})

	_, err := svc.ListProviders(context.Background(), "clinic-a")
	assert.ErrorIs(t, err, boom)
}

func TestNewClientFactory(t *testing.T) {
	var gotKey, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotPath = r.Header.Get("x-api-key"), r.URL.Path
		_, _ = w.Write([]byte(`{"status":"success","data":{"id":7}}`))
	}))
	defer srv.Close()

	c := NewClientFactory(srv.URL, "prod")("key-a")

	call, err := c.GetIncomingCall(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), call.ID)
	assert.Equal(t, "key-a", gotKey)
	assert.Equal(t, "/api/v1/partners/incoming-calls/7", gotPath)
}
