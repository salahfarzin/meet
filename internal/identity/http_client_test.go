package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetByUUIDsEnrichesByUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users", r.URL.Path)
		assert.Equal(t, "Bearer tok-123", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"uuid": "u1", "first_name": "Ada", "last_name": "Lovelace", "national_code": "123", "mobile": "0900"},
			},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	ctx := WithBearer(context.Background(), "tok-123")
	got, err := c.GetByUUIDs(ctx, []string{"u1"})
	require.NoError(t, err)
	assert.Equal(t, "Ada", got["u1"].FirstName)
	assert.Equal(t, "123", got["u1"].NationalCode)
}

func TestSearchReturnsUUIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "national_code", r.URL.Query().Get("filters[0][field]"))
		assert.Equal(t, "555", r.URL.Query().Get("filters[0][value]"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{"uuid": "u2"}, {"uuid": "u3"}},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	got, err := c.Search(context.Background(), IdentityFilter{NationalID: "555"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"u2", "u3"}, got)
}
