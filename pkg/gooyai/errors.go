package gooyai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Sentinel errors, matched with errors.Is against an *APIError by HTTP status.
var (
	ErrBadRequest   = errors.New("gooyai: bad request")
	ErrUnauthorized = errors.New("gooyai: missing or invalid tenant API key")
	ErrNotFound     = errors.New("gooyai: not found")
	ErrConflict     = errors.New("gooyai: conflict")
	ErrRateLimited  = errors.New("gooyai: rate limited")
	ErrServer       = errors.New("gooyai: server error")
)

// APIError is a non-2xx response. Gooyai's messages are Persian and meant for
// its own panel, so callers should map StatusCode (via errors.Is and the
// sentinels above) to their own user-facing text rather than surface Message.
type APIError struct {
	StatusCode int
	Message    string
	Details    []string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("gooyai: HTTP %d", e.StatusCode)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if len(e.Details) > 0 {
		msg += " (" + strings.Join(e.Details, "; ") + ")"
	}
	return msg
}

// Is lets errors.Is(err, ErrNotFound) etc. match by status code.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrBadRequest:
		return e.StatusCode == http.StatusBadRequest
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrConflict:
		return e.StatusCode == http.StatusConflict
	case ErrRateLimited:
		return e.StatusCode == http.StatusTooManyRequests
	case ErrServer:
		return e.StatusCode >= http.StatusInternalServerError
	}
	return false
}

// errorEnvelope is {"status":400,"error":{"message":"...","errors":["..."]}}.
type errorEnvelope struct {
	Error struct {
		Message string   `json:"message"`
		Errors  []string `json:"errors"`
	} `json:"error"`
}

// decodeError builds an *APIError from a non-2xx response. A body that isn't
// the documented error envelope (e.g. an HTML page from a proxy) still yields
// an APIError carrying the status code.
func decodeError(resp *http.Response) error {
	apiErr := &APIError{StatusCode: resp.StatusCode}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil {
		return apiErr
	}
	var env errorEnvelope
	if json.Unmarshal(body, &env) == nil {
		apiErr.Message = env.Error.Message
		apiErr.Details = env.Error.Errors
	}
	return apiErr
}

// transportError wraps a network-level failure (DNS, refused, timeout) so it
// can be told apart from an API response when deciding whether to retry.
type transportError struct {
	err error
}

func (e *transportError) Error() string { return "gooyai: request failed: " + e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

// retryable reports whether a GET that failed with err is worth repeating.
func retryable(err error) bool {
	if err == nil {
		return false
	}
	var te *transportError
	if errors.As(err, &te) {
		return true
	}
	return errors.Is(err, ErrServer)
}
