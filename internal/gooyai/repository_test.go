package gooyai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

const testBearer = "Bearer caller-token"

// fakeUsersService mimics psychometrist's /users/{uuid} and /me: it keeps one
// settings object per uuid and merges PUT settings key by key, like
// UserController::update does.
type fakeUsersService struct {
	settings map[string]map[string]any
	meUUID   string
	// dropKey, when set, makes PUT silently ignore that settings key, like a
	// key missing from allowedSettingsKeys.
	dropKey string
	status  int
	// tokens records the Authorization header of every request.
	tokens []string
}

func (f *fakeUsersService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.tokens = append(f.tokens, r.Header.Get("Authorization"))
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, `{"success":false,"errors":["nope"]}`)
		return
	}
	reply := func(v any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "return": v})
	}

	if r.URL.Path == "/v1/me" {
		reply(map[string]any{"uuid": f.meUUID})
		return
	}
	uuid := r.URL.Path[len("/v1/users/"):]
	switch r.Method {
	case http.MethodGet:
		reply(map[string]any{"uuid": uuid, "settings": f.settings[uuid]})
	case http.MethodPut:
		var body struct {
			Settings map[string]any `json:"settings"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if f.settings[uuid] == nil {
			f.settings[uuid] = map[string]any{}
		}
		for k, v := range body.Settings {
			if k != f.dropKey {
				f.settings[uuid][k] = v
			}
		}
		reply(map[string]any{"message": "ok"})
	}
}

func newUsersRepo(t *testing.T, fake *fakeUsersService) Repository {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return NewUsersRepository(srv.URL+"/v1/", srv.Client())
}

func callerCtx() context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", testBearer))
}

func TestUsersRepository_RoundTrip(t *testing.T) {
	fake := &fakeUsersService{settings: map[string]map[string]any{
		"clinic-a": {"timezone": "Asia/Tehran"},
	}}
	repo := newUsersRepo(t, fake)
	ctx := callerCtx()
	updated := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

	conn, err := repo.Get(ctx, "clinic-a")
	require.NoError(t, err)
	assert.Nil(t, conn, "no phone_assistant setting = not connected, not an error")

	require.NoError(t, repo.Upsert(ctx, &Connection{
		OrganizerUUID: "clinic-a", APIKeySealed: []byte("sealed"), KeyLast4: "ab12", Environment: "prod",
		ProxySealed: []byte("sealed-proxy"), UpdatedAt: updated,
	}))

	stored := fake.settings["clinic-a"]
	assert.Equal(t, "Asia/Tehran", stored["timezone"], "other settings are left alone")
	assert.Equal(t, map[string]any{
		"key_ciphertext":   base64.StdEncoding.EncodeToString([]byte("sealed")),
		"key_last4":        "ab12",
		"environment":      "prod",
		"proxy_ciphertext": base64.StdEncoding.EncodeToString([]byte("sealed-proxy")),
		"updated_at":       "2026-10-11T09:00:00Z",
	}, stored["phone_assistant"])

	conn, err = repo.Get(ctx, "clinic-a")
	require.NoError(t, err)
	assert.Equal(t, &Connection{
		OrganizerUUID: "clinic-a", APIKeySealed: []byte("sealed"), KeyLast4: "ab12", Environment: "prod",
		ProxySealed: []byte("sealed-proxy"), UpdatedAt: updated,
	}, conn)

	require.NoError(t, repo.Delete(ctx, "clinic-a"))
	assert.Nil(t, fake.settings["clinic-a"]["phone_assistant"])
	assert.Equal(t, "Asia/Tehran", fake.settings["clinic-a"]["timezone"])
	conn, err = repo.Get(ctx, "clinic-a")
	require.NoError(t, err)
	assert.Nil(t, conn)

	for _, tok := range fake.tokens {
		assert.Equal(t, testBearer, tok, "every call is made as the caller")
	}
}

func TestUsersRepository_LocalTenantResolvesThroughMe(t *testing.T) {
	fake := &fakeUsersService{meUUID: "real-uuid", settings: map[string]map[string]any{}}
	repo := newUsersRepo(t, fake)

	require.NoError(t, repo.Upsert(callerCtx(), &Connection{OrganizerUUID: localTenant, APIKeySealed: []byte("s"), Environment: "sandbox"}))
	assert.Contains(t, fake.settings, "real-uuid")
	assert.NotContains(t, fake.settings, localTenant)
}

func TestUsersRepository_Errors(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		_, err := NewUsersRepository("", http.DefaultClient).Get(callerCtx(), "clinic-a")
		assert.ErrorIs(t, err, ErrUsersServiceNotConfigured)
	})

	t.Run("no caller token", func(t *testing.T) {
		repo := newUsersRepo(t, &fakeUsersService{settings: map[string]map[string]any{}})
		_, err := repo.Get(context.Background(), "clinic-a")
		assert.ErrorIs(t, err, errNoCallerToken)
	})

	t.Run("users service refuses", func(t *testing.T) {
		repo := newUsersRepo(t, &fakeUsersService{status: http.StatusUnprocessableEntity})
		_, err := repo.Get(callerCtx(), "someone-else")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 422")
	})

	t.Run("corrupt ciphertext", func(t *testing.T) {
		repo := newUsersRepo(t, &fakeUsersService{settings: map[string]map[string]any{
			"clinic-a": {"phone_assistant": map[string]any{"key_ciphertext": "%%%", "environment": "prod"}},
		}})
		_, err := repo.Get(callerCtx(), "clinic-a")
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrNotConnected))
	})
}
