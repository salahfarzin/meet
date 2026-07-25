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

// TestGetByUUIDsCarriesNameField verifies the `name` field (a clinic/center's
// display title - clinics have no first_name/last_name) survives into Identity.
func TestGetByUUIDsCarriesNameField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"uuid": "clinic-1", "first_name": "", "last_name": "", "name": "Tehran Clinic"},
			},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	ctx := WithBearer(context.Background(), "tok-123")
	got, err := c.GetByUUIDs(ctx, []string{"clinic-1"})
	require.NoError(t, err)
	assert.Equal(t, "Tehran Clinic", got["clinic-1"].Name)
}

func TestSearchReturnsUUIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "national_code", r.URL.Query().Get("filters[0][field]"))
		assert.Equal(t, "555", r.URL.Query().Get("filters[0][value]"))
		assert.Equal(t, "contains", r.URL.Query().Get("match_mode[national_code]"))
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

func TestGetByUUIDsEmptyInputSkipsRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be made for empty uuids")
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	got, err := c.GetByUUIDs(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestNewHTTPClientDefaultsToDefaultClient(t *testing.T) {
	c := NewHTTPClient("http://example.com", nil)
	require.NotNil(t, c)
}

func TestListClinicsReturnsNameFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "center", r.URL.Query().Get("filters[0][field]"))
		assert.Equal(t, "1", r.URL.Query().Get("filters[0][value]"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"uuid": "c1", "name": "Named Clinic"},
				{"uuid": "c2", "first_name": "Fallback", "last_name": "Clinic"},
			},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	got, err := c.ListClinics(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, Clinic{UUID: "c1", Name: "Named Clinic"}, got[0])
	assert.Equal(t, Clinic{UUID: "c2", Name: "Fallback Clinic"}, got[1])
}

func TestListClinicsPropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	_, err := c.ListClinics(context.Background())
	assert.Error(t, err)
}

func TestItemsNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	_, err := c.Search(context.Background(), IdentityFilter{FirstName: "x"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}

func TestItemsDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	_, err := c.Search(context.Background(), IdentityFilter{FirstName: "x"})
	assert.Error(t, err)
}

func TestItemsUsesReturnEnvelopeWhenPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"return": map[string]any{
				"items": []map[string]any{{"uuid": "wrapped"}},
			},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	got, err := c.Search(context.Background(), IdentityFilter{FirstName: "x"})
	require.NoError(t, err)
	assert.Equal(t, []string{"wrapped"}, got)
}

func TestSearchNoFiltersSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.Query().Get("filters[0][field]"))
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{}})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	got, err := c.Search(context.Background(), IdentityFilter{})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSearchMultipleFiltersOrdered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// first_name comes before national_code in the canonical order
		assert.Equal(t, "first_name", q.Get("filters[0][field]"))
		assert.Equal(t, "Ada", q.Get("filters[0][value]"))
		assert.Equal(t, "national_code", q.Get("filters[1][field]"))
		assert.Equal(t, "555", q.Get("filters[1][value]"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{"uuid": "u4"}},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, srv.Client())
	got, err := c.Search(context.Background(), IdentityFilter{FirstName: "Ada", NationalID: "555"})
	require.NoError(t, err)
	assert.Equal(t, []string{"u4"}, got)
}
