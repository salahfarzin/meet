package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestConfig holds configuration for E2E tests
type TestConfig struct {
	BaseURL    string
	AuthToken  string
	HTTPClient *http.Client
}

// NewTestConfig creates a new test configuration from environment variables
func NewTestConfig() *TestConfig {
	baseURL := os.Getenv("APP_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8083"
	}

	return &TestConfig{
		BaseURL: baseURL + "/api/v1",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// APIRequest represents a generic API request
type APIRequest struct {
	Method  string
	Path    string
	Body    interface{}
	Headers map[string]string
}

// APIResponse represents a generic API response
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// DoRequest performs an HTTP request and returns the response
func (c *TestConfig) DoRequest(t *testing.T, req APIRequest) *APIResponse {
	t.Helper()

	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), req.Method, c.BaseURL+req.Path, bodyReader)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set default headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Set custom headers (including auth)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       bodyBytes,
		Headers:    resp.Header,
	}
}

// DecodeJSON decodes the response body into the given interface
func (r *APIResponse) DecodeJSON(t *testing.T, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("Failed to unmarshal response: %v\nBody: %s", err, string(r.Body))
	}
}

// AssertStatusCode asserts that the response has the expected status code
func (r *APIResponse) AssertStatusCode(t *testing.T, expected int) {
	t.Helper()
	if r.StatusCode != expected {
		t.Errorf("Expected status code %d, got %d\nBody: %s", expected, r.StatusCode, string(r.Body))
	}
}

// GetAuthHeaders returns headers with mock authentication for testing
// In a real scenario, you would authenticate and get a real token
func GetAuthHeaders() map[string]string {
	return map[string]string{
		"X-User":       "test-user",
		"X-User-Uuid":  "test-user-uuid-123",
		"X-User-Roles": "Programmer",
	}
}

// WaitForServer waits for the server to be ready
func WaitForServer(baseURL string, maxRetries int) error {
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(baseURL + "/api/v1/meets")
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("server not ready after %d retries", maxRetries)
}
