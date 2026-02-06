package health

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Version   string            `json:"version,omitempty"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// DatabasePinger interface for database operations
type DatabasePinger interface {
	Ping() error
}

// HealthHandler handles health check requests
type HealthHandler struct {
	db      DatabasePinger
	version string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db DatabasePinger, version string) *HealthHandler {
	return &HealthHandler{
		db:      db,
		version: version,
	}
}

// Health returns basic health status (for load balancer/k8s probes)
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   h.version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Ready returns readiness status including dependency checks
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string)
	status := "healthy"
	httpStatus := http.StatusOK

	// Check database connectivity
	if h.db != nil {
		if err := h.db.Ping(); err != nil {
			checks["database"] = "unhealthy: " + err.Error()
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		} else {
			checks["database"] = "healthy"
		}
	}

	// Add runtime info
	checks["goroutines"] = strconv.Itoa(runtime.NumGoroutine())

	response := HealthResponse{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   h.version,
		Checks:    checks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(response)
}

// Live returns basic liveness status (for k8s liveness probes)
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "alive",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
