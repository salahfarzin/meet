package health

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockDB struct {
	pingErr error
}

func (m *mockDB) Ping() error {
	return m.pingErr
}

func TestHealth(t *testing.T) {
	h := NewHealthHandler(nil, "1.0.0")
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected healthy, got %s", resp.Status)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", resp.Version)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name           string
		db             DatabasePinger
		expectedStatus int
		expectedHealth string
	}{
		{
			name:           "Healthy",
			db:             &mockDB{},
			expectedStatus: http.StatusOK,
			expectedHealth: "healthy",
		},
		{
			name:           "Unhealthy DB",
			db:             &mockDB{pingErr: errors.New("db error")},
			expectedStatus: http.StatusServiceUnavailable,
			expectedHealth: "unhealthy",
		},
		{
			name:           "No DB",
			db:             nil,
			expectedStatus: http.StatusOK,
			expectedHealth: "healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHealthHandler(tt.db, "1.0.0")
			req := httptest.NewRequest("GET", "/ready", nil)
			w := httptest.NewRecorder()

			h.Ready(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp HealthResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatal(err)
			}

			if resp.Status != tt.expectedHealth {
				t.Errorf("expected %s, got %s", tt.expectedHealth, resp.Status)
			}
		})
	}
}

func TestLive(t *testing.T) {
	h := NewHealthHandler(nil, "1.0.0")
	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()

	h.Live(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["status"] != "alive" {
		t.Errorf("expected alive, got %s", resp["status"])
	}
}
