package gooyai

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// fakeRepo is an in-memory Repository; err, when set, fails every call, and
// dropWrites makes Upsert succeed without keeping anything (the users service
// ignoring a settings key it doesn't allow).
type fakeRepo struct {
	rows       map[string]*Connection
	err        error
	dropWrites bool
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
	if !r.dropWrites {
		r.rows[c.OrganizerUUID] = c
	}
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

// used records which key, environment and proxy the last client was built for.
type used struct{ key, env, proxy string }

// newTestService connects clinic-a (key "key-a", prod) and wires fc in.
func newTestService(t *testing.T, fc *fakeClient) (Service, *fakeRepo, *used) {
	t.Helper()
	sealer := testSealer(t)
	sealed, err := sealer.Seal("key-a", phoneAssistantAAD(fieldAPIKey, "clinic-a"))
	require.NoError(t, err)
	repo := &fakeRepo{rows: map[string]*Connection{
		"clinic-a": {OrganizerUUID: "clinic-a", APIKeySealed: sealed, KeyLast4: "", Environment: "prod"},
	}}
	u := &used{}
	svc := NewService(repo, sealer, func(key, env, proxy string) Client {
		u.key, u.env, u.proxy = key, env, proxy
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

	conn, err := svc.Connect(context.Background(), "clinic-b", &ConnectInput{APIKey: "  sk-live-123456789abcd  "})
	require.NoError(t, err)

	// Probed Gooyai with the trimmed key, sandbox by default, one row only.
	assert.Equal(t, used{key: "sk-live-123456789abcd", env: "sandbox"}, *u)
	assert.Equal(t, gooyai.PageQuery{Page: 1, Limit: 1}, probed.PageQuery)

	assert.Equal(t, "abcd", conn.KeyLast4)
	assert.Equal(t, "sandbox", conn.Environment)
	stored := repo.rows["clinic-b"]
	assert.NotContains(t, string(stored.APIKeySealed), "sk-live", "the key is never stored in the clear")
	plain, err := testSealer(t).Open(stored.APIKeySealed, phoneAssistantAAD(fieldAPIKey, "clinic-b"))
	require.NoError(t, err)
	assert.Equal(t, "sk-live-123456789abcd", plain)
}

func TestService_Connect_ReplacesExistingConnection(t *testing.T) {
	var probed gooyai.ListProvidersFilter
	svc, repo, _ := newTestService(t, okProbe(&probed))

	_, err := svc.Connect(context.Background(), "clinic-a", &ConnectInput{APIKey: "new-key-0000-wxyz", Environment: "prod"})
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

			_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: tt.key, Environment: tt.env})
			assert.ErrorIs(t, err, tt.wantErr)
			assert.NotContains(t, repo.rows, "clinic-b", "nothing is stored on failure")
		})
	}

	t.Run("Gooyai unreachable is passed through", func(t *testing.T) {
		down := &gooyai.APIError{StatusCode: 503}
		svc, _, _ := newTestService(t, &fakeClient{listProviders: func(*gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
			return nil, down
		}})
		_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "key", Environment: "prod"})
		assert.ErrorIs(t, err, gooyai.ErrServer)
	})

	t.Run("no encryption key: refuses before contacting Gooyai", func(t *testing.T) {
		svc := NewService(&fakeRepo{rows: map[string]*Connection{}}, nil, func(string, string, string) Client {
			t.Fatal("Gooyai must not be contacted")
			return nil
		})
		_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "key", Environment: "prod"})
		assert.ErrorIs(t, err, ErrEncryptionNotConfigured)
	})

	t.Run("storage failure", func(t *testing.T) {
		svc, repo, _ := newTestService(t, okProbe(&probed))
		repo.err = errors.New("users service down")
		_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "key", Environment: "prod"})
		assert.EqualError(t, err, "users service down")
	})

	t.Run("save accepted but not kept", func(t *testing.T) {
		svc, repo, _ := newTestService(t, okProbe(&probed))
		repo.dropWrites = true
		_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "key", Environment: "prod"})
		assert.ErrorIs(t, err, errNotStored)
	})

	t.Run("replacement accepted but the old connection kept", func(t *testing.T) {
		svc, repo, _ := newTestService(t, okProbe(&probed)) // clinic-a: key-a, prod
		old := repo.rows["clinic-a"]
		repo.dropWrites = true
		_, err := svc.Connect(ctx, "clinic-a", &ConnectInput{APIKey: "new-key-0000-wxyz", Environment: "sandbox"})
		assert.ErrorIs(t, err, errNotStored, "the old connection still being there is not success")
		assert.Same(t, old, repo.rows["clinic-a"])
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

	c := NewClientFactory(srv.URL, "")("key-a", "prod", "")

	call, err := c.GetIncomingCall(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), call.ID)
	assert.Equal(t, "key-a", gotKey)
	assert.Equal(t, "/api/v1/partners/incoming-calls/7", gotPath)
}

// fakeProxy is an HTTP forward proxy that answers every request itself and
// records the absolute URLs it was asked for.
func fakeProxy(t *testing.T) (srv *httptest.Server, seen *[]string) {
	t.Helper()
	var urls []string
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urls = append(urls, r.URL.String())
		_, _ = w.Write([]byte(`{"status":"success","data":{"id":7}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &urls
}

func TestNewClientFactory_Proxy(t *testing.T) {
	const target = "http://gooyai.invalid"

	t.Run("the clinic's own proxy", func(t *testing.T) {
		proxy, seen := fakeProxy(t)
		_, err := NewClientFactory(target, "")("key-a", "prod", proxy.URL).GetIncomingCall(context.Background(), 7)
		require.NoError(t, err)
		assert.Equal(t, []string{target + "/api/v1/partners/incoming-calls/7"}, *seen)
	})

	t.Run("the server default when the clinic has none", func(t *testing.T) {
		proxy, seen := fakeProxy(t)
		_, err := NewClientFactory(target, proxy.URL)("key-a", "prod", "").GetIncomingCall(context.Background(), 7)
		require.NoError(t, err)
		assert.Len(t, *seen, 1)
	})

	t.Run("a malformed default proxy is refused", func(t *testing.T) {
		_, err := NewClientFactoryChecked(target, "ftp://proxy")
		assert.ErrorIs(t, err, ErrInvalidProxy)
	})
}

func TestService_Connect_Proxy(t *testing.T) {
	ctx := context.Background()
	var probed gooyai.ListProvidersFilter

	t.Run("probes through the proxy, stores it sealed, shows it redacted", func(t *testing.T) {
		svc, repo, u := newTestService(t, okProbe(&probed))

		conn, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "sk-live-123456789abcd", ProxyURL: " socks5://u:pw@proxy:1080 "})
		require.NoError(t, err)

		assert.Equal(t, "socks5://u:pw@proxy:1080", u.proxy, "the probe goes through the proxy")
		assert.Equal(t, "socks5://u:****@proxy:1080", conn.ProxyRedacted)
		sealed := repo.rows["clinic-b"].ProxySealed
		assert.NotContains(t, string(sealed), "pw", "the proxy is never stored in the clear")
		plain, err := testSealer(t).Open(sealed, phoneAssistantAAD(fieldProxyURL, "clinic-b"))
		require.NoError(t, err)
		assert.Equal(t, "socks5://u:pw@proxy:1080", plain)

		got, err := svc.GetConnection(ctx, "clinic-b")
		require.NoError(t, err)
		assert.Equal(t, "socks5://u:****@proxy:1080", got.ProxyRedacted)

		u.proxy = ""
		_, err = svc.ListProviders(ctx, "clinic-b")
		require.NoError(t, err)
		assert.Equal(t, "socks5://u:pw@proxy:1080", u.proxy, "calls use the stored proxy")
	})

	t.Run("blank key and masked password keep what's stored", func(t *testing.T) {
		svc, repo, u := newTestService(t, okProbe(&probed))
		withPW := proxyURL("proxy:3128", "pw")
		_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "sk-live-123456789abcd", ProxyURL: withPW})
		require.NoError(t, err)

		conn, err := svc.Connect(ctx, "clinic-b", &ConnectInput{Environment: "prod", ProxyURL: redactProxy(withPW)})
		require.NoError(t, err)
		assert.Equal(t, used{key: "sk-live-123456789abcd", env: "prod", proxy: withPW}, *u)
		assert.Equal(t, "abcd", conn.KeyLast4)
		assert.Equal(t, "prod", repo.rows["clinic-b"].Environment)
	})

	t.Run("a masked password doesn't follow the proxy to another host", func(t *testing.T) {
		svc, _, u := newTestService(t, okProbe(&probed))
		_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "sk-live-123456789abcd", ProxyURL: proxyURL("proxy:3128", "pw")})
		require.NoError(t, err)

		elsewhere := redactProxy(proxyURL("elsewhere:3128", "pw"))
		_, err = svc.Connect(ctx, "clinic-b", &ConnectInput{ProxyURL: elsewhere})
		require.NoError(t, err)
		assert.Equal(t, elsewhere, u.proxy)
	})

	t.Run("an empty proxy clears it", func(t *testing.T) {
		svc, repo, _ := newTestService(t, okProbe(&probed))
		_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "sk-live-123456789abcd", ProxyURL: "http://proxy:3128"})
		require.NoError(t, err)

		conn, err := svc.Connect(ctx, "clinic-b", &ConnectInput{})
		require.NoError(t, err)
		assert.Empty(t, repo.rows["clinic-b"].ProxySealed)
		assert.Empty(t, conn.ProxyRedacted)
	})

	t.Run("rejections", func(t *testing.T) {
		svc, repo, _ := newTestService(t, &fakeClient{listProviders: func(*gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error) {
			t.Fatal("Gooyai must not be contacted")
			return nil, nil
		}})

		_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "key", ProxyURL: "ftp://proxy:21"})
		assert.ErrorIs(t, err, ErrInvalidProxy)
		_, err = svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "key", ProxyURL: "socks5://"})
		assert.ErrorIs(t, err, ErrInvalidProxy)
		_, err = svc.Connect(ctx, "clinic-b", &ConnectInput{})
		assert.ErrorIs(t, err, ErrInvalidConnection, "a blank key needs a stored one to keep")
		assert.NotContains(t, repo.rows, "clinic-b")
	})
}

// proxyURL builds http://u:<password>@host (built, not a literal, so gosec
// doesn't take test fixtures for hardcoded credentials).
func proxyURL(host, password string) string {
	return (&url.URL{Scheme: "http", User: url.UserPassword("u", password), Host: host}).String()
}

func TestRedactProxy(t *testing.T) {
	assert.Equal(t, "", redactProxy(""))
	assert.Equal(t, "http://proxy:3128", redactProxy("http://proxy:3128"))
	assert.Equal(t, "socks5://user@proxy:1080", redactProxy("socks5://user@proxy:1080"))
	assert.Equal(t, "socks5://user:****@proxy:1080", redactProxy("socks5://user:s3cret@proxy:1080"))
	assert.Equal(t, maskedPassword, redactProxy("http://[::1"))
}

func TestService_SwappedCiphertextIsRefused(t *testing.T) {
	ctx := context.Background()
	var probed gooyai.ListProvidersFilter
	svc, repo, _ := newTestService(t, okProbe(&probed))
	_, err := svc.Connect(ctx, "clinic-b", &ConnectInput{APIKey: "sk-live-123456789abcd"})
	require.NoError(t, err)

	// What a clinic could do by editing its own settings: copy the sealed key
	// into the proxy field.
	repo.rows["clinic-b"].ProxySealed = repo.rows["clinic-b"].APIKeySealed

	conn, err := svc.GetConnection(ctx, "clinic-b")
	require.Error(t, err)
	assert.Nil(t, conn)
	assert.NotContains(t, err.Error(), "sk-live")

	// And copying another clinic's sealed key doesn't make it usable here.
	repo.rows["clinic-b"].ProxySealed = nil
	repo.rows["clinic-b"].APIKeySealed = repo.rows["clinic-a"].APIKeySealed
	_, err = svc.ListProviders(ctx, "clinic-b")
	assert.Error(t, err)
}

func TestService_InvalidStoredProxyIsNotReturned(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newTestService(t, &fakeClient{})
	sealed, err := testSealer(t).Seal("not a proxy", phoneAssistantAAD(fieldProxyURL, "clinic-a"))
	require.NoError(t, err)
	repo.rows["clinic-a"].ProxySealed = sealed

	_, err = svc.GetConnection(ctx, "clinic-a")
	assert.ErrorIs(t, err, errStoredProxyInvalid)
	assert.NotErrorIs(t, err, ErrInvalidProxy, "not reported as the caller's bad input")
}
