package gooyai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKey = "tenant-key"

// recorded is what the fake server saw for the last request.
type recorded struct {
	method string
	path   string
	query  string
	apiKey string
	ctype  string
	body   string
}

// newTestClient starts a server that replies with status/body and records
// each request. Retries are off unless a test turns them on.
func newTestClient(t *testing.T, status int, body string, opts ...Option) (*Client, *recorded) {
	t.Helper()
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*rec = recorded{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			apiKey: r.Header.Get("x-api-key"),
			ctype:  r.Header.Get("Content-Type"),
			body:   string(b),
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	all := append([]Option{WithBaseURL(srv.URL + "/"), WithRetry(0, 0)}, opts...)
	return New(testKey, all...), rec
}

func success(data string) string {
	return `{"status":"success","message":"ok","data":` + data + `}`
}

func TestNew_Defaults(t *testing.T) {
	c := New(testKey)
	assert.Equal(t, DefaultBaseURL, c.baseURL)
	assert.Equal(t, EnvSandbox, c.env)
	assert.Equal(t, defaultRetries, c.retries)
	assert.Equal(t, defaultTimeout, c.httpClient.Timeout)
}

func TestOptions(t *testing.T) {
	hc := &http.Client{}
	c := New(testKey, WithHTTPClient(hc), WithEnv(EnvProduction), WithRetry(-3, time.Second))
	assert.Same(t, hc, c.httpClient)
	assert.Equal(t, EnvProduction, c.env)
	assert.Equal(t, 0, c.retries, "negative retries clamp to zero")
	assert.Equal(t, time.Second, c.backoff)
}

func TestEnvPathPrefix(t *testing.T) {
	assert.Equal(t, "/api/v1/partners", EnvProduction.pathPrefix())
	assert.Equal(t, "/api/sandbox/v1/partners", EnvSandbox.pathPrefix())
	assert.Equal(t, "/api/sandbox/v1/partners", Env("production").pathPrefix(), "unknown env falls back to sandbox")
}

func TestRequest_SendsKeyAndUsesEnvPrefix(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(`[]`), WithEnv(EnvProduction))

	_, err := c.AttributeCatalog(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api/v1/partners/service-provider-attributes", rec.path)
	assert.Equal(t, testKey, rec.apiKey)
	assert.Empty(t, rec.query)
	assert.Empty(t, rec.ctype, "no Content-Type without a body")
}

const callJSON = `{
  "id": 101,
  "from_number": "+989121111111",
  "to_number": "+989121000001",
  "start_at": "2026-06-23T08:15:00.000Z",
  "end_at": "2026-06-23T08:17:00.000Z",
  "duration_seconds": 120,
  "recording_url": "https://example.com/recordings/call-101.mp3",
  "call_forward": false,
  "should_recall_user": true,
  "analytics": {"sentiment": "neutral", "resolved": true},
  "chats": [{"role": "assistant", "content": "Sample conversation transcript"}],
  "model": {"id": 1, "name": "Support Bot", "type": "support"}
}`

func TestListIncomingCalls(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(`{"calls":[`+callJSON+`],"pagination":{"total":42,"page":2,"limit":10,"total_pages":5}}`))
	recall, forward := true, false

	page, err := c.ListIncomingCalls(context.Background(), &ListCallsFilter{
		PageQuery:        PageQuery{Page: 2, Limit: 10},
		FromNumber:       "0912",
		ToNumber:         "021",
		ModelID:          7,
		StartDate:        "2026-06-01",
		EndDate:          "2026-06-30",
		CallForward:      &forward,
		ShouldRecallUser: &recall,
	})
	require.NoError(t, err)

	assert.Equal(t, "/api/sandbox/v1/partners/incoming-calls", rec.path)
	assert.Equal(t, "call_forward=false&end_date=2026-06-30&from_number=0912&limit=10&model_id=7&page=2&should_recall_user=true&start_date=2026-06-01&to_number=021", rec.query)
	assert.Equal(t, Pagination{Total: 42, Page: 2, Limit: 10, TotalPages: 5}, page.Pagination)
	require.Len(t, page.Calls, 1)

	call := page.Calls[0]
	assert.Equal(t, int64(101), call.ID)
	assert.Equal(t, 120, call.DurationSeconds)
	assert.True(t, call.ShouldRecallUser)
	assert.Equal(t, "neutral", call.Analytics["sentiment"])
	assert.Equal(t, &AIModel{ID: 1, Name: "Support Bot", Type: "support"}, call.Model)
}

func TestListIncomingCalls_OmitsZeroFilters(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(`{"calls":[],"pagination":{}}`))

	_, err := c.ListIncomingCalls(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, rec.query, "Gooyai rejects unknown/empty params, so nothing is sent")
}

func TestGetIncomingCall(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(callJSON))

	call, err := c.GetIncomingCall(context.Background(), 101)
	require.NoError(t, err)
	assert.Equal(t, "/api/sandbox/v1/partners/incoming-calls/101", rec.path)
	assert.Equal(t, "+989121111111", call.FromNumber)

	msgs, ok := call.ChatMessages()
	require.True(t, ok)
	assert.Equal(t, []ChatMessage{{Role: "assistant", Content: "Sample conversation transcript"}}, msgs)
}

func TestChatMessages(t *testing.T) {
	tests := []struct {
		name  string
		chats string
		ok    bool
	}{
		{"absent", ``, false},
		{"null", `null`, false},
		{"unexpected shape", `{"turns":3}`, false},
		{"documented shape", `[{"role":"user","content":"hi"}]`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call := IncomingCall{Chats: json.RawMessage(tt.chats)}
			_, ok := call.ChatMessages()
			assert.Equal(t, tt.ok, ok)
		})
	}
}

const providerJSON = `{
  "id": 1, "tenant_id": 1, "first_name": "Aaran", "last_name": "Ahmadi",
  "specialty": "general practitioner", "phone": "+989121000001",
  "profile_image_url": null, "is_active": true,
  "created_at": "2026-06-17T12:00:00.000Z", "updated_at": "2026-06-17T12:00:00.000Z",
  "attributes": [{"id": 6, "name": "نحوه حضور", "data_type": "string", "value": "حضوری", "attribute_value_id": 42}]
}`

func TestListProviders(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(`{"providers":[`+providerJSON+`],"pagination":{"total":1,"page":1,"limit":100,"total_pages":1}}`))
	active := true

	page, err := c.ListProviders(context.Background(), &ListProvidersFilter{PageQuery: PageQuery{Limit: 100}, IsActive: &active, Search: "Ahmadi"})
	require.NoError(t, err)
	assert.Equal(t, "/api/sandbox/v1/partners/providers", rec.path)
	assert.Equal(t, "is_active=true&limit=100&search=Ahmadi", rec.query)
	require.Len(t, page.Providers, 1)
	assert.Nil(t, page.Providers[0].ProfileImageURL)
	assert.Equal(t, int64(42), page.Providers[0].Attributes[0].AttributeValueID)
}

func TestGetProvider(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(providerJSON))

	p, err := c.GetProvider(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1", rec.path)
	assert.Equal(t, "Aaran", p.FirstName)
}

func TestProviderAttributes(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(`[{"id":6,"name":"نحوه حضور","data_type":"string","value":"حضوری","attribute_value_id":42}]`))

	attrs, err := c.ProviderAttributes(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/attributes", rec.path)
	assert.Equal(t, "حضوری", attrs[0].Value)
}

const catalogJSON = `[{"id":6,"name":"نحوه حضور","data_type":"string","values":[{"id":42,"value":"حضوری"},{"id":43,"value":"غیر حضوری"}]}]`

func TestAttributeCatalog(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, success(catalogJSON))

	attrs, err := c.AttributeCatalog(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []AttributeValue{{ID: 42, Value: "حضوری"}, {ID: 43, Value: "غیر حضوری"}}, attrs[0].Values)
}

const slotJSON = `{
  "id": 10, "service_provider_id": 1, "slot_date": "2026-07-15", "status": "available",
  "start_time": "09:00:00", "end_time": "12:00:00", "interval_minutes": 30,
  "description": "Morning schedule",
  "created_at": "2026-06-17T12:00:00.000Z", "updated_at": "2026-06-17T12:00:00.000Z",
  "attributes": [{"id": 6, "name": "نحوه حضور", "data_type": "string", "value": "حضوری", "attribute_value_id": 42}],
  "time_segments": [
    {"start_time": "09:00:00", "end_time": "09:30:00", "status": "available"},
    {"start_time": "09:30:00", "end_time": "10:00:00", "status": "booked"}
  ]
}`

func TestListSlots(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(`[`+slotJSON+`]`))

	slots, err := c.ListSlots(context.Background(), 1, &ListSlotsFilter{
		Date: "2026-07-15", StartDate: "2026-07-01", EndDate: "2026-07-31", StartTime: "09:00", EndTime: "12:00",
	})
	require.NoError(t, err)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/availability-slots", rec.path)
	assert.Equal(t, "date=2026-07-15&end_date=2026-07-31&end_time=12%3A00&start_date=2026-07-01&start_time=09%3A00", rec.query)
	require.Len(t, slots, 1)
	assert.Equal(t, SlotBooked, slots[0].TimeSegments[1].Status)
}

func TestCreateSlot(t *testing.T) {
	c, rec := newTestClient(t, http.StatusCreated, success(slotJSON))

	slot, err := c.CreateSlot(context.Background(), 1, &CreateSlotRequest{
		SlotDate: "2026-07-15", StartTime: "09:00", EndTime: "12:00", Description: "sync:meet",
		Attributes: []AttributeSelection{{AttributeID: 6, AttributeValueIDs: []int64{42}}},
	})
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/availability-slots", rec.path)
	assert.Equal(t, "application/json", rec.ctype)
	assert.JSONEq(t, `{"slot_date":"2026-07-15","start_time":"09:00","end_time":"12:00","description":"sync:meet","attributes":[{"attribute_id":6,"attribute_value_ids":[42]}]}`, rec.body)
	assert.Equal(t, int64(10), slot.ID)
}

func TestUpdateSlot(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(slotJSON))
	status := SlotNotAvailable

	_, err := c.UpdateSlot(context.Background(), 1, 10, &UpdateSlotRequest{Status: &status})
	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, rec.method)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/availability-slots/10", rec.path)
	assert.JSONEq(t, `{"status":"not_available"}`, rec.body, "nil fields are left out")
}

func TestDeleteSlot(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(`null`))

	require.NoError(t, c.DeleteSlot(context.Background(), 1, 10))
	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/availability-slots/10", rec.path)
}

func TestBookingOptions(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(catalogJSON))

	attrs, err := c.BookingOptions(context.Background(), 1, 10, "10:00", "10:30")
	require.NoError(t, err)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/availability-slots/10/booking-options", rec.path)
	assert.Equal(t, "end_time=10%3A30&start_time=10%3A00", rec.query)
	assert.Len(t, attrs[0].Values, 2)
}

const bookingJSON = `{
  "id": 10, "service_provider_id": 1, "availability_slot_id": 10,
  "customer_name": "Sara Ahmadi", "customer_phone": "09121234567",
  "slot_date": "2026-07-15", "start_time": "10:00:00", "end_time": "10:30:00",
  "status": "active",
  "attributes": [{"id": 6, "name": "نحوه حضور", "data_type": "string", "values": [{"id": 42, "value": "حضوری"}]}],
  "created_at": "2026-07-01T08:00:00.000Z", "updated_at": "2026-07-01T08:00:00.000Z"
}`

func TestListBookings(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(`{"bookings":[`+bookingJSON+`],"pagination":{"total":1,"page":1,"limit":10,"total_pages":1}}`))

	page, err := c.ListBookings(context.Background(), 1, &ListBookingsFilter{PageQuery: PageQuery{Page: 1}, Status: BookingActive})
	require.NoError(t, err)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/bookings", rec.path)
	assert.Equal(t, "page=1&status=active", rec.query)
	require.Len(t, page.Bookings, 1)
	assert.Equal(t, int64(42), page.Bookings[0].Attributes[0].Values[0].ID)
}

func TestGetBooking(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(bookingJSON))

	b, err := c.GetBooking(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/bookings/10", rec.path)
	assert.Equal(t, "09121234567", b.CustomerPhone)
}

func TestCreateBooking(t *testing.T) {
	c, rec := newTestClient(t, http.StatusCreated, success(bookingJSON))

	b, err := c.CreateBooking(context.Background(), 1, &CreateBookingRequest{
		ServiceProviderID: 1, AvailabilitySlotID: 10, StartTime: "10:00", EndTime: "10:30",
		CustomerName: "Sara Ahmadi", CustomerPhone: "09121234567",
	})
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/bookings", rec.path)
	assert.JSONEq(t, `{"service_provider_id":1,"availability_slot_id":10,"start_time":"10:00","end_time":"10:30","customer_name":"Sara Ahmadi","customer_phone":"09121234567"}`, rec.body)
	assert.Equal(t, BookingActive, b.Status)
}

func TestCancelBooking(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, success(`{"id":10,"status":"canceled"}`))

	b, err := c.CancelBooking(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, rec.method)
	assert.Equal(t, "/api/sandbox/v1/partners/providers/1/bookings/10", rec.path)
	assert.Empty(t, rec.body, "cancel takes no body")
	assert.Equal(t, BookingCanceled, b.Status)
}

// Every method should surface API errors unchanged. One table covers them so
// each method's error branch is exercised.
func TestMethods_PropagateAPIErrors(t *testing.T) {
	c, _ := newTestClient(t, http.StatusNotFound, `{"status":404,"error":{"message":"یافت نشد","errors":["یافت نشد"]}}`)
	ctx := context.Background()

	calls := map[string]func() error{
		"ListIncomingCalls":  func() error { _, err := c.ListIncomingCalls(ctx, nil); return err },
		"GetIncomingCall":    func() error { _, err := c.GetIncomingCall(ctx, 1); return err },
		"ListProviders":      func() error { _, err := c.ListProviders(ctx, nil); return err },
		"GetProvider":        func() error { _, err := c.GetProvider(ctx, 1); return err },
		"ProviderAttributes": func() error { _, err := c.ProviderAttributes(ctx, 1); return err },
		"AttributeCatalog":   func() error { _, err := c.AttributeCatalog(ctx); return err },
		"ListSlots":          func() error { _, err := c.ListSlots(ctx, 1, nil); return err },
		"CreateSlot":         func() error { _, err := c.CreateSlot(ctx, 1, &CreateSlotRequest{}); return err },
		"UpdateSlot":         func() error { _, err := c.UpdateSlot(ctx, 1, 1, &UpdateSlotRequest{}); return err },
		"DeleteSlot":         func() error { return c.DeleteSlot(ctx, 1, 1) },
		"BookingOptions":     func() error { _, err := c.BookingOptions(ctx, 1, 1, "", ""); return err },
		"ListBookings":       func() error { _, err := c.ListBookings(ctx, 1, nil); return err },
		"GetBooking":         func() error { _, err := c.GetBooking(ctx, 1, 1); return err },
		"CreateBooking":      func() error { _, err := c.CreateBooking(ctx, 1, &CreateBookingRequest{}); return err },
		"CancelBooking":      func() error { _, err := c.CancelBooking(ctx, 1, 1); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			assert.ErrorIs(t, call(), ErrNotFound)
		})
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{http.StatusBadRequest, ErrBadRequest},
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusNotFound, ErrNotFound},
		{http.StatusConflict, ErrConflict},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusInternalServerError, ErrServer},
		{http.StatusBadGateway, ErrServer},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			c, _ := newTestClient(t, tt.status, `{"status":0,"error":{"message":"m","errors":["d1","d2"]}}`)

			_, err := c.GetIncomingCall(context.Background(), 1)

			assert.ErrorIs(t, err, tt.want)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, tt.status, apiErr.StatusCode)
			assert.Equal(t, "m", apiErr.Message)
			assert.Equal(t, []string{"d1", "d2"}, apiErr.Details)
		})
	}
}

func TestAPIError_IsUnrelatedTarget(t *testing.T) {
	err := &APIError{StatusCode: http.StatusNotFound}
	assert.NotErrorIs(t, err, ErrConflict)
	assert.NotErrorIs(t, err, errors.New("other"))
}

func TestAPIError_Error(t *testing.T) {
	assert.Equal(t, "gooyai: HTTP 502", (&APIError{StatusCode: 502}).Error())
	assert.Equal(t, "gooyai: HTTP 400: bad (a; b)", (&APIError{StatusCode: 400, Message: "bad", Details: []string{"a", "b"}}).Error())
}

func TestError_NonJSONBody(t *testing.T) {
	c, _ := newTestClient(t, http.StatusBadGateway, `<html>bad gateway</html>`)

	_, err := c.GetIncomingCall(context.Background(), 1)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	assert.Empty(t, apiErr.Message)
}

func TestDecode_MalformedEnvelope(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, `not json`)

	_, err := c.GetIncomingCall(context.Background(), 1)
	assert.ErrorContains(t, err, "decode GET /incoming-calls/1 response")
}

func TestDecode_MalformedData(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, success(`{"id":"not-a-number"}`))

	_, err := c.GetIncomingCall(context.Background(), 1)
	assert.ErrorContains(t, err, "decode GET /incoming-calls/1 data")
}

func TestDecode_NullOrMissingDataLeavesZeroValue(t *testing.T) {
	for _, body := range []string{success(`null`), `{"status":"success"}`} {
		c, _ := newTestClient(t, http.StatusOK, body)

		attrs, err := c.AttributeCatalog(context.Background())
		require.NoError(t, err)
		assert.Nil(t, attrs)
	}
}

func TestRequest_InvalidBaseURL(t *testing.T) {
	c := New(testKey, WithBaseURL("http://bad host"), WithRetry(0, 0))

	_, err := c.GetIncomingCall(context.Background(), 1)
	assert.ErrorContains(t, err, "build GET /incoming-calls/1 request")
}

func TestRequest_UnencodableBody(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, success(`{}`))

	err := c.do(context.Background(), http.MethodPost, "/x", nil, map[string]any{"bad": make(chan int)}, nil)
	assert.ErrorContains(t, err, "encode POST /x body")
}

// flakyServer fails the first `failures` requests with status, then succeeds.
func flakyServer(t *testing.T, failures int32, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= failures {
			w.WriteHeader(status)
			return
		}
		_, _ = io.WriteString(w, success(callJSON))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestRetry_GETRetriesServerErrors(t *testing.T) {
	srv, hits := flakyServer(t, 2, http.StatusServiceUnavailable)
	c := New(testKey, WithBaseURL(srv.URL), WithRetry(2, time.Millisecond))

	_, err := c.GetIncomingCall(context.Background(), 101)
	require.NoError(t, err)
	assert.Equal(t, int32(3), hits.Load())
}

func TestRetry_GivesUpAfterRetries(t *testing.T) {
	srv, hits := flakyServer(t, 10, http.StatusInternalServerError)
	c := New(testKey, WithBaseURL(srv.URL), WithRetry(1, 0))

	_, err := c.GetIncomingCall(context.Background(), 101)
	assert.ErrorIs(t, err, ErrServer)
	assert.Equal(t, int32(2), hits.Load())
}

func TestRetry_ClientErrorsAreNotRetried(t *testing.T) {
	srv, hits := flakyServer(t, 10, http.StatusNotFound)
	c := New(testKey, WithBaseURL(srv.URL), WithRetry(3, 0))

	_, err := c.GetIncomingCall(context.Background(), 101)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Equal(t, int32(1), hits.Load())
}

func TestRetry_WritesAreNeverRetried(t *testing.T) {
	srv, hits := flakyServer(t, 10, http.StatusInternalServerError)
	c := New(testKey, WithBaseURL(srv.URL), WithRetry(3, 0))

	_, err := c.CreateBooking(context.Background(), 1, &CreateBookingRequest{})
	assert.ErrorIs(t, err, ErrServer)
	assert.Equal(t, int32(1), hits.Load(), "a retried POST could double-book")
}

func TestRetry_TransportErrorIsRetried(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens any more: every attempt is refused

	var attempts atomic.Int32
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return http.DefaultTransport.RoundTrip(r)
	})}
	c := New(testKey, WithBaseURL(url), WithHTTPClient(hc), WithRetry(2, 0))

	_, err := c.GetIncomingCall(context.Background(), 1)
	assert.ErrorContains(t, err, "gooyai: request failed")
	assert.Equal(t, int32(3), attempts.Load())
	var te *transportError
	require.ErrorAs(t, err, &te)
	assert.Error(t, te.Unwrap())
}

func TestRetry_ContextCanceledDuringBackoff(t *testing.T) {
	srv, hits := flakyServer(t, 10, http.StatusInternalServerError)
	c := New(testKey, WithBaseURL(srv.URL), WithRetry(3, time.Hour))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := c.GetIncomingCall(ctx, 1)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, int32(1), hits.Load())
}

func TestSleep_ZeroDelayReportsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, sleep(ctx, 0), context.Canceled)
	assert.NoError(t, sleep(context.Background(), 0))
}

func TestRetryable(t *testing.T) {
	assert.False(t, retryable(nil))
	assert.False(t, retryable(errors.New("decode failure")))
	assert.True(t, retryable(&transportError{err: errors.New("refused")}))
	assert.True(t, retryable(&APIError{StatusCode: 503}))
	assert.False(t, retryable(&APIError{StatusCode: 429}))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
